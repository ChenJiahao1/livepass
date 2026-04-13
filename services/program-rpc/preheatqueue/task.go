package preheatqueue

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	TaskTypeRushInventoryPreheat = "program:rush_inventory_preheat"
	taskTimeLayout               = "2006-01-02 15:04:05"
	taskIDTimeLayout             = "20060102150405"
)

type RushInventoryPreheatPayload struct {
	ShowTimeId               int64  `json:"showTimeId"`
	ExpectedRushSaleOpenTime string `json:"expectedRushSaleOpenTime"`
	LeadTime                 string `json:"leadTime"`
}

func RushInventoryPreheatTaskID(showTimeID int64, expectedOpenTime time.Time) string {
	return fmt.Sprintf("rush-inventory-preheat:%d:%s", showTimeID, expectedOpenTime.Format(taskIDTimeLayout))
}

func MarshalRushInventoryPreheatPayload(showTimeID int64, expectedOpenTime time.Time, leadTime time.Duration) ([]byte, error) {
	return json.Marshal(RushInventoryPreheatPayload{
		ShowTimeId:               showTimeID,
		ExpectedRushSaleOpenTime: expectedOpenTime.Format(taskTimeLayout),
		LeadTime:                 leadTime.String(),
	})
}

func ParseRushInventoryPreheatPayload(body []byte) (RushInventoryPreheatPayload, error) {
	var payload RushInventoryPreheatPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return RushInventoryPreheatPayload{}, err
	}
	return payload, nil
}
