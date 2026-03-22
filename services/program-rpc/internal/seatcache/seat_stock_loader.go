package seatcache

import (
	"context"
	"strconv"

	"damai-go/services/program-rpc/internal/model"
)

type seatLedgerSource interface {
	FindByProgramAndTicketCategoryAndSeatStatus(ctx context.Context, programID, ticketCategoryID, seatStatus int64) ([]*model.DSeat, error)
}

type seatStockLoader struct {
	redis                  redisClient
	seatModel              seatLedgerSource
	prefix                 string
	stockTTLSeconds        int
	seatTTLSeconds         int
	loadingCooldownSeconds int
}

func newSeatStockLoader(redis redisClient, seatModel seatLedgerSource, prefix string, stockTTLSeconds, seatTTLSeconds, loadingCooldownSeconds int) *seatStockLoader {
	if redis == nil || seatModel == nil {
		return nil
	}

	return &seatStockLoader{
		redis:                  redis,
		seatModel:              seatModel,
		prefix:                 prefix,
		stockTTLSeconds:        stockTTLSeconds,
		seatTTLSeconds:         seatTTLSeconds,
		loadingCooldownSeconds: loadingCooldownSeconds,
	}
}

func (l *seatStockLoader) Schedule(programID, ticketCategoryID int64) {
	if l == nil {
		return
	}

	ok, err := l.redis.SetnxEx(loadingKey(l.prefix, programID, ticketCategoryID), "1", l.loadingCooldownSeconds)
	if err != nil || !ok {
		return
	}

	go l.load(programID, ticketCategoryID)
}

func (l *seatStockLoader) LoadSync(ctx context.Context, programID, ticketCategoryID int64) error {
	if l == nil {
		return nil
	}

	return l.loadWithContext(ctx, programID, ticketCategoryID)
}

func (l *seatStockLoader) load(programID, ticketCategoryID int64) {
	ctx := context.Background()
	_ = l.loadWithContext(ctx, programID, ticketCategoryID)
	_, _ = l.redis.Del(loadingKey(l.prefix, programID, ticketCategoryID))
}

func (l *seatStockLoader) loadWithContext(ctx context.Context, programID, ticketCategoryID int64) error {
	availableSeats, err := l.seatModel.FindByProgramAndTicketCategoryAndSeatStatus(ctx, programID, ticketCategoryID, seatStatusAvailable)
	if err != nil {
		return err
	}
	frozenSeats, err := l.seatModel.FindByProgramAndTicketCategoryAndSeatStatus(ctx, programID, ticketCategoryID, seatStatusFrozen)
	if err != nil {
		return err
	}
	soldSeats, err := l.seatModel.FindByProgramAndTicketCategoryAndSeatStatus(ctx, programID, ticketCategoryID, seatStatusSold)
	if err != nil {
		return err
	}

	frozenKeys, err := l.redis.KeysCtx(ctx, frozenSeatsPattern(l.prefix, programID, ticketCategoryID))
	if err != nil {
		return err
	}

	keysToDelete := []string{
		stockKey(l.prefix, programID, ticketCategoryID),
		availableSeatsKey(l.prefix, programID, ticketCategoryID),
		soldSeatsKey(l.prefix, programID, ticketCategoryID),
		loadingKey(l.prefix, programID, ticketCategoryID),
	}
	keysToDelete = append(keysToDelete, frozenKeys...)
	if len(keysToDelete) > 0 {
		if _, err := l.redis.DelCtx(ctx, keysToDelete...); err != nil {
			return err
		}
	}

	stockRedisKey := stockKey(l.prefix, programID, ticketCategoryID)
	if err := l.redis.HsetCtx(ctx, stockRedisKey, seatStockAvailableCountField, strconv.FormatInt(int64(len(availableSeats)), 10)); err != nil {
		return err
	}
	if l.stockTTLSeconds > 0 {
		if err := l.redis.ExpireCtx(ctx, stockRedisKey, l.stockTTLSeconds); err != nil {
			return err
		}
	}

	for _, seat := range availableSeats {
		if err := l.addSeat(ctx, availableSeatsKey(l.prefix, programID, ticketCategoryID), newSeatFromModel(seat)); err != nil {
			return err
		}
	}
	for _, seat := range soldSeats {
		if err := l.addSeat(ctx, soldSeatsKey(l.prefix, programID, ticketCategoryID), newSeatFromModel(seat)); err != nil {
			return err
		}
	}
	frozenSeatGroups := make(map[string][]*model.DSeat)
	for _, seat := range frozenSeats {
		if !seat.FreezeToken.Valid || seat.FreezeToken.String == "" {
			continue
		}
		frozenSeatGroups[seat.FreezeToken.String] = append(frozenSeatGroups[seat.FreezeToken.String], seat)
	}
	for freezeToken, seats := range frozenSeatGroups {
		redisKey := frozenSeatsKey(l.prefix, programID, ticketCategoryID, freezeToken)
		for _, seat := range seats {
			if err := l.addSeat(ctx, redisKey, newSeatFromModel(seat)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (l *seatStockLoader) addSeat(ctx context.Context, redisKey string, seat Seat) error {
	if _, err := l.redis.ZaddCtx(ctx, redisKey, seatSortScore(seat.RowCode, seat.ColCode), encodeSeatMember(seat)); err != nil {
		return err
	}
	if l.seatTTLSeconds > 0 {
		return l.redis.ExpireCtx(ctx, redisKey, l.seatTTLSeconds)
	}

	return nil
}
