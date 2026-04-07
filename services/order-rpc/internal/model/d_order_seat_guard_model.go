package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type DOrderSeatGuard struct {
	Id          int64     `db:"id"`
	OrderNumber int64     `db:"order_number"`
	ProgramId   int64     `db:"program_id"`
	ShowTimeId  int64     `db:"show_time_id"`
	SeatId      int64     `db:"seat_id"`
	CreateTime  time.Time `db:"create_time"`
	EditTime    time.Time `db:"edit_time"`
	Status      int64     `db:"status"`
}

type DOrderSeatGuardModel interface {
	InsertBatch(ctx context.Context, session sqlx.Session, rows []*DOrderSeatGuard) error
	DeleteByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64) error
}

type customDOrderSeatGuardModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewDOrderSeatGuardModelWithTable(conn sqlx.SqlConn, table string) DOrderSeatGuardModel {
	return &customDOrderSeatGuardModel{
		conn:  conn,
		table: normalizeTableName(table),
	}
}

func (m *customDOrderSeatGuardModel) withSession(session sqlx.Session) *customDOrderSeatGuardModel {
	return &customDOrderSeatGuardModel{
		conn:  sqlx.NewSqlConnFromSession(session),
		table: m.table,
	}
}

func (m *customDOrderSeatGuardModel) InsertBatch(ctx context.Context, session sqlx.Session, rows []*DOrderSeatGuard) error {
	if len(rows) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(rows))
	args := make([]interface{}, 0, len(rows)*8)
	for _, row := range rows {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args, row.Id, row.OrderNumber, row.ProgramId, row.ShowTimeId, row.SeatId, row.CreateTime, row.EditTime, row.Status)
	}

	query := fmt.Sprintf(
		"insert into %s (`id`, `order_number`, `program_id`, `show_time_id`, `seat_id`, `create_time`, `edit_time`, `status`) values %s",
		m.table,
		strings.Join(placeholders, ", "),
	)
	_, err := m.withSession(session).conn.ExecCtx(ctx, query, args...)
	return err
}

func (m *customDOrderSeatGuardModel) DeleteByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64) error {
	query := fmt.Sprintf("delete from %s where `order_number` = ?", m.table)
	_, err := m.withSession(session).conn.ExecCtx(ctx, query, orderNumber)
	return err
}
