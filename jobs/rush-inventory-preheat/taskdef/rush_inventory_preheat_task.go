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
	ProgramId                int64  `json:"programId"`
	ExpectedRushSaleOpenTime string `json:"expectedRushSaleOpenTime"`
	LeadTime                 string `json:"leadTime"`
}

func TaskKey(programID int64, expectedOpenTime time.Time) string {
	return fmt.Sprintf("%s:%d:%s", TaskTypeRushInventoryPreheat, programID, expectedOpenTime.Format(taskKeyTimeLayout))
}

func Marshal(programID int64, expectedOpenTime time.Time, leadTime time.Duration) ([]byte, error) {
	return json.Marshal(RushInventoryPreheatPayload{
		ProgramId:                programID,
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

func NewMessage(programID int64, expectedOpenTime time.Time, leadTime time.Duration) (delaytask.Message, error) {
	payload, err := Marshal(programID, expectedOpenTime, leadTime)
	if err != nil {
		return delaytask.Message{}, err
	}

	return delaytask.Message{
		Type:      TaskTypeRushInventoryPreheat,
		Key:       TaskKey(programID, expectedOpenTime),
		Payload:   payload,
		ExecuteAt: expectedOpenTime.Add(-leadTime),
	}, nil
}
