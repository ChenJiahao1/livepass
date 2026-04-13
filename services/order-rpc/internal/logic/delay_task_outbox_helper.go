package logic

import (
	"database/sql"
	"time"

	"damai-go/jobs/order-close/taskdef"
	"damai-go/pkg/xid"
	"damai-go/services/order-rpc/internal/model"
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
		PublishedStatus:  0,
		PublishAttempts:  0,
		LastPublishError: "",
		PublishedTime:    sql.NullTime{},
		CreateTime:       now,
		EditTime:         now,
		Status:           1,
	}, nil
}
