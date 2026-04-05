package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type DOrderUserGuard struct {
	Id          int64     `db:"id"`
	OrderNumber int64     `db:"order_number"`
	ProgramId   int64     `db:"program_id"`
	UserId      int64     `db:"user_id"`
	CreateTime  time.Time `db:"create_time"`
	EditTime    time.Time `db:"edit_time"`
	Status      int64     `db:"status"`
}

type DOrderUserGuardModel interface {
	InsertWithSession(ctx context.Context, session sqlx.Session, data *DOrderUserGuard) (sql.Result, error)
	DeleteByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64) error
}

type customDOrderUserGuardModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewDOrderUserGuardModelWithTable(conn sqlx.SqlConn, table string) DOrderUserGuardModel {
	return &customDOrderUserGuardModel{
		conn:  conn,
		table: normalizeTableName(table),
	}
}

func (m *customDOrderUserGuardModel) withSession(session sqlx.Session) *customDOrderUserGuardModel {
	return &customDOrderUserGuardModel{
		conn:  sqlx.NewSqlConnFromSession(session),
		table: m.table,
	}
}

func (m *customDOrderUserGuardModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *DOrderUserGuard) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`id`, `order_number`, `program_id`, `user_id`, `create_time`, `edit_time`, `status`) values (?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return m.withSession(session).conn.ExecCtx(
		ctx,
		query,
		data.Id,
		data.OrderNumber,
		data.ProgramId,
		data.UserId,
		data.CreateTime,
		data.EditTime,
		data.Status,
	)
}

func (m *customDOrderUserGuardModel) DeleteByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64) error {
	query := fmt.Sprintf("delete from %s where `order_number` = ?", m.table)
	_, err := m.withSession(session).conn.ExecCtx(ctx, query, orderNumber)
	return err
}
