package logic

import (
	"database/sql"
	"time"

	"livepass/jobs/order-close/taskdef"
	"livepass/pkg/delaytask"
	"livepass/pkg/xid"
	"livepass/services/order-rpc/internal/model"
)

func newCloseTimeoutDelayTaskRow(now time.Time, orderNumber int64, executeAt time.Time) (*model.DDelayTaskOutbox, error) {
	payload, err := taskdef.Marshal(orderNumber)
	if err != nil {
		return nil, err
	}

	return &model.DDelayTaskOutbox{
		Id:               xid.New(),
		TaskType:         taskdef.TaskTypeCloseTimeout,
		TaskKey:          taskdef.TaskKey(orderNumber),
		Payload:          string(payload),
		ExecuteAt:        executeAt,
		TaskStatus:       delaytask.OutboxTaskStatusPending,
		PublishAttempts:  0,
		ConsumeAttempts:  0,
		LastPublishError: "",
		LastConsumeError: "",
		PublishedTime:    sql.NullTime{},
		ProcessedTime:    sql.NullTime{},
		CreateTime:       now,
		EditTime:         now,
		Status:           1,
	}, nil
}
