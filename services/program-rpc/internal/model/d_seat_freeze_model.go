package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DSeatFreezeModel = (*customDSeatFreezeModel)(nil)

type (
	DSeatFreezeModel interface {
		dSeatFreezeModel
		withSession(session sqlx.Session) DSeatFreezeModel
		InsertWithSession(ctx context.Context, session sqlx.Session, data *DSeatFreeze) (sql.Result, error)
		FindOneByRequestNo(ctx context.Context, requestNo string) (*DSeatFreeze, error)
		FindOneByFreezeToken(ctx context.Context, freezeToken string) (*DSeatFreeze, error)
		FindOneByFreezeTokenForUpdate(ctx context.Context, session sqlx.Session, freezeToken string) (*DSeatFreeze, error)
		FindExpiredByProgramAndTicketCategory(ctx context.Context, session sqlx.Session, programId, ticketCategoryId int64, now time.Time) ([]*DSeatFreeze, error)
		MarkExpiredByFreezeTokens(ctx context.Context, session sqlx.Session, freezeTokens []string, now time.Time) error
		MarkConfirmedByFreezeToken(ctx context.Context, session sqlx.Session, freezeToken string, now time.Time) error
	}

	customDSeatFreezeModel struct {
		*defaultDSeatFreezeModel
	}
)

func NewDSeatFreezeModel(conn sqlx.SqlConn) DSeatFreezeModel {
	return &customDSeatFreezeModel{
		defaultDSeatFreezeModel: newDSeatFreezeModel(conn),
	}
}

func (m *customDSeatFreezeModel) withSession(session sqlx.Session) DSeatFreezeModel {
	return NewDSeatFreezeModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDSeatFreezeModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *DSeatFreeze) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`id`, `freeze_token`, `request_no`, `program_id`, `ticket_category_id`, `seat_count`, `freeze_status`, `expire_time`, `release_reason`, `release_time`, `create_time`, `edit_time`, `status`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return m.withSession(session).(*customDSeatFreezeModel).conn.ExecCtx(
		ctx,
		query,
		data.Id,
		data.FreezeToken,
		data.RequestNo,
		data.ProgramId,
		data.TicketCategoryId,
		data.SeatCount,
		data.FreezeStatus,
		data.ExpireTime,
		data.ReleaseReason,
		data.ReleaseTime,
		data.CreateTime,
		data.EditTime,
		data.Status,
	)
}

func (m *customDSeatFreezeModel) FindOneByRequestNo(ctx context.Context, requestNo string) (*DSeatFreeze, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `request_no` = ? limit 1",
		dSeatFreezeRows,
		m.table,
	)

	var resp DSeatFreeze
	err := m.conn.QueryRowCtx(ctx, &resp, query, requestNo)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customDSeatFreezeModel) FindOneByFreezeToken(ctx context.Context, freezeToken string) (*DSeatFreeze, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `freeze_token` = ? limit 1",
		dSeatFreezeRows,
		m.table,
	)

	var resp DSeatFreeze
	err := m.conn.QueryRowCtx(ctx, &resp, query, freezeToken)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customDSeatFreezeModel) FindOneByFreezeTokenForUpdate(ctx context.Context, session sqlx.Session, freezeToken string) (*DSeatFreeze, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `freeze_token` = ? limit 1 for update",
		dSeatFreezeRows,
		m.table,
	)

	var resp DSeatFreeze
	err := m.withSession(session).(*customDSeatFreezeModel).conn.QueryRowCtx(ctx, &resp, query, freezeToken)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customDSeatFreezeModel) FindExpiredByProgramAndTicketCategory(ctx context.Context, session sqlx.Session, programId, ticketCategoryId int64, now time.Time) ([]*DSeatFreeze, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `program_id` = ? and `ticket_category_id` = ? and `freeze_status` = 1 and `expire_time` <= ? order by `id` asc",
		dSeatFreezeRows,
		m.table,
	)

	var resp []*DSeatFreeze
	err := m.withSession(session).(*customDSeatFreezeModel).conn.QueryRowsCtx(ctx, &resp, query, programId, ticketCategoryId, now)
	switch err {
	case nil:
		return resp, nil
	case sqlx.ErrNotFound:
		return []*DSeatFreeze{}, nil
	default:
		return nil, err
	}
}

func (m *customDSeatFreezeModel) MarkExpiredByFreezeTokens(ctx context.Context, session sqlx.Session, freezeTokens []string, now time.Time) error {
	if len(freezeTokens) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(freezeTokens))
	args := make([]interface{}, 0, len(freezeTokens)+3)
	args = append(args, now, "expired", now)
	for _, token := range freezeTokens {
		placeholders = append(placeholders, "?")
		args = append(args, token)
	}

	query := fmt.Sprintf(
		"update %s set `freeze_status` = 3, `release_time` = ?, `release_reason` = ?, `edit_time` = ? where `status` = 1 and `freeze_status` = 1 and `freeze_token` in (%s)",
		m.table,
		strings.Join(placeholders, ", "),
	)

	_, err := m.withSession(session).(*customDSeatFreezeModel).conn.ExecCtx(ctx, query, args...)
	return err
}

func (m *customDSeatFreezeModel) MarkConfirmedByFreezeToken(ctx context.Context, session sqlx.Session, freezeToken string, now time.Time) error {
	query := fmt.Sprintf(
		"update %s set `freeze_status` = 4, `edit_time` = ? where `status` = 1 and `freeze_status` = 1 and `freeze_token` = ?",
		m.table,
	)

	_, err := m.withSession(session).(*customDSeatFreezeModel).conn.ExecCtx(ctx, query, now, freezeToken)
	return err
}
