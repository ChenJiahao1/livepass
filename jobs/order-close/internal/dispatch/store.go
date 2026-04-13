package dispatch

import (
	"context"
	"time"
)

type TaskRef struct {
	DBKey string
	ID    int64
}

type PendingTask struct {
	Ref       TaskRef
	TaskType  string
	TaskKey   string
	Payload   string
	ExecuteAt time.Time
}

type Store interface {
	ListPendingByTaskType(ctx context.Context, taskType string, limit int64) ([]PendingTask, error)
	MarkPublished(ctx context.Context, ref TaskRef, publishedAt time.Time) error
	MarkPublishFailed(ctx context.Context, ref TaskRef, failedAt time.Time, publishErr string) error
}
