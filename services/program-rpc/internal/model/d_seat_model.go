package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DSeatModel = (*customDSeatModel)(nil)

type (
	DSeatModel interface {
		dSeatModel
		withSession(session sqlx.Session) DSeatModel
		FindAvailableByProgramAndTicketCategoryForUpdate(ctx context.Context, session sqlx.Session, programId, ticketCategoryId int64) ([]*DSeat, error)
		FindAvailableCountByProgramId(ctx context.Context, programId int64) ([]*SeatRemainAggregate, error)
		FindByFreezeToken(ctx context.Context, freezeToken string) ([]*DSeat, error)
		BatchFreezeByIDs(ctx context.Context, session sqlx.Session, seatIDs []int64, freezeToken string, expireTime time.Time) error
		ReleaseByFreezeToken(ctx context.Context, session sqlx.Session, freezeToken string) error
		ConfirmByFreezeToken(ctx context.Context, session sqlx.Session, freezeToken string) error
	}

	customDSeatModel struct {
		*defaultDSeatModel
	}

	SeatRemainAggregate struct {
		TicketCategoryId int64 `db:"ticket_category_id"`
		RemainNumber     int64 `db:"remain_number"`
	}
)

func NewDSeatModel(conn sqlx.SqlConn) DSeatModel {
	return &customDSeatModel{
		defaultDSeatModel: newDSeatModel(conn),
	}
}

func (m *customDSeatModel) withSession(session sqlx.Session) DSeatModel {
	return NewDSeatModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDSeatModel) FindAvailableByProgramAndTicketCategoryForUpdate(ctx context.Context, session sqlx.Session, programId, ticketCategoryId int64) ([]*DSeat, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `program_id` = ? and `ticket_category_id` = ? and `seat_status` = 1 order by `row_code` asc, `col_code` asc, `id` asc for update",
		dSeatRows,
		m.table,
	)

	var resp []*DSeat
	err := m.withSession(session).(*customDSeatModel).conn.QueryRowsCtx(ctx, &resp, query, programId, ticketCategoryId)
	switch err {
	case nil:
		return resp, nil
	case sqlx.ErrNotFound:
		return []*DSeat{}, nil
	default:
		return nil, err
	}
}

func (m *customDSeatModel) FindAvailableCountByProgramId(ctx context.Context, programId int64) ([]*SeatRemainAggregate, error) {
	query := fmt.Sprintf(
		"select `ticket_category_id`, count(1) as `remain_number` from %s where `status` = 1 and `program_id` = ? and `seat_status` = 1 group by `ticket_category_id` order by `ticket_category_id` asc",
		m.table,
	)

	var resp []*SeatRemainAggregate
	err := m.conn.QueryRowsCtx(ctx, &resp, query, programId)
	switch err {
	case nil:
		return resp, nil
	case sqlx.ErrNotFound:
		return []*SeatRemainAggregate{}, nil
	default:
		return nil, err
	}
}

func (m *customDSeatModel) FindByFreezeToken(ctx context.Context, freezeToken string) ([]*DSeat, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `freeze_token` = ? order by `row_code` asc, `col_code` asc, `id` asc",
		dSeatRows,
		m.table,
	)

	var resp []*DSeat
	err := m.conn.QueryRowsCtx(ctx, &resp, query, freezeToken)
	switch err {
	case nil:
		return resp, nil
	case sqlx.ErrNotFound:
		return []*DSeat{}, nil
	default:
		return nil, err
	}
}

func (m *customDSeatModel) BatchFreezeByIDs(ctx context.Context, session sqlx.Session, seatIDs []int64, freezeToken string, expireTime time.Time) error {
	if len(seatIDs) == 0 {
		return nil
	}

	inClause, args := buildInt64InClause(seatIDs)
	query := fmt.Sprintf(
		"update %s set `seat_status` = 2, `freeze_token` = ?, `freeze_expire_time` = ?, `edit_time` = ? where `status` = 1 and `seat_status` = 1 and `id` in (%s)",
		m.table,
		inClause,
	)

	now := time.Now()
	args = append([]interface{}{freezeToken, expireTime, now}, args...)
	_, err := m.withSession(session).(*customDSeatModel).conn.ExecCtx(ctx, query, args...)
	return err
}

func (m *customDSeatModel) ReleaseByFreezeToken(ctx context.Context, session sqlx.Session, freezeToken string) error {
	query := fmt.Sprintf(
		"update %s set `seat_status` = 1, `freeze_token` = null, `freeze_expire_time` = null, `edit_time` = ? where `status` = 1 and `freeze_token` = ?",
		m.table,
	)

	_, err := m.withSession(session).(*customDSeatModel).conn.ExecCtx(ctx, query, time.Now(), freezeToken)
	return err
}

func (m *customDSeatModel) ConfirmByFreezeToken(ctx context.Context, session sqlx.Session, freezeToken string) error {
	query := fmt.Sprintf(
		"update %s set `seat_status` = 3, `edit_time` = ? where `status` = 1 and `seat_status` = 2 and `freeze_token` = ?",
		m.table,
	)

	_, err := m.withSession(session).(*customDSeatModel).conn.ExecCtx(ctx, query, time.Now(), freezeToken)
	return err
}
