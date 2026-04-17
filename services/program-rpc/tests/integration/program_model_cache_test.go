package integration_test

import (
	"context"
	"errors"
	"testing"

	"livepass/services/program-rpc/internal/model"
)

func TestProgramCachedModelsReturnCachedRowsAfterSourceDelete(t *testing.T) {
	const (
		programID      int64 = 71111
		programGroupID int64 = 72111
	)

	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	ctx := context.Background()
	if err := svcCtx.ProgramCacheInvalidator.InvalidateProgram(ctx, programID, programGroupID); err != nil {
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
		Title:                   "模型缓存测试演出",
		ShowTime:                "2026-11-21 19:30:00",
		ShowDayTime:             "2026-11-21 00:00:00",
		ShowWeekTime:            "周五",
		GroupAreaName:           "上海",
		TicketCategories: []ticketCategoryFixture{
			{ID: 73111, Introduce: "普通票", Price: 180, TotalNumber: 50, RemainNumber: 50},
		},
	})

	firstProgram, err := svcCtx.DProgramModel.FindOne(ctx, programID)
	if err != nil {
		t.Fatalf("first DProgramModel.FindOne returned error: %v", err)
	}
	firstGroup, err := svcCtx.DProgramGroupModel.FindOne(ctx, programGroupID)
	if err != nil {
		t.Fatalf("first DProgramGroupModel.FindOne returned error: %v", err)
	}
	firstShowTime, err := svcCtx.DProgramShowTimeModel.FindFirstByProgramId(ctx, programID)
	if err != nil {
		t.Fatalf("first DProgramShowTimeModel.FindFirstByProgramId returned error: %v", err)
	}

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	mustExecProgramSQL(t, db, "DELETE FROM d_program_show_time WHERE program_id = ?", programID)
	mustExecProgramSQL(t, db, "DELETE FROM d_program WHERE id = ?", programID)
	mustExecProgramSQL(t, db, "DELETE FROM d_program_group WHERE id = ?", programGroupID)

	secondProgram, err := svcCtx.DProgramModel.FindOne(ctx, programID)
	if err != nil {
		t.Fatalf("second DProgramModel.FindOne returned error: %v", err)
	}
	if secondProgram.Id != firstProgram.Id || secondProgram.Title != firstProgram.Title {
		t.Fatalf("expected cached program to stay stable, first=%+v second=%+v", firstProgram, secondProgram)
	}

	secondGroup, err := svcCtx.DProgramGroupModel.FindOne(ctx, programGroupID)
	if err != nil {
		t.Fatalf("second DProgramGroupModel.FindOne returned error: %v", err)
	}
	if secondGroup.Id != firstGroup.Id || secondGroup.RecentShowTime != firstGroup.RecentShowTime {
		t.Fatalf("expected cached group to stay stable, first=%+v second=%+v", firstGroup, secondGroup)
	}

	secondShowTime, err := svcCtx.DProgramShowTimeModel.FindFirstByProgramId(ctx, programID)
	if err != nil {
		t.Fatalf("second DProgramShowTimeModel.FindFirstByProgramId returned error: %v", err)
	}
	if secondShowTime.Id != firstShowTime.Id || !secondShowTime.ShowTime.Equal(firstShowTime.ShowTime) {
		t.Fatalf("expected cached show time to stay stable, first=%+v second=%+v", firstShowTime, secondShowTime)
	}
}

func TestProgramCachedModelsKeepNotFoundAfterImmediateBackfill(t *testing.T) {
	const (
		programID      int64 = 71001
		programGroupID int64 = 72001
	)

	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	ctx := context.Background()

	if _, err := svcCtx.DProgramModel.FindOne(ctx, programID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected DProgramModel.FindOne not found, got %v", err)
	}
	if _, err := svcCtx.DProgramShowTimeModel.FindFirstByProgramId(ctx, programID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected DProgramShowTimeModel.FindFirstByProgramId not found, got %v", err)
	}

	seedProgramFixtures(t, svcCtx, programFixture{
		ProgramID:               programID,
		ProgramGroupID:          programGroupID,
		ParentProgramCategoryID: 1,
		ProgramCategoryID:       11,
		AreaID:                  1,
		Title:                   "缓存回填测试演出",
		ShowTime:                "2026-11-11 19:30:00",
		ShowDayTime:             "2026-11-11 00:00:00",
		ShowWeekTime:            "周三",
		GroupAreaName:           "上海",
		TicketCategories: []ticketCategoryFixture{
			{ID: 73001, Introduce: "普通票", Price: 180, TotalNumber: 50, RemainNumber: 50},
		},
	})

	if _, err := svcCtx.DProgramModel.FindOne(ctx, programID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected DProgramModel.FindOne to keep not found placeholder, got %v", err)
	}
	if _, err := svcCtx.DProgramShowTimeModel.FindFirstByProgramId(ctx, programID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected DProgramShowTimeModel.FindFirstByProgramId to keep not found placeholder, got %v", err)
	}
}
