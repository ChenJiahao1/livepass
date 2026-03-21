package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DOrderTicketUserModel = (*customDOrderTicketUserModel)(nil)

type (
	// DOrderTicketUserModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDOrderTicketUserModel.
	DOrderTicketUserModel interface {
		dOrderTicketUserModel
		withSession(session sqlx.Session) DOrderTicketUserModel
		FindByOrderNumber(ctx context.Context, orderNumber int64) ([]*DOrderTicketUser, error)
		InsertBatch(ctx context.Context, session sqlx.Session, rows []*DOrderTicketUser) error
		UpdateCancelStatusByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64, cancelTime time.Time) error
		UpdatePayStatusByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64, payTime time.Time) error
		UpdateRefundStatusByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64, refundTime time.Time) error
	}

	customDOrderTicketUserModel struct {
		*defaultDOrderTicketUserModel
	}
)

// NewDOrderTicketUserModel returns a model for the database table.
func NewDOrderTicketUserModel(conn sqlx.SqlConn) DOrderTicketUserModel {
	return &customDOrderTicketUserModel{
		defaultDOrderTicketUserModel: newDOrderTicketUserModel(conn),
	}
}

func (m *customDOrderTicketUserModel) withSession(session sqlx.Session) DOrderTicketUserModel {
	return NewDOrderTicketUserModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDOrderTicketUserModel) FindByOrderNumber(ctx context.Context, orderNumber int64) ([]*DOrderTicketUser, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `order_number` = ? order by `id` asc",
		dOrderTicketUserRows,
		m.table,
	)

	var resp []*DOrderTicketUser
	err := m.conn.QueryRowsCtx(ctx, &resp, query, orderNumber)
	switch err {
	case nil:
		return resp, nil
	case sqlx.ErrNotFound:
		return []*DOrderTicketUser{}, nil
	default:
		return nil, err
	}
}

func (m *customDOrderTicketUserModel) InsertBatch(ctx context.Context, session sqlx.Session, rows []*DOrderTicketUser) error {
	if len(rows) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(rows))
	args := make([]interface{}, 0, len(rows)*18)
	for _, row := range rows {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			row.Id,
			row.OrderNumber,
			row.UserId,
			row.TicketUserId,
			row.TicketUserName,
			row.TicketUserIdNumber,
			row.TicketCategoryId,
			row.TicketCategoryName,
			row.TicketPrice,
			row.SeatId,
			row.SeatRow,
			row.SeatCol,
			row.SeatPrice,
			row.OrderStatus,
			row.CreateOrderTime,
			row.CreateTime,
			row.EditTime,
			row.Status,
		)
	}

	query := fmt.Sprintf(
		"insert into %s (`id`, `order_number`, `user_id`, `ticket_user_id`, `ticket_user_name`, `ticket_user_id_number`, `ticket_category_id`, `ticket_category_name`, `ticket_price`, `seat_id`, `seat_row`, `seat_col`, `seat_price`, `order_status`, `create_order_time`, `create_time`, `edit_time`, `status`) values %s",
		m.table,
		strings.Join(placeholders, ", "),
	)

	_, err := m.withSession(session).(*customDOrderTicketUserModel).conn.ExecCtx(ctx, query, args...)
	return err
}

func (m *customDOrderTicketUserModel) UpdateCancelStatusByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64, cancelTime time.Time) error {
	query := fmt.Sprintf(
		"update %s set `order_status` = 2, `edit_time` = ? where `status` = 1 and `order_number` = ? and `order_status` = 1",
		m.table,
	)

	_, err := m.withSession(session).(*customDOrderTicketUserModel).conn.ExecCtx(ctx, query, cancelTime, orderNumber)
	return err
}

func (m *customDOrderTicketUserModel) UpdatePayStatusByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64, payTime time.Time) error {
	query := fmt.Sprintf(
		"update %s set `order_status` = 3, `edit_time` = ? where `status` = 1 and `order_number` = ? and `order_status` = 1",
		m.table,
	)

	_, err := m.withSession(session).(*customDOrderTicketUserModel).conn.ExecCtx(ctx, query, payTime, orderNumber)
	return err
}

func (m *customDOrderTicketUserModel) UpdateRefundStatusByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64, refundTime time.Time) error {
	query := fmt.Sprintf(
		"update %s set `order_status` = 4, `edit_time` = ? where `status` = 1 and `order_number` = ? and `order_status` in (2, 3)",
		m.table,
	)

	_, err := m.withSession(session).(*customDOrderTicketUserModel).conn.ExecCtx(ctx, query, refundTime, orderNumber)
	return err
}
