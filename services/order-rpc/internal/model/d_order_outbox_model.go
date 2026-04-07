package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type DOrderOutbox struct {
	Id              int64        `db:"id"`
	OrderNumber     int64        `db:"order_number"`
	ShowTimeId      int64        `db:"show_time_id"`
	EventType       string       `db:"event_type"`
	Payload         string       `db:"payload"`
	PublishedStatus int64        `db:"published_status"`
	PublishedTime   sql.NullTime `db:"published_time"`
	CreateTime      time.Time    `db:"create_time"`
	EditTime        time.Time    `db:"edit_time"`
	Status          int64        `db:"status"`
}

type DOrderOutboxModel interface {
	InsertBatch(ctx context.Context, session sqlx.Session, rows []*DOrderOutbox) error
}

type customDOrderOutboxModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewDOrderOutboxModelWithTable(conn sqlx.SqlConn, table string) DOrderOutboxModel {
	return &customDOrderOutboxModel{
		conn:  conn,
		table: normalizeTableName(table),
	}
}

func (m *customDOrderOutboxModel) withSession(session sqlx.Session) *customDOrderOutboxModel {
	return &customDOrderOutboxModel{
		conn:  sqlx.NewSqlConnFromSession(session),
		table: m.table,
	}
}

func (m *customDOrderOutboxModel) InsertBatch(ctx context.Context, session sqlx.Session, rows []*DOrderOutbox) error {
	if len(rows) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(rows))
	args := make([]interface{}, 0, len(rows)*10)
	for _, row := range rows {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			row.Id,
			row.OrderNumber,
			row.ShowTimeId,
			row.EventType,
			row.Payload,
			row.PublishedStatus,
			row.PublishedTime,
			row.CreateTime,
			row.EditTime,
			row.Status,
		)
	}

	query := fmt.Sprintf(
		"insert into %s (`id`, `order_number`, `show_time_id`, `event_type`, `payload`, `published_status`, `published_time`, `create_time`, `edit_time`, `status`) values %s",
		m.table,
		strings.Join(placeholders, ", "),
	)
	_, err := m.withSession(session).conn.ExecCtx(ctx, query, args...)
	return err
}
