package closequeue

import (
	"encoding/json"
	"fmt"
	"time"
)

const TaskTypeVerifyAttemptDue = "order:verify_attempt_due"

type VerifyAttemptPayload struct {
	OrderNumber int64  `json:"orderNumber"`
	DueAt       string `json:"dueAt"`
}

func VerifyAttemptTaskID(orderNumber int64) string {
	return fmt.Sprintf("order-verify-attempt:%d", orderNumber)
}

func MarshalVerifyAttemptPayload(orderNumber int64, dueAt time.Time) ([]byte, error) {
	return json.Marshal(VerifyAttemptPayload{
		OrderNumber: orderNumber,
		DueAt:       dueAt.Format(timeLayout),
	})
}

func ParseVerifyAttemptPayload(body []byte) (VerifyAttemptPayload, error) {
	var payload VerifyAttemptPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return VerifyAttemptPayload{}, err
	}

	return payload, nil
}
