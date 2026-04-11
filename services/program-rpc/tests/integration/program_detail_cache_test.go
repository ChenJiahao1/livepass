package integration_test

import (
	"context"
	"testing"
	"time"

	logicpkg "damai-go/services/program-rpc/internal/logic"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetProgramDetailUsesLocalCacheUntilInvalidate(t *testing.T) {
	const (
		programID      int64 = 81101
		programGroupID int64 = 82101
	)

	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	if err := svcCtx.ProgramCacheInvalidator.InvalidateProgram(context.Background(), programID, programGroupID); err != nil {
		t.Fatalf("clear stale program caches error: %v", err)
	}
	assertProgramRelatedCacheKeysMissing(t, svcCtx, programID, programGroupID)
	assertProgramMissingFromDB(t, svcCtx, programID)

	seedProgramFixtures(t, svcCtx, programFixture{
		ProgramID:               programID,
		ProgramGroupID:          programGroupID,
		ParentProgramCategoryID: 1,
		ProgramCategoryID:       11,
		AreaID:                  1,
		Title:                   "本地缓存测试演出",
		ShowTime:                "2026-12-21 19:30:00",
		ShowDayTime:             "2026-12-21 00:00:00",
		ShowWeekTime:            "周日",
		GroupAreaName:           "上海",
		TicketCategories: []ticketCategoryFixture{
			{ID: 83101, Introduce: "普通票", Price: 188, TotalNumber: 30, RemainNumber: 30},
			{ID: 83102, Introduce: "VIP票", Price: 388, TotalNumber: 20, RemainNumber: 20},
		},
	})

	l := logicpkg.NewGetProgramDetailLogic(context.Background(), svcCtx)

	first, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID})
	if err != nil {
		t.Fatalf("first GetProgramDetail returned error: %v", err)
	}
	if len(first.TicketCategoryVoList) != 2 {
		t.Fatalf("expected 2 ticket categories on first request, got %+v", first.TicketCategoryVoList)
	}

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()
	mustExecProgramSQL(t, db, "DELETE FROM d_ticket_category WHERE program_id = ?", programID)

	second, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID})
	if err != nil {
		t.Fatalf("second GetProgramDetail returned error: %v", err)
	}
	if len(second.TicketCategoryVoList) != 2 {
		t.Fatalf("expected second request to hit L1 cache and keep 2 ticket categories, got %+v", second.TicketCategoryVoList)
	}

	svcCtx.ProgramDetailCache.Invalidate(programID)

	third, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID})
	if err != nil {
		t.Fatalf("third GetProgramDetail after invalidate returned error: %v", err)
	}
	if len(third.TicketCategoryVoList) != 0 {
		t.Fatalf("expected invalidate to force reload empty ticket categories, got %+v", third.TicketCategoryVoList)
	}
}

func TestGetProgramDetailKeepsNotFoundSemanticsAcrossRepeatedAccess(t *testing.T) {
	const (
		programID      int64 = 81001
		programGroupID int64 = 82001
	)

	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	if err := svcCtx.ProgramCacheInvalidator.InvalidateProgram(context.Background(), programID, programGroupID); err != nil {
		t.Fatalf("clear stale program caches error: %v", err)
	}
	assertProgramRelatedCacheKeysMissing(t, svcCtx, programID, programGroupID)
	assertProgramMissingFromDB(t, svcCtx, programID)

	l := logicpkg.NewGetProgramDetailLogic(context.Background(), svcCtx)

	if _, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID}); status.Code(err) != codes.NotFound {
		t.Fatalf("expected first GetProgramDetail to return not found, got %v", err)
	}

	seedProgramFixtures(t, svcCtx, programFixture{
		ProgramID:               programID,
		ProgramGroupID:          programGroupID,
		ParentProgramCategoryID: 1,
		ProgramCategoryID:       11,
		AreaID:                  1,
		Title:                   "负缓存测试演出",
		ShowTime:                "2026-12-01 19:30:00",
		ShowDayTime:             "2026-12-01 00:00:00",
		ShowWeekTime:            "周二",
		GroupAreaName:           "上海",
		TicketCategories: []ticketCategoryFixture{
			{ID: 83001, Introduce: "普通票", Price: 188, TotalNumber: 30, RemainNumber: 30},
		},
	})

	if _, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID}); status.Code(err) != codes.NotFound {
		t.Fatalf("expected second GetProgramDetail to keep not found semantics, got %v", err)
	}
}

func TestGetProgramDetailReturnsBackfilledProgramImmediatelyAfterWrite(t *testing.T) {
	const (
		programID      int64 = 91001
		programGroupID int64 = 92001
	)

	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	if err := svcCtx.ProgramCacheInvalidator.InvalidateProgram(context.Background(), programID, programGroupID); err != nil {
		t.Fatalf("clear stale program caches error: %v", err)
	}
	assertProgramRelatedCacheKeysMissing(t, svcCtx, programID, programGroupID)
	assertProgramMissingFromDB(t, svcCtx, programID)

	l := logicpkg.NewGetProgramDetailLogic(context.Background(), svcCtx)

	if _, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID}); status.Code(err) != codes.NotFound {
		t.Fatalf("expected first GetProgramDetail to return not found, got %v", err)
	}

	seedProgramFixtures(t, svcCtx, programFixture{
		ProgramID:               programID,
		ProgramGroupID:          programGroupID,
		ParentProgramCategoryID: 1,
		ProgramCategoryID:       11,
		AreaID:                  1,
		Title:                   "新增后立即可见演出",
		ShowTime:                "2026-12-12 19:30:00",
		ShowDayTime:             "2026-12-12 00:00:00",
		ShowWeekTime:            "周六",
		GroupAreaName:           "上海",
		TicketCategories: []ticketCategoryFixture{
			{ID: 93001, Introduce: "普通票", Price: 288, TotalNumber: 60, RemainNumber: 60},
		},
	})
	if err := svcCtx.ProgramCacheInvalidator.InvalidateProgram(context.Background(), programID, programGroupID); err != nil {
		t.Fatalf("invalidate program caches error: %v", err)
	}
	assertProgramRelatedCacheKeysMissing(t, svcCtx, programID, programGroupID)

	resp, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID})
	if err != nil {
		t.Fatalf("expected backfilled program to be visible immediately, got %v", err)
	}
	if resp.Id != programID || resp.Title != "新增后立即可见演出" {
		t.Fatalf("unexpected backfilled program detail: %+v", resp)
	}
	if len(resp.TicketCategoryVoList) != 1 {
		t.Fatalf("expected backfilled ticket categories to be visible immediately, got %+v", resp.TicketCategoryVoList)
	}
}

func TestResetProgramDomainStateClearsRedisProgramCachesAcrossServiceContexts(t *testing.T) {
	const (
		programID     int64  = 10001
		programGroup  int64  = 20001
		baselineTitle string = "Phase1 示例演出"
		staleTitle    string = "reset 前遗留的 Redis 标题"
	)

	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
		if svcCtx.Redis == nil {
			return
		}

		if _, err := svcCtx.Redis.DelCtx(
			context.Background(),
			model.ProgramCacheKey(programID),
			model.ProgramFirstShowTimeCacheKey(programID),
			model.ProgramGroupCacheKey(programGroup),
		); err != nil {
			t.Fatalf("cleanup program redis cache error: %v", err)
		}
	})

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	mustExecProgramSQL(t, db, "UPDATE d_program SET title = ? WHERE id = ?", staleTitle, programID)
	if err := db.Close(); err != nil {
		t.Fatalf("close db error: %v", err)
	}

	staleLogic := logicpkg.NewGetProgramDetailLogic(context.Background(), svcCtx)
	staleResp, err := staleLogic.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID})
	if err != nil {
		t.Fatalf("prime stale program detail cache returned error: %v", err)
	}
	if staleResp.Title != staleTitle {
		t.Fatalf("expected stale title to be cached before reset, got %+v", staleResp)
	}

	resetProgramDomainState(t)

	freshSvcCtx := newProgramTestServiceContext(t)
	freshLogic := logicpkg.NewGetProgramDetailLogic(context.Background(), freshSvcCtx)
	freshResp, err := freshLogic.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID})
	if err != nil {
		t.Fatalf("GetProgramDetail after reset returned error: %v", err)
	}
	if freshResp.Title != baselineTitle {
		t.Fatalf("expected reset to clear stale redis caches and restore baseline title %q, got %+v", baselineTitle, freshResp)
	}
}

func TestProgramDetailCacheReloadAfterPeerBroadcastHitsFreshL2(t *testing.T) {
	const (
		programID      int64 = 160001
		programGroupID int64 = 160101
		oldTitle             = "广播刷新演出"
		newTitle             = "广播刷新演出-更新"
	)

	svcCtxA := newProgramTestServiceContext(t)
	svcCtxB := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	channel := programCachePubSubChannel(t, "detail-reload")
	configureProgramCachePublisher(t, svcCtxA, channel)
	configureProgramCachePublisher(t, svcCtxB, channel)
	startProgramCacheSubscriber(t, svcCtxA, channel, 1)
	startProgramCacheSubscriber(t, svcCtxB, channel, 2)

	seedProgramFixtures(t, svcCtxA, programFixture{
		ProgramID:               programID,
		ProgramGroupID:          programGroupID,
		ParentProgramCategoryID: 1,
		ProgramCategoryID:       11,
		AreaID:                  1,
		Title:                   oldTitle,
		ShowTime:                "2026-11-30 20:00:00",
		ShowDayTime:             "2026-11-30 00:00:00",
		ShowWeekTime:            "周六",
		GroupAreaName:           "上海",
		TicketCategories: []ticketCategoryFixture{
			{ID: 160201, Introduce: "普通票", Price: 188, TotalNumber: 20, RemainNumber: 20},
		},
	})

	lA := logicpkg.NewGetProgramDetailLogic(context.Background(), svcCtxA)
	first, err := lA.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID})
	if err != nil {
		t.Fatalf("first GetProgramDetail returned error: %v", err)
	}
	if first.Title != oldTitle {
		t.Fatalf("expected first title %q, got %+v", oldTitle, first)
	}

	db := openProgramTestDB(t, svcCtxA.Config.MySQL.DataSource)
	mustExecProgramSQL(t, db, "UPDATE d_program SET title = ?, edit_time = ? WHERE id = ?", newTitle, time.Now().Format(testProgramDateTimeLayout), programID)
	if err := db.Close(); err != nil {
		t.Fatalf("close db error: %v", err)
	}

	stale, err := lA.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID})
	if err != nil {
		t.Fatalf("second GetProgramDetail returned error: %v", err)
	}
	if stale.Title != oldTitle {
		t.Fatalf("expected local cache to keep old title before invalidation, got %+v", stale)
	}

	if err := svcCtxB.ProgramCacheInvalidator.InvalidateProgram(context.Background(), programID, programGroupID); err != nil {
		t.Fatalf("invalidate program caches error: %v", err)
	}

	waitForProgramDetailTitle(t, lA, programID, newTitle)
}

func assertProgramRelatedCacheKeysMissing(t *testing.T, svcCtx *svc.ServiceContext, programID, programGroupID int64) {
	t.Helper()

	if svcCtx.Redis == nil {
		return
	}

	for _, key := range []string{
		model.ProgramCacheKey(programID),
		model.ProgramFirstShowTimeCacheKey(programID),
		model.ProgramGroupCacheKey(programGroupID),
	} {
		exists, err := svcCtx.Redis.ExistsCtx(context.Background(), key)
		if err != nil {
			t.Fatalf("check redis key %s error: %v", key, err)
		}
		if exists {
			t.Fatalf("expected redis key %s to be deleted", key)
		}
	}
}

func assertProgramMissingFromDB(t *testing.T, svcCtx *svc.ServiceContext, programID int64) {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	var count int
	row := db.QueryRow("SELECT COUNT(1) FROM d_program WHERE id = ?", programID)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("query d_program count error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected program %d to be absent from db before backfill, got count=%d", programID, count)
	}
}
