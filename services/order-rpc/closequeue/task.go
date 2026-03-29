package closequeue

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	TaskTypeCloseTimeout = "order:close_timeout"
	timeLayout           = "2006-01-02 15:04:05"
)

type CloseTimeoutPayload struct {
	OrderNumber int64  `json:"orderNumber"`
	ExpireAt    string `json:"expireAt"`
}

func CloseTimeoutTaskID(orderNumber int64) string {
	return fmt.Sprintf("order-close:%d", orderNumber)
}

func MarshalCloseTimeoutPayload(orderNumber int64, expireAt time.Time) ([]byte, error) {
	return json.Marshal(CloseTimeoutPayload{
		OrderNumber: orderNumber,
		ExpireAt:    expireAt.Format(timeLayout),
	})
}

func ParseCloseTimeoutPayload(body []byte) (CloseTimeoutPayload, error) {
	var payload CloseTimeoutPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return CloseTimeoutPayload{}, err
	}
	return payload, nil
}
