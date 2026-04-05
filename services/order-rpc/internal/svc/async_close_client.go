package svc

import (
	"context"
	"fmt"
	"time"

	"damai-go/services/order-rpc/closequeue"
	"damai-go/services/order-rpc/internal/config"

	"github.com/hibiken/asynq"
)

type asynqEnqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type asynqAsyncCloseClient struct {
	enqueuer  asynqEnqueuer
	queue     string
	maxRetry  int
	uniqueTTL time.Duration
}

func newAsyncCloseClient(cfg config.AsyncCloseConfig) (AsyncCloseClient, error) {
	if !cfg.Enable || cfg.Redis.Host == "" {
		return nil, nil
	}
	if cfg.Redis.Type != "" && cfg.Redis.Type != "node" {
		return nil, fmt.Errorf("unsupported async close redis type: %s", cfg.Redis.Type)
	}

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Host,
		Username: cfg.Redis.User,
		Password: cfg.Redis.Pass,
	})

	return &asynqAsyncCloseClient{
		enqueuer:  client,
		queue:     cfg.Queue,
		maxRetry:  cfg.MaxRetry,
		uniqueTTL: cfg.UniqueTTL,
	}, nil
}

func (c *asynqAsyncCloseClient) EnqueueCloseTimeout(ctx context.Context, orderNumber int64, expireAt time.Time) error {
	body, err := closequeue.MarshalCloseTimeoutPayload(orderNumber, expireAt)
	if err != nil {
		return err
	}

	opts := []asynq.Option{
		asynq.Queue(c.queue),
		asynq.ProcessAt(expireAt),
		asynq.TaskID(closequeue.CloseTimeoutTaskID(orderNumber)),
	}
	if c.maxRetry > 0 {
		opts = append(opts, asynq.MaxRetry(c.maxRetry))
	}
	if c.uniqueTTL > 0 {
		opts = append(opts, asynq.Unique(c.uniqueTTL))
	}

	_, err = c.enqueuer.EnqueueContext(ctx, asynq.NewTask(closequeue.TaskTypeCloseTimeout, body), opts...)
	return err
}

func (c *asynqAsyncCloseClient) EnqueueVerifyAttemptDue(ctx context.Context, orderNumber, programID int64, dueAt time.Time) error {
	body, err := closequeue.MarshalVerifyAttemptPayload(orderNumber, programID, dueAt)
	if err != nil {
		return err
	}

	opts := []asynq.Option{
		asynq.Queue(c.queue),
		asynq.ProcessAt(dueAt),
		asynq.TaskID(closequeue.VerifyAttemptTaskID(orderNumber)),
	}
	if c.maxRetry > 0 {
		opts = append(opts, asynq.MaxRetry(c.maxRetry))
	}
	if c.uniqueTTL > 0 {
		opts = append(opts, asynq.Unique(c.uniqueTTL))
	}

	_, err = c.enqueuer.EnqueueContext(ctx, asynq.NewTask(closequeue.TaskTypeVerifyAttemptDue, body), opts...)
	return err
}
