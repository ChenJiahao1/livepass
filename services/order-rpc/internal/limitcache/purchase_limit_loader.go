package limitcache

import (
	"context"
	"strconv"

	"damai-go/pkg/xredis"
)

type activeTicketCounter interface {
	CountActiveTicketsByUserShowTime(ctx context.Context, userID, showTimeID int64) (int64, error)
	ListUnpaidReservationsByUserShowTime(ctx context.Context, userID, showTimeID int64) (map[int64]int64, error)
}

type purchaseLimitLoader struct {
	redis                  *xredis.Client
	orderModel             activeTicketCounter
	prefix                 string
	ledgerTTLSeconds       int
	loadingCooldownSeconds int
}

func newPurchaseLimitLoader(redis *xredis.Client, orderModel activeTicketCounter, prefix string, ledgerTTLSeconds, loadingCooldownSeconds int) *purchaseLimitLoader {
	if redis == nil || orderModel == nil {
		return nil
	}

	return &purchaseLimitLoader{
		redis:                  redis,
		orderModel:             orderModel,
		prefix:                 prefix,
		ledgerTTLSeconds:       ledgerTTLSeconds,
		loadingCooldownSeconds: loadingCooldownSeconds,
	}
}

func (l *purchaseLimitLoader) Schedule(userID, programID int64) {
	if l == nil {
		return
	}

	ok, err := l.redis.SetnxEx(loadingKey(l.prefix, userID, programID), "1", l.loadingCooldownSeconds)
	if err != nil || !ok {
		return
	}

	go l.load(userID, programID)
}

func (l *purchaseLimitLoader) load(userID, programID int64) {
	ctx := context.Background()
	activeCount, err := l.orderModel.CountActiveTicketsByUserShowTime(ctx, userID, programID)
	if err != nil {
		_, _ = l.redis.Del(loadingKey(l.prefix, userID, programID))
		return
	}
	reservations, err := l.orderModel.ListUnpaidReservationsByUserShowTime(ctx, userID, programID)
	if err != nil {
		_, _ = l.redis.Del(loadingKey(l.prefix, userID, programID))
		return
	}

	ledgerRedisKey := ledgerKey(l.prefix, userID, programID)
	exists, err := l.redis.Exists(ledgerRedisKey)
	if err != nil {
		_, _ = l.redis.Del(loadingKey(l.prefix, userID, programID))
		return
	}
	if exists {
		_, _ = l.redis.Del(loadingKey(l.prefix, userID, programID))
		return
	}

	fields := map[string]string{
		purchaseLimitCountField: strconv.FormatInt(activeCount, 10),
	}
	for orderNumber, ticketCount := range reservations {
		fields[reservationField(orderNumber)] = strconv.FormatInt(ticketCount, 10)
	}
	if err := l.redis.Hmset(ledgerRedisKey, fields); err != nil {
		_, _ = l.redis.Del(loadingKey(l.prefix, userID, programID))
		return
	}
	if l.ledgerTTLSeconds > 0 {
		_ = l.redis.Expire(ledgerRedisKey, l.ledgerTTLSeconds)
	}
	_, _ = l.redis.Del(loadingKey(l.prefix, userID, programID))
}
