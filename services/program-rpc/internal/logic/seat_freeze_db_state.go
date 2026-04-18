package logic

import (
	"context"

	"livepass/pkg/seatfreeze"
	"livepass/pkg/xerr"
	"livepass/services/program-rpc/internal/model"
)

type seatFreezeDBState struct {
	token    seatfreeze.Token
	dbSeats  []*model.DSeat
	allFrozen bool
	allSold   bool
}

func loadSeatFreezeDBState(ctx context.Context, seatModel model.DSeatModel, freezeToken string) (*seatFreezeDBState, error) {
	if seatModel == nil {
		return nil, xerr.ErrSeatFreezeStatusInvalid
	}

	token, err := seatfreeze.ParseToken(freezeToken)
	if err != nil {
		return nil, xerr.ErrInvalidParam
	}

	dbSeats, err := seatModel.FindByFreezeToken(ctx, freezeToken)
	if err != nil {
		return nil, err
	}

	state := &seatFreezeDBState{
		token:     token,
		dbSeats:   dbSeats,
		allFrozen: len(dbSeats) > 0,
		allSold:   len(dbSeats) > 0,
	}
	for _, seat := range dbSeats {
		if seat == nil || seat.ShowTimeId != token.ShowTimeID || seat.TicketCategoryId != token.TicketCategoryID {
			return nil, xerr.ErrSeatFreezeStatusInvalid
		}
		if seat.SeatStatus != 2 {
			state.allFrozen = false
		}
		if seat.SeatStatus != 3 {
			state.allSold = false
		}
	}

	return state, nil
}
