package seatcache

import "fmt"

const (
	defaultSeatLedgerPrefix      = "damai-go:program:seat-ledger"
	seatStockAvailableCountField = "available_count"
)

func stockKey(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:stock:%s:%d", prefix, seatLedgerScopeTag(programID), ticketCategoryID)
}

func availableSeatsKey(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:available:%s:%d", prefix, seatLedgerScopeTag(programID), ticketCategoryID)
}

func soldSeatsKey(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:sold:%s:%d", prefix, seatLedgerScopeTag(programID), ticketCategoryID)
}

func frozenSeatsKey(prefix string, programID, ticketCategoryID int64, freezeToken string) string {
	return fmt.Sprintf("%s:frozen:%s:%d:%s", prefix, seatLedgerScopeTag(programID), ticketCategoryID, freezeToken)
}

func frozenSeatsPattern(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:frozen:%s:%d:*", prefix, seatLedgerScopeTag(programID), ticketCategoryID)
}

func loadingKey(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:loading:%s:%d", prefix, seatLedgerScopeTag(programID), ticketCategoryID)
}

func seatLedgerScopeTag(showTimeID int64) string {
	return fmt.Sprintf("{st:%d:g:%s}", showTimeID, seatLedgerGeneration(showTimeID))
}

func seatLedgerGeneration(showTimeID int64) string {
	return fmt.Sprintf("g-%d", showTimeID)
}
