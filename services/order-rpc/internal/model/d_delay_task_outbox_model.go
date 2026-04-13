package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type DDelayTaskOutbox struct {
	Id               int64        `db:"id"`
	TaskType         string       `db:"task_type"`
	TaskKey          string       `db:"task_key"`
	Payload          string       `db:"payload"`
	ExecuteAt        time.Time    `db:"execute_at"`
	PublishedStatus  int64        `db:"published_status"`
	PublishAttempts  int64        `db:"publish_attempts"`
	LastPublishError string       `db:"last_publish_error"`
	PublishedTime    sql.NullTime `db:"published_time"`
	CreateTime       time.Time    `db:"create_time"`
	EditTime         time.Time    `db:"edit_time"`
	Status           int64        `db:"status"`
}

type DDelayTaskOutboxModel interface {
	InsertBatch(ctx context.Context, session sqlx.Session, rows []*DDelayTaskOutbox) error
}

type customDDelayTaskOutboxModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewDDelayTaskOutboxModelWithTable(conn sqlx.SqlConn, table string) DDelayTaskOutboxModel {
	return &customDDelayTaskOutboxModel{
		conn:  conn,
		table: normalizeTableName(table),
	}
}

func (m *customDDelayTaskOutboxModel) withSession(session sqlx.Session) *customDDelayTaskOutboxModel {
	return &customDDelayTaskOutboxModel{
		conn:  sqlx.NewSqlConnFromSession(session),
		table: m.table,
	}
}

func (m *customDDelayTaskOutboxModel) InsertBatch(ctx context.Context, session sqlx.Session, rows []*DDelayTaskOutbox) error {
	if len(rows) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(rows))
	args := make([]interface{}, 0, len(rows)*12)
	for _, row := range rows {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			row.Id,
			row.TaskType,
			row.TaskKey,
			row.Payload,
			row.ExecuteAt,
			row.PublishedStatus,
			row.PublishAttempts,
			row.LastPublishError,
			row.PublishedTime,
			row.CreateTime,
			row.EditTime,
			row.Status,
		)
	}

	query := fmt.Sprintf(
		"insert into %s (`id`, `task_type`, `task_key`, `payload`, `execute_at`, `published_status`, `publish_attempts`, `last_publish_error`, `published_time`, `create_time`, `edit_time`, `status`) values %s",
		m.table,
		strings.Join(placeholders, ", "),
	)
	_, err := m.withSession(session).conn.ExecCtx(ctx, query, args...)
	return err
}
