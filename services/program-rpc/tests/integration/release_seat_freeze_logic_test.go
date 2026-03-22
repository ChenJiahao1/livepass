package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/program-rpc/internal/logic"
	"damai-go/services/program-rpc/pb"
)

func TestReleaseSeatFreezeRestoresStockAndSeats(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	const programID int64 = 52101
	const ticketCategoryID int64 = 62101
	seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 76101, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1},
		seatFixture{ID: 76102, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2},
		seatFixture{ID: 76103, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 2, ColCode: 1},
	)
	primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

	autoResp, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
		ProgramId:        programID,
		TicketCategoryId: ticketCategoryID,
		Count:            2,
		RequestNo:        "req-seat-ledger-release",
		FreezeSeconds:    900,
	})
	if err != nil {
		t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
	}

	frozenSnapshot := requireProgramSeatLedgerSnapshot(t, svcCtx, programID, ticketCategoryID)
	if frozenSnapshot.AvailableCount != 1 {
		t.Fatalf("expected seat ledger available count to be 1 after freeze, got %d", frozenSnapshot.AvailableCount)
	}
	if len(frozenSnapshot.FrozenSeats[autoResp.FreezeToken]) != 2 {
		t.Fatalf("expected seat ledger to record 2 frozen seats, got %+v", frozenSnapshot.FrozenSeats)
	}

	releaseResp, err := logicpkg.NewReleaseSeatFreezeLogic(context.Background(), svcCtx).ReleaseSeatFreeze(&pb.ReleaseSeatFreezeReq{
		FreezeToken:   autoResp.FreezeToken,
		ReleaseReason: "order canceled",
	})
	if err != nil {
		t.Fatalf("ReleaseSeatFreeze returned error: %v", err)
	}
	if !releaseResp.Success {
		t.Fatalf("expected release success")
	}

	if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusAvailable) != 3 {
		t.Fatalf("expected all seats available after release")
	}
	if countSeatRowsByFreezeToken(t, svcCtx, autoResp.FreezeToken) != 0 {
		t.Fatalf("expected all seats released for freeze token %q", autoResp.FreezeToken)
	}

	releasedSnapshot := requireProgramSeatLedgerSnapshot(t, svcCtx, programID, ticketCategoryID)
	if releasedSnapshot.AvailableCount != 3 {
		t.Fatalf("expected seat ledger available count to be 3 after release, got %d", releasedSnapshot.AvailableCount)
	}
	if len(releasedSnapshot.FrozenSeats[autoResp.FreezeToken]) != 0 {
		t.Fatalf("expected seat ledger frozen seats to be cleared, got %+v", releasedSnapshot.FrozenSeats[autoResp.FreezeToken])
	}
}
