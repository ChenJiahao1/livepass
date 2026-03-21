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
