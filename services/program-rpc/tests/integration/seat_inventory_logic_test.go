package integration_test

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	logicpkg "damai-go/services/program-rpc/internal/logic"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testSeatStatusAvailable = 1
	testSeatStatusFrozen    = 2

	testFreezeStatusFrozen   = 1
	testFreezeStatusReleased = 2
	testFreezeStatusExpired  = 3
)

func TestAutoAssignAndFreezeSeats(t *testing.T) {
	t.Run("succeeds for seeded program and ticket category", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 51001
		const ticketCategoryID int64 = 61001
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 71001, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
			seatFixture{ID: 71002, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
			seatFixture{ID: 71003, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 2, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		)
		primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

		l := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx)
		resp, err := l.AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            2,
			RequestNo:        "req-seat-success",
			FreezeSeconds:    900,
		})
		if err != nil {
			t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
		}
		if resp.FreezeToken == "" {
			t.Fatalf("expected non-empty freeze token")
		}
		if resp.ExpireTime == "" {
			t.Fatalf("expected non-empty expire time")
		}
		if len(resp.Seats) != 2 {
			t.Fatalf("expected 2 seats, got %d", len(resp.Seats))
		}
		if resp.Seats[0].SeatId != 71001 || resp.Seats[1].SeatId != 71002 {
			t.Fatalf("expected consecutive seats [71001 71002], got %+v", resp.Seats)
		}

		if countSeatRowsByFreezeToken(t, svcCtx, resp.FreezeToken) != 0 {
			t.Fatalf("expected freeze stage not to persist seat rows in db")
		}
		if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusAvailable) != 3 {
			t.Fatalf("expected all seats to remain available in db before payment confirm")
		}

		freeze := querySeatFreezeByRequestNo(t, svcCtx, "req-seat-success")
		if freeze.FreezeStatus != testFreezeStatusFrozen {
			t.Fatalf("expected freeze status frozen, got %+v", freeze)
		}
		if freeze.FreezeToken != resp.FreezeToken {
			t.Fatalf("expected redis freeze token %q, got %+v", resp.FreezeToken, freeze)
		}
	})

	t.Run("falls back to split seats when no same-row consecutive block exists", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 51006
		const ticketCategoryID int64 = 61006
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 71101, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
			seatFixture{ID: 71102, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 3, SeatStatus: testSeatStatusAvailable},
			seatFixture{ID: 71103, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 2, ColCode: 2, SeatStatus: testSeatStatusAvailable},
		)
		primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

		l := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx)
		resp, err := l.AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            2,
			RequestNo:        "req-seat-split-fallback",
			FreezeSeconds:    900,
		})
		if err != nil {
			t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
		}
		if len(resp.Seats) != 2 {
			t.Fatalf("expected 2 seats, got %d", len(resp.Seats))
		}
		if resp.Seats[0].SeatId != 71101 || resp.Seats[1].SeatId != 71102 {
			t.Fatalf("expected split fallback seats [71101 71102], got %+v", resp.Seats)
		}
		if resp.Seats[0].RowCode != 1 || resp.Seats[0].ColCode != 1 || resp.Seats[1].RowCode != 1 || resp.Seats[1].ColCode != 3 {
			t.Fatalf("expected non-consecutive fallback seats at (1,1) and (1,3), got %+v", resp.Seats)
		}
	})

	t.Run("repeated requestNo returns same freeze token and seat set", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 51002
		const ticketCategoryID int64 = 61002
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 72001, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1},
			seatFixture{ID: 72002, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2},
			seatFixture{ID: 72003, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 3},
		)
		primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

		l := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx)
		first, err := l.AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            2,
			RequestNo:        "req-seat-idempotent",
			FreezeSeconds:    900,
		})
		if err != nil {
			t.Fatalf("first AutoAssignAndFreezeSeats returned error: %v", err)
		}

		second, err := l.AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            2,
			RequestNo:        "req-seat-idempotent",
			FreezeSeconds:    900,
		})
		if err != nil {
			t.Fatalf("second AutoAssignAndFreezeSeats returned error: %v", err)
		}

		if first.FreezeToken != second.FreezeToken {
			t.Fatalf("expected same freeze token, got %q and %q", first.FreezeToken, second.FreezeToken)
		}
		if !sameSeatIDs(first.Seats, second.Seats) {
			t.Fatalf("expected same seat set, got %+v and %+v", first.Seats, second.Seats)
		}
		if countSeatFreezesByRequestNo(t, svcCtx, "req-seat-idempotent") != 1 {
			t.Fatalf("expected exactly one freeze row for repeated requestNo")
		}
	})

	t.Run("expired freeze is recycled before a new freeze request runs", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 51003
		const ticketCategoryID int64 = 61003
		const expiredToken = "expired-freeze-token"
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 73001, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
			seatFixture{ID: 73002, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
			seatFixture{ID: 73003, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 2, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		)
		primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)
		seedRedisSeatFreezeFixture(t, svcCtx, seatFreezeFixture{
			ID:               83001,
			FreezeToken:      expiredToken,
			RequestNo:        "req-expired-old",
			ProgramID:        programID,
			TicketCategoryID: ticketCategoryID,
			SeatCount:        2,
			FreezeStatus:     testFreezeStatusFrozen,
			ExpireTime:       "2026-03-01 10:00:00",
		})

		l := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx)
		resp, err := l.AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            2,
			RequestNo:        "req-expired-new",
			FreezeSeconds:    900,
		})
		if err != nil {
			t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
		}
		if len(resp.Seats) != 2 {
			t.Fatalf("expected 2 seats, got %d", len(resp.Seats))
		}
		if querySeatFreezeByToken(t, svcCtx, expiredToken).FreezeStatus != testFreezeStatusExpired {
			t.Fatalf("expected expired redis freeze metadata to be marked expired")
		}
		if countSeatRowsByFreezeToken(t, svcCtx, expiredToken) != 0 {
			t.Fatalf("expected expired freeze token not to exist in db seats")
		}
		if countSeatRowsByFreezeToken(t, svcCtx, resp.FreezeToken) != 0 {
			t.Fatalf("expected new freeze token not to persist db seats before confirm")
		}
	})

	t.Run("insufficient available seats returns failed precondition", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 51004
		const ticketCategoryID int64 = 61004
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 74001, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1},
		)
		primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

		l := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx)
		_, err := l.AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            2,
			RequestNo:        "req-insufficient",
			FreezeSeconds:    900,
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("expected failed precondition, got %v", err)
		}
	})

	t.Run("missing ProgramShowTime returns not found", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 51005
		const ticketCategoryID int64 = 61005
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 75001, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1},
			seatFixture{ID: 75002, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2},
		)
		db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
		defer db.Close()
		mustExecProgramSQL(t, db, "DELETE FROM d_program_show_time WHERE program_id = ?", programID)

		l := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx)
		_, err := l.AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            2,
			RequestNo:        "req-show-time-missing",
			FreezeSeconds:    900,
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("expected not found, got %v", err)
		}
	})
}

func TestSeatInventoryQueriesAreScopedByShowTime(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	const (
		programID         int64 = 59001
		showTimeOneID     int64 = 69001
		showTimeTwoID     int64 = 69002
		ticketCategoryOne int64 = 79001
		ticketCategoryTwo int64 = 79002
	)

	seedProgramFixtures(t, svcCtx, programFixture{
		ProgramID:               programID,
		ProgramGroupID:          59901,
		ParentProgramCategoryID: 1,
		ProgramCategoryID:       11,
		AreaID:                  1,
		ShowTimeID:              showTimeOneID,
		ShowTime:                "2026-12-31 19:30:00",
		ShowDayTime:             "2026-12-31 00:00:00",
		ShowWeekTime:            "周四",
		RushSaleOpenTime:        "2026-12-31 18:00:00",
		RushSaleEndTime:         "2026-12-31 19:00:00",
		ShowEndTime:             "2026-12-31 22:30:00",
		TicketCategories: []ticketCategoryFixture{
			{ID: ticketCategoryOne, ShowTimeID: showTimeOneID, Introduce: "第一场普通票", Price: 299, TotalNumber: 2, RemainNumber: 2},
		},
	})

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	mustExecProgramSQL(
		t,
		db,
		`INSERT INTO d_program_show_time (
			id, program_id, show_time, show_day_time, show_week_time,
			rush_sale_open_time, rush_sale_end_time, show_end_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		showTimeTwoID,
		programID,
		"2027-01-01 19:30:00",
		"2027-01-01 00:00:00",
		"周五",
		"2027-01-01 18:00:00",
		"2027-01-01 19:00:00",
		"2027-01-01 22:30:00",
		"2026-01-01 00:00:00",
		"2026-01-01 00:00:00",
		1,
	)
	mustExecProgramSQL(
		t,
		db,
		`INSERT INTO d_ticket_category (
			id, program_id, show_time_id, introduce, price, total_number, remain_number, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ticketCategoryTwo,
		programID,
		showTimeTwoID,
		"第二场普通票",
		399,
		3,
		3,
		"2026-01-01 00:00:00",
		"2026-01-01 00:00:00",
		1,
	)

	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 89001, ProgramID: programID, ShowTimeID: showTimeOneID, TicketCategoryID: ticketCategoryOne, RowCode: 1, ColCode: 1},
		seatFixture{ID: 89002, ProgramID: programID, ShowTimeID: showTimeOneID, TicketCategoryID: ticketCategoryOne, RowCode: 1, ColCode: 2},
		seatFixture{ID: 89003, ProgramID: programID, ShowTimeID: showTimeTwoID, TicketCategoryID: ticketCategoryTwo, RowCode: 1, ColCode: 1},
		seatFixture{ID: 89004, ProgramID: programID, ShowTimeID: showTimeTwoID, TicketCategoryID: ticketCategoryTwo, RowCode: 1, ColCode: 2},
		seatFixture{ID: 89005, ProgramID: programID, ShowTimeID: showTimeTwoID, TicketCategoryID: ticketCategoryTwo, RowCode: 1, ColCode: 3},
	)

	ctx := context.Background()
	secondShowTime, err := svcCtx.DProgramShowTimeModel.FindOne(ctx, showTimeTwoID)
	if err != nil {
		t.Fatalf("FindOne(showTimeID) returned error: %v", err)
	}
	if secondShowTime.Id != showTimeTwoID {
		t.Fatalf("expected showTimeID=%d, got %+v", showTimeTwoID, secondShowTime)
	}

	firstCategories, err := svcCtx.DTicketCategoryModel.FindByShowTimeId(ctx, showTimeOneID)
	if err != nil {
		t.Fatalf("FindByShowTimeId(first) returned error: %v", err)
	}
	if len(firstCategories) != 1 || firstCategories[0].Id != ticketCategoryOne {
		t.Fatalf("expected only first show time ticket category, got %+v", firstCategories)
	}

	secondCategories, err := svcCtx.DTicketCategoryModel.FindByShowTimeId(ctx, showTimeTwoID)
	if err != nil {
		t.Fatalf("FindByShowTimeId(second) returned error: %v", err)
	}
	if len(secondCategories) != 1 || secondCategories[0].Id != ticketCategoryTwo {
		t.Fatalf("expected only second show time ticket category, got %+v", secondCategories)
	}

	firstRemain, err := svcCtx.DSeatModel.FindAvailableCountByShowTimeId(ctx, showTimeOneID)
	if err != nil {
		t.Fatalf("FindAvailableCountByShowTimeId(first) returned error: %v", err)
	}
	if len(firstRemain) != 1 || firstRemain[0].TicketCategoryId != ticketCategoryOne || firstRemain[0].RemainNumber != 2 {
		t.Fatalf("expected first show time remain aggregate to be isolated, got %+v", firstRemain)
	}

	secondRemain, err := svcCtx.DSeatModel.FindAvailableCountByShowTimeId(ctx, showTimeTwoID)
	if err != nil {
		t.Fatalf("FindAvailableCountByShowTimeId(second) returned error: %v", err)
	}
	if len(secondRemain) != 1 || secondRemain[0].TicketCategoryId != ticketCategoryTwo || secondRemain[0].RemainNumber != 3 {
		t.Fatalf("expected second show time remain aggregate to be isolated, got %+v", secondRemain)
	}

	if err := svcCtx.SqlConn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		seats, err := svcCtx.DSeatModel.FindByShowTimeAndIDsForUpdate(ctx, session, showTimeTwoID, []int64{89003, 89005})
		if err != nil {
			return err
		}
		if len(seats) != 2 {
			t.Fatalf("expected 2 locked seats, got %+v", seats)
		}
		if seats[0].Id != 89003 || seats[1].Id != 89005 {
			t.Fatalf("expected locked seats to stay scoped to second show time, got %+v", seats)
		}
		return nil
	}); err != nil {
		t.Fatalf("FindByShowTimeAndIDsForUpdate transaction returned error: %v", err)
	}
}

func TestReleaseSeatFreeze(t *testing.T) {
	t.Run("release by freezeToken succeeds and seats become available again", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 52001
		const ticketCategoryID int64 = 62001
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 76001, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1},
			seatFixture{ID: 76002, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2},
			seatFixture{ID: 76003, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 2, ColCode: 1},
		)
		primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

		autoLogic := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx)
		freezeResp, err := autoLogic.AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            2,
			RequestNo:        "req-release-success",
			FreezeSeconds:    900,
		})
		if err != nil {
			t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
		}

		releaseLogic := logicpkg.NewReleaseSeatFreezeLogic(context.Background(), svcCtx)
		releaseResp, err := releaseLogic.ReleaseSeatFreeze(&pb.ReleaseSeatFreezeReq{
			FreezeToken:   freezeResp.FreezeToken,
			ReleaseReason: "order canceled",
		})
		if err != nil {
			t.Fatalf("ReleaseSeatFreeze returned error: %v", err)
		}
		if !releaseResp.Success {
			t.Fatalf("expected release success")
		}

		if countSeatRowsByFreezeToken(t, svcCtx, freezeResp.FreezeToken) != 0 {
			t.Fatalf("expected all seats released for freeze token %q", freezeResp.FreezeToken)
		}
		if countSeatRowsByStatus(t, svcCtx, programID, ticketCategoryID, testSeatStatusAvailable) != 3 {
			t.Fatalf("expected db seats to stay available after release")
		}
		if querySeatFreezeByToken(t, svcCtx, freezeResp.FreezeToken).FreezeStatus != testFreezeStatusReleased {
			t.Fatalf("expected redis freeze metadata to be marked released")
		}
	})

	t.Run("repeated release is idempotent", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 52002
		const ticketCategoryID int64 = 62002
		seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
		seedSeatFixtures(t, svcCtx,
			seatFixture{ID: 77001, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1},
			seatFixture{ID: 77002, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2},
		)
		primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

		autoLogic := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx)
		freezeResp, err := autoLogic.AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
			ShowTimeId:       programID,
			TicketCategoryId: ticketCategoryID,
			Count:            2,
			RequestNo:        "req-release-idempotent",
			FreezeSeconds:    900,
		})
		if err != nil {
			t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
		}

		releaseLogic := logicpkg.NewReleaseSeatFreezeLogic(context.Background(), svcCtx)
		for i := 0; i < 2; i++ {
			resp, err := releaseLogic.ReleaseSeatFreeze(&pb.ReleaseSeatFreezeReq{
				FreezeToken:   freezeResp.FreezeToken,
				ReleaseReason: "idempotent retry",
			})
			if err != nil {
				t.Fatalf("ReleaseSeatFreeze returned error on attempt %d: %v", i+1, err)
			}
			if !resp.Success {
				t.Fatalf("expected release success on attempt %d", i+1)
			}
		}
	})
}

func TestConcurrentSeatFreezeDoesNotOverlap(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	const programID int64 = 53001
	const ticketCategoryID int64 = 63001
	seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 78001, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1},
		seatFixture{ID: 78002, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2},
		seatFixture{ID: 78003, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 3},
		seatFixture{ID: 78004, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 4},
	)
	primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		errs  []error
		resps []*pb.AutoAssignAndFreezeSeatsResp
	)

	requests := []*pb.AutoAssignAndFreezeSeatsReq{
		{ShowTimeId: programID, TicketCategoryId: ticketCategoryID, Count: 2, RequestNo: "req-concurrent-1", FreezeSeconds: 900},
		{ShowTimeId: programID, TicketCategoryId: ticketCategoryID, Count: 2, RequestNo: "req-concurrent-2", FreezeSeconds: 900},
	}

	for _, req := range requests {
		wg.Add(1)
		go func(req *pb.AutoAssignAndFreezeSeatsReq) {
			defer wg.Done()

			resp, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(req)

			mu.Lock()
			defer mu.Unlock()
			errs = append(errs, err)
			resps = append(resps, resp)
		}(req)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("expected concurrent freeze success, got error: %v", err)
		}
	}
	if len(resps) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(resps))
	}
	if len(resps[0].Seats) != 2 || len(resps[1].Seats) != 2 {
		t.Fatalf("expected each response to contain 2 seats, got %+v", resps)
	}

	seen := make(map[int64]struct{}, 4)
	for _, resp := range resps {
		for _, seat := range resp.Seats {
			if _, ok := seen[seat.SeatId]; ok {
				t.Fatalf("seat %d allocated more than once", seat.SeatId)
			}
			seen[seat.SeatId] = struct{}{}
		}
	}
}

func TestConcurrentSeatFreezeUsesDifferentHotspotKeysPerTicketCategory(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	const programID int64 = 53002
	const ticketCategoryIDOne int64 = 63011
	const ticketCategoryIDTwo int64 = 63012

	seedProgramFixtures(t, svcCtx, programFixture{
		ProgramID:               programID,
		ProgramGroupID:          programID + 1000,
		ParentProgramCategoryID: 1,
		ProgramCategoryID:       11,
		AreaID:                  1,
		Title:                   "不同票档热点锁测试演出",
		ShowTime:                "2026-12-31 19:30:00",
		ShowDayTime:             "2026-12-31 00:00:00",
		ShowWeekTime:            "周四",
		TicketCategories: []ticketCategoryFixture{
			{ID: ticketCategoryIDOne, Introduce: "普通票A", Price: 299, TotalNumber: 100, RemainNumber: 100},
			{ID: ticketCategoryIDTwo, Introduce: "普通票B", Price: 399, TotalNumber: 100, RemainNumber: 100},
		},
	})
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 78101, ProgramID: programID, TicketCategoryID: ticketCategoryIDOne, RowCode: 1, ColCode: 1},
		seatFixture{ID: 78102, ProgramID: programID, TicketCategoryID: ticketCategoryIDOne, RowCode: 1, ColCode: 2},
		seatFixture{ID: 78201, ProgramID: programID, TicketCategoryID: ticketCategoryIDTwo, RowCode: 2, ColCode: 1},
		seatFixture{ID: 78202, ProgramID: programID, TicketCategoryID: ticketCategoryIDTwo, RowCode: 2, ColCode: 2},
	)
	primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryIDOne)
	primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryIDTwo)

	recordingLocker := &recordingSeatFreezeLocker{}
	svcCtx.SeatFreezeLocker = recordingLocker

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	requests := []*pb.AutoAssignAndFreezeSeatsReq{
		{ShowTimeId: programID, TicketCategoryId: ticketCategoryIDOne, Count: 1, RequestNo: "req-hotspot-cat-1", FreezeSeconds: 900},
		{ShowTimeId: programID, TicketCategoryId: ticketCategoryIDTwo, Count: 1, RequestNo: "req-hotspot-cat-2", FreezeSeconds: 900},
	}

	for _, req := range requests {
		wg.Add(1)
		go func(req *pb.AutoAssignAndFreezeSeatsReq) {
			defer wg.Done()
			_, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(req)
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}(req)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("expected concurrent freeze success for different categories, got %v", err)
		}
	}

	keys := recordingLocker.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 lock keys, got %v", keys)
	}
	if keys[0] == keys[1] {
		t.Fatalf("expected different ticket categories to use different hotspot keys, got %v", keys)
	}
}

type recordingSeatFreezeLocker struct {
	mu   sync.Mutex
	keys []string
}

func (l *recordingSeatFreezeLocker) Lock(key string) func() {
	l.mu.Lock()
	l.keys = append(l.keys, key)
	l.mu.Unlock()
	return func() {}
}

func (l *recordingSeatFreezeLocker) Keys() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	keys := make([]string, len(l.keys))
	copy(keys, l.keys)
	return keys
}

type seatRow struct {
	ID          int64
	SeatStatus  int
	FreezeToken sql.NullString
}

type seatFreezeRow struct {
	FreezeToken  string
	RequestNo    string
	FreezeStatus int
}

func querySeatRowsByFreezeToken(t *testing.T, svcCtx *svc.ServiceContext, freezeToken string) []seatRow {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	rows, err := db.Query(`SELECT id, seat_status, freeze_token FROM d_seat WHERE status = 1 AND freeze_token = ? ORDER BY row_code ASC, col_code ASC`, freezeToken)
	if err != nil {
		t.Fatalf("query seats by freeze token error: %v", err)
	}
	defer rows.Close()

	var resp []seatRow
	for rows.Next() {
		var row seatRow
		if err := rows.Scan(&row.ID, &row.SeatStatus, &row.FreezeToken); err != nil {
			t.Fatalf("scan seat row error: %v", err)
		}
		resp = append(resp, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate seat rows error: %v", err)
	}

	return resp
}

func querySeatFreezeByRequestNo(t *testing.T, svcCtx *svc.ServiceContext, requestNo string) seatFreezeRow {
	t.Helper()

	meta := requireSeatFreezeMetadataByRequestNo(t, svcCtx, requestNo)
	return seatFreezeRow{
		FreezeToken:  meta.FreezeToken,
		RequestNo:    meta.RequestNo,
		FreezeStatus: int(meta.FreezeStatus),
	}
}

func querySeatFreezeByToken(t *testing.T, svcCtx *svc.ServiceContext, freezeToken string) seatFreezeRow {
	t.Helper()

	meta := requireSeatFreezeMetadataByToken(t, svcCtx, freezeToken)
	return seatFreezeRow{
		FreezeToken:  meta.FreezeToken,
		RequestNo:    meta.RequestNo,
		FreezeStatus: int(meta.FreezeStatus),
	}
}

func countSeatFreezesByRequestNo(t *testing.T, svcCtx *svc.ServiceContext, requestNo string) int {
	t.Helper()

	if _, ok := findSeatFreezeMetadataByRequestNo(t, svcCtx, requestNo); ok {
		return 1
	}

	return 0
}

func countSeatRowsByFreezeToken(t *testing.T, svcCtx *svc.ServiceContext, freezeToken string) int {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(1) FROM d_seat WHERE status = 1 AND freeze_token = ?`,
		freezeToken,
	).Scan(&count); err != nil {
		t.Fatalf("count seat rows by freeze token error: %v", err)
	}

	return count
}

func countSeatRowsByStatus(t *testing.T, svcCtx *svc.ServiceContext, programID, ticketCategoryID int64, seatStatus int) int {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(1) FROM d_seat WHERE status = 1 AND program_id = ? AND ticket_category_id = ? AND seat_status = ?`,
		programID,
		ticketCategoryID,
		seatStatus,
	).Scan(&count); err != nil {
		t.Fatalf("count seat rows by status error: %v", err)
	}

	return count
}

func sameSeatIDs(first, second []*pb.SeatInfo) bool {
	if len(first) != len(second) {
		return false
	}

	for i := range first {
		if first[i].SeatId != second[i].SeatId {
			return false
		}
	}

	return true
}
