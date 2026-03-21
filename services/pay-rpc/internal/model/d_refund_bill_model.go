package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DRefundBillModel = (*customDRefundBillModel)(nil)

type (
	// DRefundBillModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDRefundBillModel.
	DRefundBillModel interface {
		dRefundBillModel
		withSession(session sqlx.Session) DRefundBillModel
		InsertWithSession(ctx context.Context, session sqlx.Session, data *DRefundBill) (sql.Result, error)
		FindOneByOrderNumberForUpdate(ctx context.Context, session sqlx.Session, orderNumber int64) (*DRefundBill, error)
	}

	customDRefundBillModel struct {
		*defaultDRefundBillModel
	}
)

// NewDRefundBillModel returns a model for the database table.
func NewDRefundBillModel(conn sqlx.SqlConn) DRefundBillModel {
	return &customDRefundBillModel{
		defaultDRefundBillModel: newDRefundBillModel(conn),
	}
}

func (m *customDRefundBillModel) withSession(session sqlx.Session) DRefundBillModel {
	return NewDRefundBillModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDRefundBillModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *DRefundBill) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`id`, `refund_bill_no`, `order_number`, `pay_bill_id`, `user_id`, `refund_amount`, `refund_status`, `refund_reason`, `refund_time`, `create_time`, `edit_time`, `status`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return m.withSession(session).(*customDRefundBillModel).conn.ExecCtx(
		ctx,
		query,
		data.Id,
		data.RefundBillNo,
		data.OrderNumber,
		data.PayBillId,
		data.UserId,
		data.RefundAmount,
		data.RefundStatus,
		data.RefundReason,
		data.RefundTime,
		data.CreateTime,
		data.EditTime,
		data.Status,
	)
}

func (m *customDRefundBillModel) FindOneByOrderNumberForUpdate(ctx context.Context, session sqlx.Session, orderNumber int64) (*DRefundBill, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `order_number` = ? limit 1 for update",
		dRefundBillRows,
		m.table,
	)

	var resp DRefundBill
	err := m.withSession(session).(*customDRefundBillModel).conn.QueryRowCtx(ctx, &resp, query, orderNumber)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
