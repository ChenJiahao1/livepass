package logic

import (
	"time"
)

var rushContractNow = func() time.Time {
	return time.Now()
}

func allocateRushContractOrderNumber(userID int64) int64 {
	return defaultOrderNumberGenerator.Next(userID, rushContractNow())
}
