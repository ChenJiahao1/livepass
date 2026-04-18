package seatfreeze

import (
	"fmt"
)

type Token struct {
	ShowTimeID       int64
	TicketCategoryID int64
	OrderNumber      int64
}

func FormatToken(showTimeID, ticketCategoryID, orderNumber int64) string {
	return fmt.Sprintf(
		"freeze-st%d-tc%d-o%d",
		showTimeID,
		ticketCategoryID,
		orderNumber,
	)
}

func ParseToken(value string) (Token, error) {
	var token Token
	if _, err := fmt.Sscanf(value, "freeze-st%d-tc%d-o%d", &token.ShowTimeID, &token.TicketCategoryID, &token.OrderNumber); err != nil {
		return Token{}, fmt.Errorf("invalid freeze token: %s", value)
	}
	if token.ShowTimeID <= 0 || token.TicketCategoryID <= 0 || token.OrderNumber <= 0 || FormatToken(token.ShowTimeID, token.TicketCategoryID, token.OrderNumber) != value {
		return Token{}, fmt.Errorf("invalid freeze token: %s", value)
	}

	return token, nil
}
