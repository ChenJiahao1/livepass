package taskdef

import (
	"encoding/json"
	"fmt"
	"time"

	"livepass/pkg/delaytask"
)

const TaskTypeCloseTimeout = "order.close_timeout"

type CloseTimeoutPayload struct {
	OrderNumber int64 `json:"orderNumber"`
}

func TaskKey(orderNumber int64) string {
	return fmt.Sprintf("%s:%d", TaskTypeCloseTimeout, orderNumber)
}

func Marshal(orderNumber int64) ([]byte, error) {
	return json.Marshal(CloseTimeoutPayload{
		OrderNumber: orderNumber,
	})
}

func Parse(body []byte) (CloseTimeoutPayload, error) {
	var payload CloseTimeoutPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return CloseTimeoutPayload{}, err
	}
	return payload, nil
}

func NewMessage(orderNumber int64, executeAt time.Time) (delaytask.Message, error) {
	payload, err := Marshal(orderNumber)
	if err != nil {
		return delaytask.Message{}, err
	}

	return delaytask.Message{
		Type:      TaskTypeCloseTimeout,
		Key:       TaskKey(orderNumber),
		Payload:   payload,
		ExecuteAt: executeAt,
	}, nil
}
