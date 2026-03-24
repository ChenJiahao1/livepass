package repeatguard

import (
	"context"
	"errors"
	"fmt"
)

var ErrLocked = errors.New("repeat guard locked")

type UnlockFunc func()

type Guard interface {
	Lock(ctx context.Context, key string) (UnlockFunc, error)
}

func OrderCreateKey(userID, programID int64) string {
	return fmt.Sprintf("create_order:%d:%d", userID, programID)
}

func OrderStatusKey(orderNumber int64) string {
	return fmt.Sprintf("order_status:%d", orderNumber)
}
