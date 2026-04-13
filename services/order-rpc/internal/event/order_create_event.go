package event

import (
	"encoding/json"
	"fmt"
)

const OrderCreateEventVersion = "v1"

type OrderCreateEvent struct {
	EventID                string                 `json:"eventId"`
	Version                string                 `json:"version"`
	OrderNumber            int64                  `json:"orderNumber"`
	RequestNo              string                 `json:"requestNo"`
	OccurredAt             string                 `json:"occurredAt"`
	UserID                 int64                  `json:"userId"`
	ProgramID              int64                  `json:"programId"`
	ShowTimeID             int64                  `json:"showTimeId"`
	TicketCategoryID       int64                  `json:"ticketCategoryId"`
	TicketUserIDs          []int64                `json:"ticketUserIds"`
	TicketCount            int64                  `json:"ticketCount"`
	DistributionMode       string                 `json:"distributionMode"`
	TakeTicketMode         string                 `json:"takeTicketMode"`
	SaleWindowEndAt        string                 `json:"saleWindowEndAt"`
	ShowEndAt              string                 `json:"showEndAt"`
	FreezeToken            string                 `json:"freezeToken"`
	FreezeExpireTime       string                 `json:"freezeExpireTime"`
	ProgramSnapshot        ProgramSnapshot        `json:"programSnapshot"`
	TicketCategorySnapshot TicketCategorySnapshot `json:"ticketCategorySnapshot"`
	TicketUserSnapshot     []TicketUserSnapshot   `json:"ticketUserSnapshot"`
	SeatSnapshot           []SeatSnapshot         `json:"seatSnapshot"`
}

type ProgramSnapshot struct {
	Title            string `json:"title"`
	ItemPicture      string `json:"itemPicture"`
	Place            string `json:"place"`
	ShowTime         string `json:"showTime"`
	PermitChooseSeat int64  `json:"permitChooseSeat"`
}

type TicketCategorySnapshot struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Price int64  `json:"price"`
}

type TicketUserSnapshot struct {
	TicketUserID int64  `json:"ticketUserId"`
	Name         string `json:"name"`
	IDNumber     string `json:"idNumber"`
}

type SeatSnapshot struct {
	SeatID  int64 `json:"seatId"`
	RowCode int64 `json:"rowCode"`
	ColCode int64 `json:"colCode"`
	Price   int64 `json:"price"`
}

func (e *OrderCreateEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *OrderCreateEvent) PartitionKey() string {
	if e == nil {
		return ""
	}

	return fmt.Sprintf("%d#%d", e.ShowTimeID, e.TicketCategoryID)
}

func UnmarshalOrderCreateEvent(body []byte) (*OrderCreateEvent, error) {
	var evt OrderCreateEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return nil, err
	}

	return &evt, nil
}
