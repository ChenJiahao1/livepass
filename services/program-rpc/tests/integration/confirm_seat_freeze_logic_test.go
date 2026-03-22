package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/program-rpc/internal/logic"
	"damai-go/services/program-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testSeatStatusSold        = 3
	testFreezeStatusConfirmed = 4
)

func TestAutoAssignAndFreezeSeatsPersistsOnlySelectedSeatsAfterRedisReservation(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	const programID int64 = 53101
	const ticketCategoryID int64 = 63101
	seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 78101, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 78102, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 78103, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 2, ColCode: 1, SeatStatus: testSeatStatusAvailable},
	)
	primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

	resp, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
		ProgramId:        programID,
		TicketCategoryId: ticketCategoryID,
		Count:            2,
		RequestNo:        "req-confirm-ledger-persist",
		FreezeSeconds:    900,
	})
	if err != nil {
		t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
	}
	if len(resp.Seats) != 2 {
		t.Fatalf("expected 2 selected seats, got %d", len(resp.Seats))
	}

	frozenSeats := querySeatRowsByFreezeToken(t, svcCtx, resp.FreezeToken)
	if len(frozenSeats) != 2 {
		t.Fatalf("expected exactly 2 persisted frozen seats, got %+v", frozenSeats)
	}
	if frozenSeats[0].ID != resp.Seats[0].SeatId || frozenSeats[1].ID != resp.Seats[1].SeatId {
		t.Fatalf("expected persisted seats to match redis reservation, seats=%+v db=%+v", resp.Seats, frozenSeats)
	}
	if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusFrozen) != 2 {
		t.Fatalf("expected only 2 seats frozen in db")
	}
	if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusAvailable) != 1 {
		t.Fatalf("expected one seat to remain available in db")
	}

	snapshot := requireProgramSeatLedgerSnapshot(t, svcCtx, programID, ticketCategoryID)
	if snapshot.AvailableCount != 1 {
		t.Fatalf("expected ledger available count to be 1, got %d", snapshot.AvailableCount)
	}
	if len(snapshot.FrozenSeats[resp.FreezeToken]) != 2 {
		t.Fatalf("expected ledger to keep 2 frozen seats, got %+v", snapshot.FrozenSeats[resp.FreezeToken])
	}
}

func TestConfirmSeatFreezeMovesSeatsToSoldLedger(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	const programID int64 = 53102
	const ticketCategoryID int64 = 63102
	const freezeToken = "freeze-confirm-ledger-sold"
	seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 78201, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusFrozen, FreezeToken: freezeToken, FreezeExpireTime: "2026-12-31 20:00:00"},
		seatFixture{ID: 78202, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusFrozen, FreezeToken: freezeToken, FreezeExpireTime: "2026-12-31 20:00:00"},
		seatFixture{ID: 78203, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 2, ColCode: 1, SeatStatus: testSeatStatusAvailable},
	)
	seedSeatFreezeFixture(t, svcCtx, seatFreezeFixture{
		ID:               84301,
		FreezeToken:      freezeToken,
		RequestNo:        "req-confirm-ledger-sold",
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		SeatCount:        2,
		FreezeStatus:     testFreezeStatusFrozen,
		ExpireTime:       "2026-12-31 20:00:00",
	})
	primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

	resp, err := logicpkg.NewConfirmSeatFreezeLogic(context.Background(), svcCtx).ConfirmSeatFreeze(&pb.ConfirmSeatFreezeReq{
		FreezeToken: freezeToken,
	})
	if err != nil {
		t.Fatalf("ConfirmSeatFreeze returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected confirm success")
	}
	if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusSold) != 2 {
		t.Fatalf("expected 2 sold seats in db")
	}
	if querySeatFreezeByToken(t, svcCtx, freezeToken).FreezeStatus != testFreezeStatusConfirmed {
		t.Fatalf("expected freeze row confirmed")
	}

	snapshot := requireProgramSeatLedgerSnapshot(t, svcCtx, programID, ticketCategoryID)
	if snapshot.AvailableCount != 1 {
		t.Fatalf("expected ledger available count to remain 1, got %d", snapshot.AvailableCount)
	}
	if len(snapshot.FrozenSeats[freezeToken]) != 0 {
		t.Fatalf("expected frozen ledger cleared after confirm, got %+v", snapshot.FrozenSeats[freezeToken])
	}
	if len(snapshot.SoldSeats) != 2 {
		t.Fatalf("expected 2 seats in sold ledger, got %+v", snapshot.SoldSeats)
	}
	if snapshot.SoldSeats[0].SeatID != 78201 || snapshot.SoldSeats[1].SeatID != 78202 {
		t.Fatalf("expected sold ledger seats [78201 78202], got %+v", snapshot.SoldSeats)
	}
}

func TestConfirmSeatFreeze(t *testing.T) {
	t.Run("confirm frozen seats to sold", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 53001
		const ticketCategoryID int64 = 63001
		const freezeToken = "freeze-confirm-success"
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 77001, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusFrozen, FreezeToken: freezeToken, FreezeExpireTime: "2026-12-31 20:00:00"},
			seatFixture{ID: 77002, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusFrozen, FreezeToken: freezeToken, FreezeExpireTime: "2026-12-31 20:00:00"},
		)
		seedSeatFreezeFixture(t, svcCtx, seatFreezeFixture{
			ID:               84001,
			FreezeToken:      freezeToken,
			RequestNo:        "req-confirm-success",
			ProgramID:        programID,
			TicketCategoryID: ticketCategoryID,
			SeatCount:        2,
			FreezeStatus:     testFreezeStatusFrozen,
			ExpireTime:       "2026-12-31 20:00:00",
		})

		l := logicpkg.NewConfirmSeatFreezeLogic(context.Background(), svcCtx)
		resp, err := l.ConfirmSeatFreeze(&pb.ConfirmSeatFreezeReq{FreezeToken: freezeToken})
		if err != nil {
			t.Fatalf("ConfirmSeatFreeze returned error: %v", err)
		}
		if !resp.Success {
			t.Fatalf("expected success response")
		}

		seats := querySeatRowsByFreezeToken(t, svcCtx, freezeToken)
		if len(seats) != 2 {
			t.Fatalf("expected 2 sold seats, got %d", len(seats))
		}
		if seats[0].SeatStatus != testSeatStatusSold || seats[1].SeatStatus != testSeatStatusSold {
			t.Fatalf("expected seats sold, got %+v", seats)
		}
		if querySeatFreezeByToken(t, svcCtx, freezeToken).FreezeStatus != testFreezeStatusConfirmed {
			t.Fatalf("expected freeze row confirmed")
		}
	})

	t.Run("expired freeze cannot be confirmed", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 53002
		const ticketCategoryID int64 = 63002
		const freezeToken = "freeze-confirm-expired"
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 77101, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusFrozen, FreezeToken: freezeToken, FreezeExpireTime: "2026-01-01 09:00:00"},
		)
		seedSeatFreezeFixture(t, svcCtx, seatFreezeFixture{
			ID:               84101,
			FreezeToken:      freezeToken,
			RequestNo:        "req-confirm-expired",
			ProgramID:        programID,
			TicketCategoryID: ticketCategoryID,
			SeatCount:        1,
			FreezeStatus:     testFreezeStatusFrozen,
			ExpireTime:       "2026-01-01 09:00:00",
		})

		l := logicpkg.NewConfirmSeatFreezeLogic(context.Background(), svcCtx)
		_, err := l.ConfirmSeatFreeze(&pb.ConfirmSeatFreezeReq{FreezeToken: freezeToken})
		if err == nil {
			t.Fatalf("expected failed precondition error")
		}
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("expected failed precondition, got %s", status.Code(err))
		}
	})

	t.Run("released freeze cannot be confirmed", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 53003
		const ticketCategoryID int64 = 63003
		const freezeToken = "freeze-confirm-released"
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFreezeFixture(t, svcCtx, seatFreezeFixture{
			ID:               84201,
			FreezeToken:      freezeToken,
			RequestNo:        "req-confirm-released",
			ProgramID:        programID,
			TicketCategoryID: ticketCategoryID,
			SeatCount:        1,
			FreezeStatus:     testFreezeStatusReleased,
			ExpireTime:       "2026-12-31 20:00:00",
			ReleaseReason:    "manual release",
			ReleaseTime:      "2026-01-01 10:00:00",
		})

		l := logicpkg.NewConfirmSeatFreezeLogic(context.Background(), svcCtx)
		_, err := l.ConfirmSeatFreeze(&pb.ConfirmSeatFreezeReq{FreezeToken: freezeToken})
		if err == nil {
			t.Fatalf("expected failed precondition error")
		}
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("expected failed precondition, got %s", status.Code(err))
		}
	})
}
