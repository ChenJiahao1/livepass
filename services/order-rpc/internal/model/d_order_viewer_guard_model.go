package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type DOrderViewerGuard struct {
	Id          int64     `db:"id"`
	OrderNumber int64     `db:"order_number"`
	ProgramId   int64     `db:"program_id"`
	ShowTimeId  int64     `db:"show_time_id"`
	ViewerId    int64     `db:"viewer_id"`
	CreateTime  time.Time `db:"create_time"`
	EditTime    time.Time `db:"edit_time"`
	Status      int64     `db:"status"`
}

type DOrderViewerGuardModel interface {
	InsertBatch(ctx context.Context, session sqlx.Session, rows []*DOrderViewerGuard) error
	DeleteByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64) error
	FindActiveByShowTimeAfterID(ctx context.Context, showTimeID, afterID, limit int64) ([]*DOrderViewerGuard, error)
}

type customDOrderViewerGuardModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewDOrderViewerGuardModelWithTable(conn sqlx.SqlConn, table string) DOrderViewerGuardModel {
	return &customDOrderViewerGuardModel{
		conn:  conn,
		table: normalizeTableName(table),
	}
}

func (m *customDOrderViewerGuardModel) withSession(session sqlx.Session) *customDOrderViewerGuardModel {
	return &customDOrderViewerGuardModel{
		conn:  sqlx.NewSqlConnFromSession(session),
		table: m.table,
	}
}

func (m *customDOrderViewerGuardModel) InsertBatch(ctx context.Context, session sqlx.Session, rows []*DOrderViewerGuard) error {
	if len(rows) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(rows))
	args := make([]interface{}, 0, len(rows)*8)
	for _, row := range rows {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args, row.Id, row.OrderNumber, row.ProgramId, row.ShowTimeId, row.ViewerId, row.CreateTime, row.EditTime, row.Status)
	}

	query := fmt.Sprintf(
		"insert into %s (`id`, `order_number`, `program_id`, `show_time_id`, `viewer_id`, `create_time`, `edit_time`, `status`) values %s",
		m.table,
		strings.Join(placeholders, ", "),
	)
	_, err := m.withSession(session).conn.ExecCtx(ctx, query, args...)
	return err
}

func (m *customDOrderViewerGuardModel) DeleteByOrderNumber(ctx context.Context, session sqlx.Session, orderNumber int64) error {
	query := fmt.Sprintf("delete from %s where `order_number` = ?", m.table)
	_, err := m.withSession(session).conn.ExecCtx(ctx, query, orderNumber)
	return err
}

func (m *customDOrderViewerGuardModel) FindActiveByShowTimeAfterID(ctx context.Context, showTimeID, afterID, limit int64) ([]*DOrderViewerGuard, error) {
	if limit <= 0 {
		limit = 100
	}

	query := fmt.Sprintf(
		"select `id`, `order_number`, `program_id`, `show_time_id`, `viewer_id`, `create_time`, `edit_time`, `status` from %s where `show_time_id` = ? and `status` = 1 and `id` > ? order by `id` asc limit ?",
		m.table,
	)
	var rows []*DOrderViewerGuard
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, showTimeID, afterID, limit); err != nil {
		if err == sqlx.ErrNotFound {
			return []*DOrderViewerGuard{}, nil
		}
		return nil, err
	}

	return rows, nil
}
