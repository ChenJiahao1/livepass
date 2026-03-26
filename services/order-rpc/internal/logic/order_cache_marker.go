package logic

import (
	"context"
	"strconv"
	"time"

	"damai-go/pkg/xredis"
)

const orderCreateMarkerTTL = time.Minute

func orderCreateMarkerKey(orderNumber int64) string {
	return "order:create:marker:" + strconv.FormatInt(orderNumber, 10)
}

func SetOrderCreateMarker(ctx context.Context, redis *xredis.Client, orderNumber int64) error {
	if redis == nil || orderNumber <= 0 {
		return nil
	}

	return redis.SetexCtx(
		ctx,
		orderCreateMarkerKey(orderNumber),
		strconv.FormatInt(orderNumber, 10),
		int(orderCreateMarkerTTL/time.Second),
	)
}

func GetOrderCreateMarker(ctx context.Context, redis *xredis.Client, orderNumber int64) (string, error) {
	if redis == nil || orderNumber <= 0 {
		return "", nil
	}

	return redis.GetCtx(ctx, orderCreateMarkerKey(orderNumber))
}
