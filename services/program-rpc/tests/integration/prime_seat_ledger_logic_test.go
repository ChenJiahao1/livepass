package integration_test

import (
	"context"
	"testing"

	"damai-go/services/program-rpc/internal/server"
	"damai-go/services/program-rpc/pb"
)

func TestPrimeSeatLedgerRPCPrimesAllTicketCategoriesByShowTime(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	programID := int64(92001)
	showTimeID := int64(93001)
	seedProgramFixtures(t, svcCtx, programFixture{
		ProgramID:    programID,
		ShowTimeID:   showTimeID,
		ShowTime:     "2026-12-31 19:30:00",
		ShowDayTime:  "2026-12-31 00:00:00",
		ShowWeekTime: "周四",
		TicketCategories: []ticketCategoryFixture{
			{ID: 41001, ShowTimeID: showTimeID, Introduce: "A 档", Price: 299, TotalNumber: 2, RemainNumber: 2},
			{ID: 41002, ShowTimeID: showTimeID, Introduce: "B 档", Price: 399, TotalNumber: 1, RemainNumber: 1},
		},
	})
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 51001, ProgramID: programID, ShowTimeID: showTimeID, TicketCategoryID: 41001, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 51002, ProgramID: programID, ShowTimeID: showTimeID, TicketCategoryID: 41001, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 51003, ProgramID: programID, ShowTimeID: showTimeID, TicketCategoryID: 41002, RowCode: 2, ColCode: 1, SeatStatus: testSeatStatusAvailable},
	)
	clearProgramSeatLedger(t, svcCtx, programID, 41001)
	clearProgramSeatLedger(t, svcCtx, programID, 41002)

	resp, err := server.NewProgramRpcServer(svcCtx).PrimeSeatLedger(context.Background(), &pb.PrimeSeatLedgerReq{
		ShowTimeId: showTimeID,
	})
	if err != nil {
		t.Fatalf("PrimeSeatLedger RPC error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected PrimeSeatLedger RPC success, got %+v", resp)
	}

	snapshotA := requireProgramSeatLedgerSnapshot(t, svcCtx, programID, 41001)
	if !snapshotA.Ready || snapshotA.AvailableCount != 2 {
		t.Fatalf("expected ticket category 41001 seat ledger ready with 2 seats, got %+v", snapshotA)
	}
	snapshotB := requireProgramSeatLedgerSnapshot(t, svcCtx, programID, 41002)
	if !snapshotB.Ready || snapshotB.AvailableCount != 1 {
		t.Fatalf("expected ticket category 41002 seat ledger ready with 1 seat, got %+v", snapshotB)
	}
}
