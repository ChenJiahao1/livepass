package integration_test

import (
	"context"
	"testing"

	logicpkg "livepass/services/program-rpc/internal/logic"
	"livepass/services/program-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testSeatStatusSold = 3

func TestAutoAssignAndFreezeSeatsPersistsFrozenSeatsBeforePaymentConfirm(t *testing.T) {
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
		ShowTimeId:       programID,
		TicketCategoryId: ticketCategoryID,
		Count:            2,
		FreezeToken:      "freeze-st53101-tc63101-o93101-e1",
		FreezeSeconds:    900,
	})
	if err != nil {
		t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
	}
	if len(resp.Seats) != 2 {
		t.Fatalf("expected 2 selected seats, got %d", len(resp.Seats))
	}

	if countSeatRowsByFreezeToken(t, svcCtx, resp.FreezeToken) != 2 {
		t.Fatalf("expected freeze stage to write freeze token into 2 db seats")
	}
	if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusFrozen) != 2 {
		t.Fatalf("expected 2 frozen seats in db before payment confirm")
	}
	if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusAvailable) != 1 {
		t.Fatalf("expected 1 available seat left in db before payment confirm")
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
	seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 78201, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 78202, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 78203, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 2, ColCode: 1, SeatStatus: testSeatStatusAvailable},
	)
	primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)
	freezeResp, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
		ShowTimeId:       programID,
		TicketCategoryId: ticketCategoryID,
		Count:            2,
		FreezeToken:      "freeze-st53102-tc63102-o93102-e1",
		FreezeSeconds:    900,
	})
	if err != nil {
		t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
	}
	if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusFrozen) != 2 {
		t.Fatalf("expected seats to enter frozen(2) before confirm")
	}

	resp, err := logicpkg.NewConfirmSeatFreezeLogic(context.Background(), svcCtx).ConfirmSeatFreeze(&pb.ConfirmSeatFreezeReq{
		FreezeToken: freezeResp.FreezeToken,
	})
	if err != nil {
		t.Fatalf("ConfirmSeatFreeze returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected confirm success")
	}
	if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusFrozen) != 0 {
		t.Fatalf("expected frozen seats to be cleared after confirm")
	}
	if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusSold) != 2 {
		t.Fatalf("expected 2 sold seats in db")
	}
	snapshot := requireProgramSeatLedgerSnapshot(t, svcCtx, programID, ticketCategoryID)
	if snapshot.AvailableCount != 1 {
		t.Fatalf("expected ledger available count to remain 1, got %d", snapshot.AvailableCount)
	}
	if len(snapshot.FrozenSeats[freezeResp.FreezeToken]) != 0 {
		t.Fatalf("expected frozen ledger cleared after confirm, got %+v", snapshot.FrozenSeats[freezeResp.FreezeToken])
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
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 77001, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
			seatFixture{ID: 77002, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
		)
		primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)
		freezeResp, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            2,
			FreezeToken:      "freeze-st53001-tc63001-o93001-e1",
			FreezeSeconds:    900,
		})
		if err != nil {
			t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
		}

		l := logicpkg.NewConfirmSeatFreezeLogic(context.Background(), svcCtx)
		resp, err := l.ConfirmSeatFreeze(&pb.ConfirmSeatFreezeReq{FreezeToken: freezeResp.FreezeToken})
		if err != nil {
			t.Fatalf("ConfirmSeatFreeze returned error: %v", err)
		}
		if !resp.Success {
			t.Fatalf("expected success response")
		}

		seats := querySeatRowsByFreezeToken(t, svcCtx, freezeResp.FreezeToken)
		if len(seats) != 2 {
			t.Fatalf("expected 2 sold seats, got %d", len(seats))
		}
		if seats[0].SeatStatus != testSeatStatusSold || seats[1].SeatStatus != testSeatStatusSold {
			t.Fatalf("expected seats sold, got %+v", seats)
		}
	})

	t.Run("released freeze cannot be confirmed", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 53003
		const ticketCategoryID int64 = 63003
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 77201, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		)
		primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)
		freezeResp, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            1,
			FreezeToken:      "freeze-st53003-tc63003-o93003-e1",
			FreezeSeconds:    900,
		})
		if err != nil {
			t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
		}
		if _, err := logicpkg.NewReleaseSeatFreezeLogic(context.Background(), svcCtx).ReleaseSeatFreeze(&pb.ReleaseSeatFreezeReq{
			FreezeToken:   freezeResp.FreezeToken,
			ReleaseReason: "manual release",
		}); err != nil {
			t.Fatalf("ReleaseSeatFreeze returned error: %v", err)
		}

		l := logicpkg.NewConfirmSeatFreezeLogic(context.Background(), svcCtx)
		_, err = l.ConfirmSeatFreeze(&pb.ConfirmSeatFreezeReq{FreezeToken: freezeResp.FreezeToken})
		if err == nil {
			t.Fatalf("expected failed precondition error")
		}
		if status.Code(err) != codes.NotFound {
			t.Fatalf("expected not found, got %s", status.Code(err))
		}
	})

	t.Run("confirmed freeze is idempotent", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 53004
		const ticketCategoryID int64 = 63004
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 77301, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
			seatFixture{ID: 77302, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
		)
		primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)
		freezeResp, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            2,
			FreezeToken:      "freeze-st53004-tc63004-o93004-e1",
			FreezeSeconds:    900,
		})
		if err != nil {
			t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
		}

		logic := logicpkg.NewConfirmSeatFreezeLogic(context.Background(), svcCtx)
		if _, err := logic.ConfirmSeatFreeze(&pb.ConfirmSeatFreezeReq{FreezeToken: freezeResp.FreezeToken}); err != nil {
			t.Fatalf("first ConfirmSeatFreeze returned error: %v", err)
		}

		resp, err := logic.ConfirmSeatFreeze(&pb.ConfirmSeatFreezeReq{FreezeToken: freezeResp.FreezeToken})
		if err != nil {
			t.Fatalf("expected idempotent confirm success, got error: %v", err)
		}
		if !resp.Success {
			t.Fatalf("expected success response on repeated confirm")
		}
		if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusSold) != 2 {
			t.Fatalf("expected sold seats to remain unchanged after repeated confirm")
		}
	})
}
