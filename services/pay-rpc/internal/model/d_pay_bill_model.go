package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DPayBillModel = (*customDPayBillModel)(nil)

type (
	// DPayBillModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDPayBillModel.
	DPayBillModel interface {
		dPayBillModel
		withSession(session sqlx.Session) DPayBillModel
		InsertWithSession(ctx context.Context, session sqlx.Session, data *DPayBill) (sql.Result, error)
		FindOneByOrderNumberForUpdate(ctx context.Context, session sqlx.Session, orderNumber int64) (*DPayBill, error)
	}

	customDPayBillModel struct {
		*defaultDPayBillModel
	}
)

// NewDPayBillModel returns a model for the database table.
func NewDPayBillModel(conn sqlx.SqlConn) DPayBillModel {
	return &customDPayBillModel{
		defaultDPayBillModel: newDPayBillModel(conn),
	}
}

func (m *customDPayBillModel) withSession(session sqlx.Session) DPayBillModel {
	return NewDPayBillModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDPayBillModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *DPayBill) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`id`, `pay_bill_no`, `order_number`, `user_id`, `subject`, `channel`, `order_amount`, `pay_status`, `pay_time`, `create_time`, `edit_time`, `status`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return m.withSession(session).(*customDPayBillModel).conn.ExecCtx(
		ctx,
		query,
		data.Id,
		data.PayBillNo,
		data.OrderNumber,
		data.UserId,
		data.Subject,
		data.Channel,
		data.OrderAmount,
		data.PayStatus,
		data.PayTime,
		data.CreateTime,
		data.EditTime,
		data.Status,
	)
}

func (m *customDPayBillModel) FindOneByOrderNumberForUpdate(ctx context.Context, session sqlx.Session, orderNumber int64) (*DPayBill, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `order_number` = ? limit 1 for update",
		dPayBillRows,
		m.table,
	)

	var resp DPayBill
	err := m.withSession(session).(*customDPayBillModel).conn.QueryRowCtx(ctx, &resp, query, orderNumber)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
