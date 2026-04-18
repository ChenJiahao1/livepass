package seatfreeze

import (
	"fmt"
)

type Token struct {
	ShowTimeID       int64
	TicketCategoryID int64
	OrderNumber      int64
	ProcessingEpoch  int64
}

func FormatToken(showTimeID, ticketCategoryID, orderNumber, processingEpoch int64) string {
	return fmt.Sprintf(
		"freeze-st%d-tc%d-o%d-e%d",
		showTimeID,
		ticketCategoryID,
		orderNumber,
		processingEpoch,
	)
}

func ParseToken(value string) (Token, error) {
	var token Token
	if _, err := fmt.Sscanf(value, "freeze-st%d-tc%d-o%d-e%d", &token.ShowTimeID, &token.TicketCategoryID, &token.OrderNumber, &token.ProcessingEpoch); err != nil {
		return Token{}, fmt.Errorf("invalid freeze token: %s", value)
	}
	if token.ShowTimeID <= 0 || token.TicketCategoryID <= 0 || token.OrderNumber <= 0 || token.ProcessingEpoch <= 0 {
		return Token{}, fmt.Errorf("invalid freeze token: %s", value)
	}

	return token, nil
}
