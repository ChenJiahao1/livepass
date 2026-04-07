package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DOrderModel = (*customDOrderModel)(nil)

type (
	// DOrderModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDOrderModel.
	DOrderModel interface {
		dOrderModel
		withSession(session sqlx.Session) DOrderModel
		InsertWithSession(ctx context.Context, session sqlx.Session, data *DOrder) (sql.Result, error)
		FindOneByOrderNumber(ctx context.Context, orderNumber int64) (*DOrder, error)
		FindOneByOrderNumberForUpdate(ctx context.Context, session sqlx.Session, orderNumber int64) (*DOrder, error)
		FindPageByUserAndStatus(ctx context.Context, userId, orderStatus, pageNumber, pageSize int64) ([]*DOrder, int64, error)
		CountByUserShowTimeAndStatus(ctx context.Context, userId, showTimeId, orderStatus int64) (int64, error)
		CountActiveTicketsByUserShowTime(ctx context.Context, userId, showTimeId int64) (int64, error)
		ListUnpaidReservationsByUserShowTime(ctx context.Context, userId, showTimeId int64) (map[int64]int64, error)
		FindExpiredUnpaid(ctx context.Context, before time.Time, limit int64) ([]*DOrder, error)
		UpdateCancelStatus(ctx context.Context, session sqlx.Session, orderNumber int64, cancelTime time.Time) error
		UpdatePayStatus(ctx context.Context, session sqlx.Session, orderNumber int64, payTime time.Time) error
		UpdateRefundStatus(ctx context.Context, session sqlx.Session, orderNumber int64, refundTime time.Time) error
	}

	customDOrderModel struct {
		*defaultDOrderModel
	}

	sumAggregate struct {
		Total int64 `db:"total"`
	}

	orderReservation struct {
		OrderNumber int64 `db:"order_number"`
		TicketCount int64 `db:"ticket_count"`
	}
)

// NewDOrderModel returns a model for the database table.
func NewDOrderModel(conn sqlx.SqlConn) DOrderModel {
	return &customDOrderModel{
		defaultDOrderModel: newDOrderModel(conn),
	}
}

func NewDOrderModelWithTable(conn sqlx.SqlConn, table string) DOrderModel {
	m := newDOrderModel(conn)
	m.table = normalizeTableName(table)
	return &customDOrderModel{
		defaultDOrderModel: m,
	}
}

func (m *customDOrderModel) withSession(session sqlx.Session) DOrderModel {
	return NewDOrderModelWithTable(sqlx.NewSqlConnFromSession(session), rawTableName(m.table))
}

func (m *customDOrderModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *DOrder) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`id`, `order_number`, `program_id`, `show_time_id`, `program_title`, `program_item_picture`, `program_place`, `program_show_time`, `program_permit_choose_seat`, `user_id`, `distribution_mode`, `take_ticket_mode`, `ticket_count`, `order_price`, `order_status`, `freeze_token`, `order_expire_time`, `create_order_time`, `cancel_order_time`, `create_time`, `edit_time`, `status`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return m.withSession(session).(*customDOrderModel).conn.ExecCtx(
		ctx,
		query,
		data.Id,
		data.OrderNumber,
		data.ProgramId,
		data.ShowTimeId,
		data.ProgramTitle,
		data.ProgramItemPicture,
		data.ProgramPlace,
		data.ProgramShowTime,
		data.ProgramPermitChooseSeat,
		data.UserId,
		data.DistributionMode,
		data.TakeTicketMode,
		data.TicketCount,
		data.OrderPrice,
		data.OrderStatus,
		data.FreezeToken,
		data.OrderExpireTime,
		data.CreateOrderTime,
		data.CancelOrderTime,
		data.CreateTime,
		data.EditTime,
		data.Status,
	)
}

func (m *customDOrderModel) FindOneByOrderNumber(ctx context.Context, orderNumber int64) (*DOrder, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `order_number` = ? limit 1",
		dOrderRows,
		m.table,
	)

	var resp DOrder
	err := m.conn.QueryRowCtx(ctx, &resp, query, orderNumber)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customDOrderModel) FindOneByOrderNumberForUpdate(ctx context.Context, session sqlx.Session, orderNumber int64) (*DOrder, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `order_number` = ? limit 1 for update",
		dOrderRows,
		m.table,
	)

	var resp DOrder
	err := m.withSession(session).(*customDOrderModel).conn.QueryRowCtx(ctx, &resp, query, orderNumber)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customDOrderModel) FindPageByUserAndStatus(ctx context.Context, userId, orderStatus, pageNumber, pageSize int64) ([]*DOrder, int64, error) {
	whereClause := "`status` = 1 and `user_id` = ?"
	args := []interface{}{userId}
	if orderStatus > 0 {
		whereClause += " and `order_status` = ?"
		args = append(args, orderStatus)
	}

	countQuery := fmt.Sprintf("select count(1) as `total` from %s where %s", m.table, whereClause)
	var total sumAggregate
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total.Total == 0 {
		return []*DOrder{}, 0, nil
	}

	offset := (pageNumber - 1) * pageSize
	query := fmt.Sprintf(
		"select %s from %s where %s order by `create_order_time` desc, `id` desc limit ? offset ?",
		dOrderRows,
		m.table,
		whereClause,
	)
	queryArgs := append(append([]interface{}{}, args...), pageSize, offset)

	var resp []*DOrder
	err := m.conn.QueryRowsCtx(ctx, &resp, query, queryArgs...)
	switch err {
	case nil:
		return resp, total.Total, nil
	case sqlx.ErrNotFound:
		return []*DOrder{}, total.Total, nil
	default:
		return nil, 0, err
	}
}

func (m *customDOrderModel) CountByUserShowTimeAndStatus(ctx context.Context, userId, showTimeId, orderStatus int64) (int64, error) {
	query := fmt.Sprintf(
		"select coalesce(sum(`ticket_count`), 0) as `total` from %s where `status` = 1 and `user_id` = ? and `show_time_id` = ? and `order_status` = ?",
		m.table,
	)

	var total sumAggregate
	if err := m.conn.QueryRowCtx(ctx, &total, query, userId, showTimeId, orderStatus); err != nil {
		return 0, err
	}

	return total.Total, nil
}

func (m *customDOrderModel) CountActiveTicketsByUserShowTime(ctx context.Context, userId, showTimeId int64) (int64, error) {
	query := fmt.Sprintf(
		"select coalesce(sum(`ticket_count`), 0) as `total` from %s where `status` = 1 and `user_id` = ? and `show_time_id` = ? and `order_status` in (1, 3)",
		m.table,
	)

	var total sumAggregate
	if err := m.conn.QueryRowCtx(ctx, &total, query, userId, showTimeId); err != nil {
		return 0, err
	}

	return total.Total, nil
}

func (m *customDOrderModel) ListUnpaidReservationsByUserShowTime(ctx context.Context, userId, showTimeId int64) (map[int64]int64, error) {
	query := fmt.Sprintf(
		"select `order_number`, `ticket_count` from %s where `status` = 1 and `user_id` = ? and `show_time_id` = ? and `order_status` = 1",
		m.table,
	)

	var rows []*orderReservation
	err := m.conn.QueryRowsCtx(ctx, &rows, query, userId, showTimeId)
	switch err {
	case nil:
	case sqlx.ErrNotFound:
		return map[int64]int64{}, nil
	default:
		return nil, err
	}

	reservations := make(map[int64]int64, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		reservations[row.OrderNumber] = row.TicketCount
	}

	return reservations, nil
}

func (m *customDOrderModel) FindExpiredUnpaid(ctx context.Context, before time.Time, limit int64) ([]*DOrder, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `order_status` = 1 and `order_expire_time` <= ? order by `order_expire_time` asc, `id` asc limit ?",
		dOrderRows,
		m.table,
	)

	var resp []*DOrder
	err := m.conn.QueryRowsCtx(ctx, &resp, query, before, limit)
	switch err {
	case nil:
		return resp, nil
	case sqlx.ErrNotFound:
		return []*DOrder{}, nil
	default:
		return nil, err
	}
}

func (m *customDOrderModel) UpdateCancelStatus(ctx context.Context, session sqlx.Session, orderNumber int64, cancelTime time.Time) error {
	query := fmt.Sprintf(
		"update %s set `order_status` = 2, `cancel_order_time` = ?, `edit_time` = ? where `status` = 1 and `order_number` = ? and `order_status` = 1",
		m.table,
	)

	_, err := m.withSession(session).(*customDOrderModel).conn.ExecCtx(ctx, query, cancelTime, cancelTime, orderNumber)
	return err
}

func (m *customDOrderModel) UpdatePayStatus(ctx context.Context, session sqlx.Session, orderNumber int64, payTime time.Time) error {
	query := fmt.Sprintf(
		"update %s set `order_status` = 3, `pay_order_time` = ?, `edit_time` = ? where `status` = 1 and `order_number` = ? and `order_status` = 1",
		m.table,
	)

	_, err := m.withSession(session).(*customDOrderModel).conn.ExecCtx(ctx, query, payTime, payTime, orderNumber)
	return err
}

func (m *customDOrderModel) UpdateRefundStatus(ctx context.Context, session sqlx.Session, orderNumber int64, refundTime time.Time) error {
	query := fmt.Sprintf(
		"update %s set `order_status` = 4, `edit_time` = ? where `status` = 1 and `order_number` = ? and `order_status` in (2, 3)",
		m.table,
	)

	_, err := m.withSession(session).(*customDOrderModel).conn.ExecCtx(ctx, query, refundTime, orderNumber)
	return err
}
