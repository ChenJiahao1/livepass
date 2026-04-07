package integration_test

import (
	"context"
	"testing"

	"damai-go/pkg/xerr"
	logicpkg "damai-go/services/program-rpc/internal/logic"
	"damai-go/services/program-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAutoAssignAndFreezeSeatsPrefersAdjacentSeats(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	const programID int64 = 51101
	const ticketCategoryID int64 = 61101
	seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 71101, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 71102, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 71103, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 2, ColCode: 1, SeatStatus: testSeatStatusAvailable},
	)
	primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

	resp, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
		ShowTimeId:       programID,
		TicketCategoryId: ticketCategoryID,
		Count:            2,
		RequestNo:        "req-seat-ledger-adjacent",
		FreezeSeconds:    900,
	})
	if err != nil {
		t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
	}
	if len(resp.Seats) != 2 {
		t.Fatalf("expected 2 seats, got %d", len(resp.Seats))
	}
	if resp.Seats[0].SeatId != 71101 || resp.Seats[1].SeatId != 71102 {
		t.Fatalf("expected adjacent seats [71101 71102], got %+v", resp.Seats)
	}

	snapshot := requireProgramSeatLedgerSnapshot(t, svcCtx, programID, ticketCategoryID)
	if snapshot.AvailableCount != 1 {
		t.Fatalf("expected seat ledger available count to be 1, got %d", snapshot.AvailableCount)
	}
	frozenSeats := snapshot.FrozenSeats[resp.FreezeToken]
	if len(frozenSeats) != 2 {
		t.Fatalf("expected 2 frozen seats in ledger, got %+v", frozenSeats)
	}
	if frozenSeats[0].SeatID != 71101 || frozenSeats[1].SeatID != 71102 {
		t.Fatalf("expected ledger frozen seats [71101 71102], got %+v", frozenSeats)
	}
}

func TestAutoAssignAndFreezeSeatsFallsBackToSortedFirstN(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	const programID int64 = 51102
	const ticketCategoryID int64 = 61102
	seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 71201, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 71202, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 3, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 71203, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 2, ColCode: 2, SeatStatus: testSeatStatusAvailable},
	)
	primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

	resp, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
		ShowTimeId:       programID,
		TicketCategoryId: ticketCategoryID,
		Count:            2,
		RequestNo:        "req-seat-ledger-fallback",
		FreezeSeconds:    900,
	})
	if err != nil {
		t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
	}
	if len(resp.Seats) != 2 {
		t.Fatalf("expected 2 seats, got %d", len(resp.Seats))
	}
	if resp.Seats[0].SeatId != 71201 || resp.Seats[1].SeatId != 71202 {
		t.Fatalf("expected sorted-first seats [71201 71202], got %+v", resp.Seats)
	}

	snapshot := requireProgramSeatLedgerSnapshot(t, svcCtx, programID, ticketCategoryID)
	if snapshot.AvailableCount != 1 {
		t.Fatalf("expected seat ledger available count to be 1, got %d", snapshot.AvailableCount)
	}
	frozenSeats := snapshot.FrozenSeats[resp.FreezeToken]
	if len(frozenSeats) != 2 {
		t.Fatalf("expected 2 frozen seats in ledger, got %+v", frozenSeats)
	}
	if frozenSeats[0].SeatID != 71201 || frozenSeats[1].SeatID != 71202 {
		t.Fatalf("expected ledger frozen seats [71201 71202], got %+v", frozenSeats)
	}
}

func TestAutoAssignAndFreezeSeatsRejectsWhenSeatLedgerMissing(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	const programID int64 = 51103
	const ticketCategoryID int64 = 61103
	seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 71301, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 71302, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 71303, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 2, ColCode: 1, SeatStatus: testSeatStatusAvailable},
	)
	clearProgramSeatLedger(t, svcCtx, programID, ticketCategoryID)

	_, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
		ShowTimeId:       programID,
		TicketCategoryId: ticketCategoryID,
		Count:            2,
		RequestNo:        "req-seat-ledger-missing",
		FreezeSeconds:    900,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
	if status.Convert(err).Message() != xerr.ErrProgramSeatLedgerNotReady.Error() {
		t.Fatalf("expected %q, got %v", xerr.ErrProgramSeatLedgerNotReady.Error(), err)
	}

	snapshot := waitProgramSeatLedgerReady(t, svcCtx, programID, ticketCategoryID, 3)
	if len(snapshot.AvailableSeats) != 3 {
		t.Fatalf("expected ledger to load 3 available seats, got %+v", snapshot.AvailableSeats)
	}
}
