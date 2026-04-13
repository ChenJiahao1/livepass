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
	ShowTimeId  int64     `db:"show_time_id"`
	UserId      int64     `db:"user_id"`
	CreateTime  time.Time `db:"create_time"`
	EditTime    time.Time `db:"edit_time"`
	Status      int64     `db:"status"`
}

type DOrderUserGuardModel interface {
	InsertWithSession(ctx context.Context, session sqlx.Session, data *DOrderUserGuard) (sql.Result, error)
	DeleteByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64) error
	FindActiveByShowTimeAfterID(ctx context.Context, showTimeID, afterID, limit int64) ([]*DOrderUserGuard, error)
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
		"insert into %s (`id`, `order_number`, `program_id`, `show_time_id`, `user_id`, `create_time`, `edit_time`, `status`) values (?, ?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return m.withSession(session).conn.ExecCtx(
		ctx,
		query,
		data.Id,
		data.OrderNumber,
		data.ProgramId,
		data.ShowTimeId,
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

func (m *customDOrderUserGuardModel) FindActiveByShowTimeAfterID(ctx context.Context, showTimeID, afterID, limit int64) ([]*DOrderUserGuard, error) {
	if limit <= 0 {
		limit = 100
	}

	query := fmt.Sprintf(
		"select `id`, `order_number`, `program_id`, `show_time_id`, `user_id`, `create_time`, `edit_time`, `status` from %s where `show_time_id` = ? and `status` = 1 and `id` > ? order by `id` asc limit ?",
		m.table,
	)
	var rows []*DOrderUserGuard
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, showTimeID, afterID, limit); err != nil {
		if err == sqlx.ErrNotFound {
			return []*DOrderUserGuard{}, nil
		}
		return nil, err
	}

	return rows, nil
}
