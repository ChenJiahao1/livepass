package seatcache

import "fmt"

const (
	defaultSeatLedgerPrefix      = "damai-go:program:seat-ledger"
	seatStockAvailableCountField = "available_count"
)

func stockKey(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:stock:%d:%d", prefix, programID, ticketCategoryID)
}

func availableSeatsKey(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:available:%d:%d", prefix, programID, ticketCategoryID)
}

func soldSeatsKey(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:sold:%d:%d", prefix, programID, ticketCategoryID)
}

func frozenSeatsKey(prefix string, programID, ticketCategoryID int64, freezeToken string) string {
	return fmt.Sprintf("%s:frozen:%d:%d:%s", prefix, programID, ticketCategoryID, freezeToken)
}

func frozenSeatsPattern(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:frozen:%d:%d:*", prefix, programID, ticketCategoryID)
}

func loadingKey(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:loading:%d:%d", prefix, programID, ticketCategoryID)
}
