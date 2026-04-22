package model

import (
	"context"
	"database/sql"
	"errors"
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
	TaskStatus       int64        `db:"task_status"`
	PublishAttempts  int64        `db:"publish_attempts"`
	ConsumeAttempts  int64        `db:"consume_attempts"`
	LastPublishError string       `db:"last_publish_error"`
	LastConsumeError string       `db:"last_consume_error"`
	PublishedTime    sql.NullTime `db:"published_time"`
	ProcessedTime    sql.NullTime `db:"processed_time"`
	CreateTime       time.Time    `db:"create_time"`
	EditTime         time.Time    `db:"edit_time"`
	Status           int64        `db:"status"`
}

type DDelayTaskOutboxModel interface {
	InsertBatch(ctx context.Context, session sqlx.Session, rows []*DDelayTaskOutbox) error
	MarkProcessed(ctx context.Context, session sqlx.Session, taskType, taskKey string, processedAt time.Time) (int64, int64, error)
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
	args := make([]interface{}, 0, len(rows)*15)
	for _, row := range rows {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			row.Id,
			row.TaskType,
			row.TaskKey,
			row.Payload,
			row.ExecuteAt,
			row.TaskStatus,
			row.PublishAttempts,
			row.ConsumeAttempts,
			row.LastPublishError,
			row.LastConsumeError,
			row.PublishedTime,
			row.ProcessedTime,
			row.CreateTime,
			row.EditTime,
			row.Status,
		)
	}

	query := fmt.Sprintf(
		"insert into %s (`id`, `task_type`, `task_key`, `payload`, `execute_at`, `task_status`, `publish_attempts`, `consume_attempts`, `last_publish_error`, `last_consume_error`, `published_time`, `processed_time`, `create_time`, `edit_time`, `status`) values %s ON DUPLICATE KEY UPDATE `payload` = VALUES(`payload`), `execute_at` = VALUES(`execute_at`), `task_status` = 0, `publish_attempts` = 0, `consume_attempts` = 0, `last_publish_error` = '', `last_consume_error` = '', `published_time` = NULL, `processed_time` = NULL, `edit_time` = VALUES(`edit_time`), `status` = 1",
		m.table,
		strings.Join(placeholders, ", "),
	)
	_, err := m.withSession(session).conn.ExecCtx(ctx, query, args...)
	return err
}

func (m *customDDelayTaskOutboxModel) MarkProcessed(ctx context.Context, session sqlx.Session, taskType, taskKey string, processedAt time.Time) (int64, int64, error) {
	var row struct {
		TaskStatus      int64 `db:"task_status"`
		ConsumeAttempts int64 `db:"consume_attempts"`
	}
	err := m.withSession(session).conn.QueryRowCtx(
		ctx,
		&row,
		fmt.Sprintf("SELECT `task_status`, `consume_attempts` FROM %s WHERE `task_type` = ? AND `task_key` = ? AND `status` = 1 LIMIT 1", m.table),
		taskType,
		taskKey,
	)
	switch {
	case err == nil:
	case errors.Is(err, sqlx.ErrNotFound), errors.Is(err, sql.ErrNoRows):
		return 0, 0, nil
	default:
		return 0, 0, err
	}

	query := fmt.Sprintf(
		"UPDATE %s SET `task_status` = 3, `consume_attempts` = `consume_attempts` + 1, `last_consume_error` = '', `processed_time` = ?, `edit_time` = ? WHERE `task_type` = ? AND `task_key` = ? AND `status` = 1",
		m.table,
	)
	_, err = m.withSession(session).conn.ExecCtx(ctx, query, processedAt, processedAt, taskType, taskKey)
	if err != nil {
		return 0, 0, err
	}
	return row.TaskStatus, row.ConsumeAttempts + 1, nil
}
