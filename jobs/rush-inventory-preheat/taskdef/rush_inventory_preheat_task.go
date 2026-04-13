package taskdef

import (
	"encoding/json"
	"fmt"
	"time"

	"damai-go/pkg/delaytask"
)

const (
	TaskTypeRushInventoryPreheat = "program.rush_inventory_preheat"
	taskTimeLayout               = "2006-01-02 15:04:05"
	taskKeyTimeLayout            = "20060102150405"
)

type RushInventoryPreheatPayload struct {
	ShowTimeId               int64  `json:"showTimeId"`
	ExpectedRushSaleOpenTime string `json:"expectedRushSaleOpenTime"`
	LeadTime                 string `json:"leadTime"`
}

func TaskKey(showTimeID int64, expectedOpenTime time.Time) string {
	return fmt.Sprintf("%s:%d:%s", TaskTypeRushInventoryPreheat, showTimeID, expectedOpenTime.Format(taskKeyTimeLayout))
}

func Marshal(showTimeID int64, expectedOpenTime time.Time, leadTime time.Duration) ([]byte, error) {
	return json.Marshal(RushInventoryPreheatPayload{
		ShowTimeId:               showTimeID,
		ExpectedRushSaleOpenTime: expectedOpenTime.Format(taskTimeLayout),
		LeadTime:                 leadTime.String(),
	})
}

func Parse(body []byte) (RushInventoryPreheatPayload, error) {
	var payload RushInventoryPreheatPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return RushInventoryPreheatPayload{}, err
	}
	return payload, nil
}

func NewMessage(showTimeID int64, expectedOpenTime time.Time, leadTime time.Duration) (delaytask.Message, error) {
	payload, err := Marshal(showTimeID, expectedOpenTime, leadTime)
	if err != nil {
		return delaytask.Message{}, err
	}

	return delaytask.Message{
		Type:      TaskTypeRushInventoryPreheat,
		Key:       TaskKey(showTimeID, expectedOpenTime),
		Payload:   payload,
		ExecuteAt: expectedOpenTime.Add(-leadTime),
	}, nil
}
