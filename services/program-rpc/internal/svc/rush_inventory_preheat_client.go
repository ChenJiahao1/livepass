package svc

import (
	"context"
	"fmt"
	"time"

	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/preheatqueue"

	"github.com/hibiken/asynq"
)

type asynqEnqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type RushInventoryPreheatClient interface {
	Enqueue(ctx context.Context, showTimeID int64, expectedOpenTime time.Time) error
}

type asynqRushInventoryPreheatClient struct {
	enqueuer  asynqEnqueuer
	queue     string
	leadTime  time.Duration
	maxRetry  int
	uniqueTTL time.Duration
}

func newRushInventoryPreheatClient(cfg config.RushInventoryPreheatConfig) (RushInventoryPreheatClient, error) {
	if !cfg.Enable || cfg.Redis.Host == "" {
		return nil, nil
	}
	if cfg.Redis.Type != "" && cfg.Redis.Type != "node" {
		return nil, fmt.Errorf("unsupported rush inventory preheat redis type: %s", cfg.Redis.Type)
	}

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Host,
		Username: cfg.Redis.User,
		Password: cfg.Redis.Pass,
	})

	return &asynqRushInventoryPreheatClient{
		enqueuer:  client,
		queue:     cfg.Queue,
		leadTime:  cfg.LeadTime,
		maxRetry:  cfg.MaxRetry,
		uniqueTTL: cfg.UniqueTTL,
	}, nil
}

func (c *asynqRushInventoryPreheatClient) Enqueue(ctx context.Context, showTimeID int64, expectedOpenTime time.Time) error {
	body, err := preheatqueue.MarshalRushInventoryPreheatPayload(showTimeID, expectedOpenTime, c.leadTime)
	if err != nil {
		return err
	}

	processAt := expectedOpenTime.Add(-c.leadTime)
	opts := []asynq.Option{
		asynq.Queue(c.queue),
		asynq.ProcessAt(processAt),
		asynq.TaskID(preheatqueue.RushInventoryPreheatTaskID(showTimeID, expectedOpenTime)),
	}
	if c.maxRetry > 0 {
		opts = append(opts, asynq.MaxRetry(c.maxRetry))
	}
	if c.uniqueTTL > 0 {
		opts = append(opts, asynq.Unique(c.uniqueTTL))
	}

	_, err = c.enqueuer.EnqueueContext(ctx, asynq.NewTask(preheatqueue.TaskTypeRushInventoryPreheat, body), opts...)
	return err
}
