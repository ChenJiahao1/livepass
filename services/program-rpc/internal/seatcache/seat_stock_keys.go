package seatcache

import "fmt"

const (
	defaultSeatLedgerPrefix = "livepass:program:seat-ledger"
)

func stockKey(prefix string, showTimeID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:stock:%s:%d", prefix, seatLedgerScopeTag(showTimeID), ticketCategoryID)
}

func availableSeatsKey(prefix string, showTimeID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:available:%s:%d", prefix, seatLedgerScopeTag(showTimeID), ticketCategoryID)
}

func soldSeatsKey(prefix string, showTimeID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:sold:%s:%d", prefix, seatLedgerScopeTag(showTimeID), ticketCategoryID)
}

func frozenSeatsKey(prefix string, showTimeID, ticketCategoryID int64, freezeToken string) string {
	return fmt.Sprintf("%s:frozen:%s:%d:%s", prefix, seatLedgerScopeTag(showTimeID), ticketCategoryID, freezeToken)
}

func frozenSeatsPattern(prefix string, showTimeID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:frozen:%s:%d:*", prefix, seatLedgerScopeTag(showTimeID), ticketCategoryID)
}

func loadingKey(prefix string, showTimeID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:loading:%s:%d", prefix, seatLedgerScopeTag(showTimeID), ticketCategoryID)
}

func seatLedgerScopeTag(showTimeID int64) string {
	return fmt.Sprintf("{st:%d}", showTimeID)
}
