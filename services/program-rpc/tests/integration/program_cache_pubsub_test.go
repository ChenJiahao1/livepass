package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	logicpkg "damai-go/services/program-rpc/internal/logic"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"
)

func TestProgramDetailPubSubInvalidatesPeerLocalCache(t *testing.T) {
	const (
		programID      int64 = 150001
		programGroupID int64 = 150101
		oldTitle             = "跨实例缓存演出"
		newTitle             = "跨实例缓存演出-更新"
	)

	svcCtxA := newProgramTestServiceContext(t)
	svcCtxB := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	channel := programCachePubSubChannel(t, "detail")
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
		ShowTime:                "2026-11-11 20:00:00",
		ShowDayTime:             "2026-11-11 00:00:00",
		ShowWeekTime:            "周三",
		GroupAreaName:           "上海",
		TicketCategories: []ticketCategoryFixture{
			{ID: 150201, Introduce: "普通票", Price: 188, TotalNumber: 20, RemainNumber: 20},
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
		t.Fatalf("expected cache to keep old title before invalidation, got %+v", stale)
	}

	if err := svcCtxB.ProgramCacheInvalidator.InvalidateProgram(context.Background(), programID, programGroupID); err != nil {
		t.Fatalf("invalidate program caches error: %v", err)
	}

	waitForProgramDetailTitle(t, lA, programID, newTitle)
}

func TestCategorySnapshotPubSubInvalidatesPeerLocalCache(t *testing.T) {
	svcCtxA := newProgramTestServiceContext(t)
	svcCtxB := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	channel := programCachePubSubChannel(t, "category")
	configureProgramCachePublisher(t, svcCtxA, channel)
	configureProgramCachePublisher(t, svcCtxB, channel)
	startProgramCacheSubscriber(t, svcCtxA, channel, 1)
	startProgramCacheSubscriber(t, svcCtxB, channel, 2)

	categoryID, oldName := requireProgramCategorySample(t, svcCtxA)

	categories, err := svcCtxA.CategorySnapshotCache.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll program category snapshot error: %v", err)
	}
	if got := findProgramCategoryName(categories, categoryID); got != oldName {
		t.Fatalf("expected category name %q, got %q", oldName, got)
	}

	newName := fmt.Sprintf("%s-更新", oldName)
	db := openProgramTestDB(t, svcCtxA.Config.MySQL.DataSource)
	mustExecProgramSQL(t, db, "UPDATE d_program_category SET name = ?, edit_time = ? WHERE id = ?", newName, time.Now().Format(testProgramDateTimeLayout), categoryID)
	if err := db.Close(); err != nil {
		t.Fatalf("close db error: %v", err)
	}

	cached, err := svcCtxA.CategorySnapshotCache.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll program category snapshot before invalidation error: %v", err)
	}
	if got := findProgramCategoryName(cached, categoryID); got != oldName {
		t.Fatalf("expected snapshot cache to keep old name before invalidation, got %q", got)
	}

	if err := svcCtxB.ProgramCacheInvalidator.InvalidateCategorySnapshot(context.Background()); err != nil {
		t.Fatalf("invalidate category snapshot error: %v", err)
	}

	waitForCategorySnapshotName(t, svcCtxA, categoryID, newName)
}

func waitForProgramDetailTitle(t *testing.T, logic *logicpkg.GetProgramDetailLogic, programID int64, expected string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := logic.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID})
		if err != nil {
			t.Fatalf("GetProgramDetail during wait returned error: %v", err)
		}
		if resp.Title == expected {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected program title to be refreshed to %q before deadline", expected)
}

func waitForCategorySnapshotName(t *testing.T, svcCtx *svc.ServiceContext, categoryID int64, expected string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		categories, err := svcCtx.CategorySnapshotCache.GetAll(context.Background())
		if err != nil {
			t.Fatalf("GetAll program category snapshot during wait error: %v", err)
		}
		if got := findProgramCategoryName(categories, categoryID); got == expected {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected category name to be refreshed to %q before deadline", expected)
}

func requireProgramCategorySample(t *testing.T, svcCtx *svc.ServiceContext) (int64, string) {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	var id int64
	var name string
	if err := db.QueryRow("SELECT id, name FROM d_program_category WHERE status = 1 LIMIT 1").Scan(&id, &name); err != nil {
		t.Fatalf("query sample program category error: %v", err)
	}

	return id, name
}

func findProgramCategoryName(categories []*model.DProgramCategory, categoryID int64) string {
	for _, category := range categories {
		if category != nil && category.Id == categoryID {
			return category.Name
		}
	}
	return ""
}

func programCachePubSubChannel(t *testing.T, suffix string) string {
	t.Helper()

	name := t.Name()
	if idx := strings.IndexByte(name, '/'); idx >= 0 {
		name = name[:idx]
	}
	name = strings.NewReplacer("/", ":", " ", ":", "\t", ":", "#", ":").Replace(name)
	return fmt.Sprintf("damai-go:test:program:cache:invalidate:%s:%s", name, suffix)
}
