package limitcache

import "fmt"

const (
	defaultPurchaseLimitPrefix    = "damai-go:order:purchase-limit"
	purchaseLimitCountField       = "active_count"
	purchaseLimitReservationField = "reservation:"
)

func ledgerKey(prefix string, userID, programID int64) string {
	return fmt.Sprintf("%s:ledger:%d:%d", prefix, userID, programID)
}

func loadingKey(prefix string, userID, programID int64) string {
	return fmt.Sprintf("%s:loading:%d:%d", prefix, userID, programID)
}

func reservationField(orderNumber int64) string {
	return fmt.Sprintf("%s%d", purchaseLimitReservationField, orderNumber)
}
