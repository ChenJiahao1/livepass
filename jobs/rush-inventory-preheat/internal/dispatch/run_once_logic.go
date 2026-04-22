package dispatch

import (
	"context"
	"fmt"
	"time"

	"livepass/jobs/rush-inventory-preheat/internal/outbox"
	"livepass/pkg/delaytask"

	"github.com/zeromicro/go-zero/core/logx"
)

type RunOnceLogic struct {
	ctx       context.Context
	store     outbox.Store
	publisher delaytask.Publisher
	batchSize int64
	logx.Logger
}

func NewRunOnceLogic(ctx context.Context, store outbox.Store, publisher delaytask.Publisher, batchSize int64) *RunOnceLogic {
	return &RunOnceLogic{
		ctx:       ctx,
		store:     store,
		publisher: publisher,
		batchSize: batchSize,
		Logger:    logx.WithContext(ctx),
	}
}

func (l *RunOnceLogic) Run(taskType string) error {
	if l.store == nil {
		return fmt.Errorf("delay task store is not configured")
	}
	if l.publisher == nil {
		return fmt.Errorf("delay task publisher is not configured")
	}

	records, err := l.store.ListDispatchableByTaskType(l.ctx, taskType, l.batchSize)
	if err != nil {
		return err
	}

	publishedCount := 0
	failedCount := 0
	for _, record := range records {
		publishErr := l.publisher.Publish(l.ctx, delaytask.Message{
			Type:      record.TaskType,
			Key:       record.TaskKey,
			Payload:   []byte(record.Payload),
			ExecuteAt: record.ExecuteAt,
		})
		now := time.Now()
		isRepublish := delaytask.ShouldRepublish(record.TaskStatus)
		nextAttempts := record.PublishAttempts + 1

		if delaytask.IsDuplicateEnqueueError(publishErr) || publishErr == nil {
			if err := l.store.MarkPublished(l.ctx, record.Ref, now); err != nil {
				return err
			}
			l.Infow("delay_task_publish_state_transition",
				logx.Field("task_type", record.TaskType),
				logx.Field("task_key", record.TaskKey),
				logx.Field("from_status", record.TaskStatus),
				logx.Field("to_status", delaytask.OutboxTaskStatusPublished),
				logx.Field("publish_attempts", nextAttempts),
				logx.Field("is_republish", isRepublish),
			)
			publishedCount++
			continue
		}

		if err := l.store.MarkPublishFailed(l.ctx, record.Ref, now, publishErr.Error()); err != nil {
			return err
		}
		l.Errorw("delay_task_publish_state_transition_failed",
			logx.Field("task_type", record.TaskType),
			logx.Field("task_key", record.TaskKey),
			logx.Field("from_status", record.TaskStatus),
			logx.Field("to_status", delaytask.OutboxTaskStatusFailed),
			logx.Field("publish_attempts", nextAttempts),
			logx.Field("is_republish", isRepublish),
			logx.Field("error", publishErr.Error()),
		)
		failedCount++
	}

	l.Infof("rush-inventory-preheat dispatcher run finished, scanned=%d published=%d failed=%d", len(records), publishedCount, failedCount)
	return nil
}
