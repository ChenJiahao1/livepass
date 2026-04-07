package logic

import (
	"database/sql"
	"encoding/json"
	"time"

	"damai-go/pkg/xid"
	"damai-go/services/order-rpc/internal/model"
)

func buildOrderOutboxPayload(orderNumber, programID, showTimeID, userID int64) (string, error) {
	payload, err := json.Marshal(map[string]int64{
		"orderNumber": orderNumber,
		"programId":   programID,
		"showTimeId":  showTimeID,
		"userId":      userID,
	})
	if err != nil {
		return "", err
	}

	return string(payload), nil
}

func newOrderOutboxRow(now time.Time, orderNumber, programID, showTimeID, userID int64, eventType string) (*model.DOrderOutbox, error) {
	payload, err := buildOrderOutboxPayload(orderNumber, programID, showTimeID, userID)
	if err != nil {
		return nil, err
	}

	return &model.DOrderOutbox{
		Id:              xid.New(),
		OrderNumber:     orderNumber,
		ShowTimeId:      showTimeID,
		EventType:       eventType,
		Payload:         payload,
		PublishedStatus: 0,
		PublishedTime:   sql.NullTime{},
		CreateTime:      now,
		EditTime:        now,
		Status:          1,
	}, nil
}
