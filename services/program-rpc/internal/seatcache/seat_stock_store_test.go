package seatcache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"livepass/pkg/xredis"
	"livepass/services/program-rpc/internal/model"
)

type fakeSeatLedgerSource struct {
	seatsByStatus map[int64][]*model.DSeat
}

func (f *fakeSeatLedgerSource) FindByShowTimeAndTicketCategoryAndSeatStatus(_ context.Context, _, _, seatStatus int64) ([]*model.DSeat, error) {
	return f.seatsByStatus[seatStatus], nil
}

func TestPrimeFromDBStoresStockAsString(t *testing.T) {
	redisClient, err := xredis.New(xredis.Config{
		Host: "127.0.0.1:6379",
		Type: "node",
	})
	if err != nil {
		t.Fatalf("new redis client error: %v", err)
	}

	const (
		showTimeID       int64 = 30001
		ticketCategoryID int64 = 40001
	)
	prefix := fmt.Sprintf("livepass:test:program:seat-ledger:%s:%d", t.Name(), time.Now().UnixNano())
	store := NewSeatStockStore(redisClient, &fakeSeatLedgerSource{
		seatsByStatus: map[int64][]*model.DSeat{
			seatStatusAvailable: {
				{Id: 1, ShowTimeId: showTimeID, TicketCategoryId: ticketCategoryID, RowCode: 1, ColCode: 1, Price: 100},
				{Id: 2, ShowTimeId: showTimeID, TicketCategoryId: ticketCategoryID, RowCode: 1, ColCode: 2, Price: 100},
				{Id: 3, ShowTimeId: showTimeID, TicketCategoryId: ticketCategoryID, RowCode: 2, ColCode: 1, Price: 100},
			},
			seatStatusFrozen: nil,
			seatStatusSold:   nil,
		},
	}, Config{
		Prefix:          prefix,
		StockTTL:        time.Hour,
		SeatTTL:         time.Hour,
		LoadingCooldown: 200 * time.Millisecond,
	})
	if store == nil {
		t.Fatalf("expected seat stock store")
	}

	t.Cleanup(func() {
		_, _ = redisClient.DelCtx(context.Background(),
			stockKey(prefix, showTimeID, ticketCategoryID),
			availableSeatsKey(prefix, showTimeID, ticketCategoryID),
			soldSeatsKey(prefix, showTimeID, ticketCategoryID),
			loadingKey(prefix, showTimeID, ticketCategoryID),
		)
	})

	if err := store.PrimeFromDB(context.Background(), showTimeID, ticketCategoryID); err != nil {
		t.Fatalf("PrimeFromDB() error = %v", err)
	}

	value, err := redisClient.GetCtx(context.Background(), stockKey(prefix, showTimeID, ticketCategoryID))
	if err != nil {
		t.Fatalf("expected stock key to be a string, got error: %v", err)
	}
	if value != "3" {
		t.Fatalf("expected stock value 3, got %q", value)
	}
}
