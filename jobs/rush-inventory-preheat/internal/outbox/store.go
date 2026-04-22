package outbox

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"livepass/pkg/delaytask"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type TaskRef struct {
	DBKey string
	ID    int64
}

type Task struct {
	Ref             TaskRef
	TaskType        string
	TaskKey         string
	Payload         string
	ExecuteAt       time.Time
	TaskStatus      int64
	PublishAttempts int64
}

type Store interface {
	ListDispatchableByTaskType(ctx context.Context, taskType string, limit int64) ([]Task, error)
	MarkPublished(ctx context.Context, ref TaskRef, publishedAt time.Time) error
	MarkPublishFailed(ctx context.Context, ref TaskRef, failedAt time.Time, publishErr string) error
}

type delayTaskOutboxRow struct {
	ID              int64     `db:"id"`
	TaskType        string    `db:"task_type"`
	TaskKey         string    `db:"task_key"`
	Payload         string    `db:"payload"`
	ExecuteAt       time.Time `db:"execute_at"`
	TaskStatus      int64     `db:"task_status"`
	PublishAttempts int64     `db:"publish_attempts"`
}

type mysqlStore struct {
	conns map[string]sqlx.SqlConn
}

func DispatchableStatuses() []int64 {
	return []int64{
		delaytask.OutboxTaskStatusPending,
		delaytask.OutboxTaskStatusPublished,
		delaytask.OutboxTaskStatusFailed,
	}
}

func NewMysqlStore(conns map[string]sqlx.SqlConn) Store {
	if len(conns) == 0 {
		return nil
	}
	return &mysqlStore{conns: conns}
}

func (s *mysqlStore) ListDispatchableByTaskType(ctx context.Context, taskType string, limit int64) ([]Task, error) {
	if s == nil || len(s.conns) == 0 || limit <= 0 {
		return nil, nil
	}

	dbKeys := make([]string, 0, len(s.conns))
	for dbKey := range s.conns {
		dbKeys = append(dbKeys, dbKey)
	}
	sort.Strings(dbKeys)

	items := make([]Task, 0, limit)
	for _, dbKey := range dbKeys {
		var rows []delayTaskOutboxRow
		err := s.conns[dbKey].QueryRowsCtx(
			ctx,
			&rows,
			"SELECT `id`, `task_type`, `task_key`, `payload`, `execute_at`, `task_status`, `publish_attempts` FROM `d_delay_task_outbox` WHERE `task_status` IN (0, 1, 4) AND `task_type` = ? AND `status` = 1 ORDER BY `id` ASC LIMIT ?",
			taskType,
			limit,
		)
		switch {
		case err == nil:
		case errors.Is(err, sqlx.ErrNotFound):
			continue
		default:
			return nil, err
		}

		for _, row := range rows {
			items = append(items, Task{
				Ref: TaskRef{
					DBKey: dbKey,
					ID:    row.ID,
				},
				TaskType:        row.TaskType,
				TaskKey:         row.TaskKey,
				Payload:         row.Payload,
				ExecuteAt:       row.ExecuteAt,
				TaskStatus:      row.TaskStatus,
				PublishAttempts: row.PublishAttempts,
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Ref.ID == items[j].Ref.ID {
			return items[i].Ref.DBKey < items[j].Ref.DBKey
		}
		return items[i].Ref.ID < items[j].Ref.ID
	})
	if int64(len(items)) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *mysqlStore) MarkPublished(ctx context.Context, ref TaskRef, publishedAt time.Time) error {
	conn, err := s.connForRef(ref)
	if err != nil {
		return err
	}

	_, err = conn.ExecCtx(
		ctx,
		"UPDATE `d_delay_task_outbox` SET `task_status` = 1, `publish_attempts` = `publish_attempts` + 1, `published_time` = ?, `last_publish_error` = '', `edit_time` = ? WHERE `id` = ? AND `task_status` IN (0, 1, 4) AND `status` = 1",
		publishedAt,
		publishedAt,
		ref.ID,
	)
	return err
}

func (s *mysqlStore) MarkPublishFailed(ctx context.Context, ref TaskRef, failedAt time.Time, publishErr string) error {
	conn, err := s.connForRef(ref)
	if err != nil {
		return err
	}

	_, err = conn.ExecCtx(
		ctx,
		"UPDATE `d_delay_task_outbox` SET `task_status` = 4, `publish_attempts` = `publish_attempts` + 1, `last_publish_error` = ?, `edit_time` = ? WHERE `id` = ? AND `task_status` IN (0, 1, 4) AND `status` = 1",
		publishErr,
		failedAt,
		ref.ID,
	)
	return err
}

func (s *mysqlStore) connForRef(ref TaskRef) (sqlx.SqlConn, error) {
	if s == nil {
		return nil, fmt.Errorf("delay task store is nil")
	}
	conn, ok := s.conns[ref.DBKey]
	if !ok {
		return nil, fmt.Errorf("delay task shard not configured: %s", ref.DBKey)
	}
	return conn, nil
}
