package seatcache

import (
	"context"

	"livepass/pkg/xerr"
)

func (s *SeatStockStore) FrozenSeats(ctx context.Context, showTimeID, ticketCategoryID int64, freezeToken string) ([]Seat, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrProgramSeatLedgerNotReady
	}

	return s.listSeats(ctx, frozenSeatsKey(s.prefix, showTimeID, ticketCategoryID, freezeToken))
}
