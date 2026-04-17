package svc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"livepass/jobs/rush-inventory-preheat/taskdef"
	"livepass/pkg/xid"
	"livepass/services/program-rpc/internal/config"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type RushInventoryPreheatClient interface {
	Enqueue(ctx context.Context, programID int64, expectedOpenTime time.Time) error
}

type outboxRushInventoryPreheatClient struct {
	conn     sqlx.SqlConn
	leadTime time.Duration
}

func newRushInventoryPreheatClient(conn sqlx.SqlConn, cfg config.RushInventoryPreheatConfig) (RushInventoryPreheatClient, error) {
	if !cfg.Enable {
		return nil, nil
	}
	if conn == nil {
		return nil, nil
	}

	return &outboxRushInventoryPreheatClient{
		conn:     conn,
		leadTime: cfg.LeadTime,
	}, nil
}

func (c *outboxRushInventoryPreheatClient) Enqueue(ctx context.Context, programID int64, expectedOpenTime time.Time) error {
	return c.EnqueueWithConn(ctx, c.conn, programID, expectedOpenTime)
}

func (c *outboxRushInventoryPreheatClient) EnqueueWithConn(ctx context.Context, conn sqlx.SqlConn, programID int64, expectedOpenTime time.Time) error {
	message, err := taskdef.NewMessage(programID, expectedOpenTime, c.leadTime)
	if err != nil {
		return err
	}
	if conn == nil {
		return fmt.Errorf("rush inventory preheat outbox conn is nil")
	}

	now := time.Now()
	_, err = conn.ExecCtx(
		ctx,
		`INSERT INTO d_delay_task_outbox (
			id, task_type, task_key, payload, execute_at, published_status, publish_attempts,
			last_publish_error, published_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, 0, 0, '', ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE
			payload = VALUES(payload),
			execute_at = VALUES(execute_at),
			published_status = 0,
			publish_attempts = 0,
			last_publish_error = '',
			published_time = NULL,
			edit_time = VALUES(edit_time),
			status = 1`,
		xid.New(),
		message.Type,
		message.Key,
		string(message.Payload),
		message.ExecuteAt,
		sql.NullTime{},
		now,
		now,
	)
	return err
}
