package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DUserOrderIndexModel = (*customDUserOrderIndexModel)(nil)

type (
	// DUserOrderIndexModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDUserOrderIndexModel.
	DUserOrderIndexModel interface {
		dUserOrderIndexModel
		withSession(session sqlx.Session) DUserOrderIndexModel
		InsertWithSession(ctx context.Context, session sqlx.Session, data *DUserOrderIndex) (sql.Result, error)
		FindPageByUserAndStatus(ctx context.Context, userId, orderStatus, pageNumber, pageSize int64) ([]*DUserOrderIndex, int64, error)
		UpdateOrderStatusByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber, orderStatus int64, editTime time.Time) error
	}

	customDUserOrderIndexModel struct {
		*defaultDUserOrderIndexModel
	}
)

// NewDUserOrderIndexModel returns a model for the database table.
func NewDUserOrderIndexModel(conn sqlx.SqlConn) DUserOrderIndexModel {
	return &customDUserOrderIndexModel{
		defaultDUserOrderIndexModel: newDUserOrderIndexModel(conn),
	}
}

func NewDUserOrderIndexModelWithTable(conn sqlx.SqlConn, table string) DUserOrderIndexModel {
	m := newDUserOrderIndexModel(conn)
	m.table = normalizeTableName(table)
	return &customDUserOrderIndexModel{
		defaultDUserOrderIndexModel: m,
	}
}

func (m *customDUserOrderIndexModel) withSession(session sqlx.Session) DUserOrderIndexModel {
	return NewDUserOrderIndexModelWithTable(sqlx.NewSqlConnFromSession(session), rawTableName(m.table))
}

func (m *customDUserOrderIndexModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *DUserOrderIndex) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`id`, `order_number`, `user_id`, `program_id`, `order_status`, `ticket_count`, `order_price`, `create_order_time`, `create_time`, `edit_time`, `status`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return m.withSession(session).(*customDUserOrderIndexModel).conn.ExecCtx(
		ctx,
		query,
		data.Id,
		data.OrderNumber,
		data.UserId,
		data.ProgramId,
		data.OrderStatus,
		data.TicketCount,
		data.OrderPrice,
		data.CreateOrderTime,
		data.CreateTime,
		data.EditTime,
		data.Status,
	)
}

func (m *customDUserOrderIndexModel) FindPageByUserAndStatus(ctx context.Context, userId, orderStatus, pageNumber, pageSize int64) ([]*DUserOrderIndex, int64, error) {
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
		return []*DUserOrderIndex{}, 0, nil
	}

	offset := (pageNumber - 1) * pageSize
	query := fmt.Sprintf(
		"select %s from %s where %s order by `create_order_time` desc, `id` desc limit ? offset ?",
		dUserOrderIndexRows,
		m.table,
		whereClause,
	)
	queryArgs := append(append([]interface{}{}, args...), pageSize, offset)

	var resp []*DUserOrderIndex
	err := m.conn.QueryRowsCtx(ctx, &resp, query, queryArgs...)
	switch err {
	case nil:
		return resp, total.Total, nil
	case sqlx.ErrNotFound:
		return []*DUserOrderIndex{}, total.Total, nil
	default:
		return nil, 0, err
	}
}

func (m *customDUserOrderIndexModel) UpdateOrderStatusByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber, orderStatus int64, editTime time.Time) error {
	query := fmt.Sprintf(
		"update %s set `order_status` = ?, `edit_time` = ? where `status` = 1 and `order_number` = ? and `order_status` <> ?",
		m.table,
	)

	_, err := m.withSession(session).(*customDUserOrderIndexModel).conn.ExecCtx(ctx, query, orderStatus, editTime, orderNumber, orderStatus)
	return err
}
