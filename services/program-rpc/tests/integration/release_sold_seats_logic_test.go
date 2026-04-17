package integration_test

import (
	"context"
	"testing"

	logicpkg "livepass/services/program-rpc/internal/logic"
	"livepass/services/program-rpc/pb"
)

func TestReleaseSoldSeats(t *testing.T) {
	t.Run("sold seats become available again", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 55001
		const ticketCategoryID int64 = 65001
		const freezeToken = "sold-release-success"
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 85001, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusSold, FreezeToken: freezeToken, FreezeExpireTime: "2026-12-31 20:00:00"},
			seatFixture{ID: 85002, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusSold, FreezeToken: freezeToken, FreezeExpireTime: "2026-12-31 20:00:00"},
		)

		l := logicpkg.NewReleaseSoldSeatsLogic(context.Background(), svcCtx)
		resp, err := l.ReleaseSoldSeats(&pb.ReleaseSoldSeatsReq{
			ShowTimeId: programID,
			SeatIds:    []int64{85001, 85002},
			RequestNo:  "refund-release-success",
		})
		if err != nil {
			t.Fatalf("ReleaseSoldSeats returned error: %v", err)
		}
		if !resp.Success {
			t.Fatalf("expected success response")
		}
		if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusAvailable) != 2 {
			t.Fatalf("expected both seats to become available")
		}
		if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusSold) != 0 {
			t.Fatalf("expected no sold seats to remain")
		}
		if countSeatRowsByFreezeToken(t, svcCtx, freezeToken) != 0 {
			t.Fatalf("expected freeze token cleared from released seats")
		}
	})

	t.Run("same requestNo is idempotent", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 55002
		const ticketCategoryID int64 = 65002
		const freezeToken = "sold-release-idempotent"
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 85101, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusSold, FreezeToken: freezeToken, FreezeExpireTime: "2026-12-31 20:00:00"},
			seatFixture{ID: 85102, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusSold, FreezeToken: freezeToken, FreezeExpireTime: "2026-12-31 20:00:00"},
		)

		l := logicpkg.NewReleaseSoldSeatsLogic(context.Background(), svcCtx)
		if _, err := l.ReleaseSoldSeats(&pb.ReleaseSoldSeatsReq{
			ShowTimeId: programID,
			SeatIds:    []int64{85101, 85102},
			RequestNo:  "refund-release-idempotent",
		}); err != nil {
			t.Fatalf("first ReleaseSoldSeats returned error: %v", err)
		}

		resp, err := l.ReleaseSoldSeats(&pb.ReleaseSoldSeatsReq{
			ShowTimeId: programID,
			SeatIds:    []int64{85101, 85102},
			RequestNo:  "refund-release-idempotent",
		})
		if err != nil {
			t.Fatalf("second ReleaseSoldSeats returned error: %v", err)
		}
		if !resp.Success {
			t.Fatalf("expected idempotent success response")
		}
		if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusAvailable) != 2 {
			t.Fatalf("expected seats to remain available after repeated request")
		}
	})
}
