package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/program-rpc/internal/logic"
	"damai-go/services/program-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetProgramDetailUsesLocalCacheUntilInvalidate(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	l := logicpkg.NewGetProgramDetailLogic(context.Background(), svcCtx)

	first, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: 10001})
	if err != nil {
		t.Fatalf("first GetProgramDetail returned error: %v", err)
	}
	if len(first.TicketCategoryVoList) != 2 {
		t.Fatalf("expected 2 ticket categories on first request, got %+v", first.TicketCategoryVoList)
	}

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()
	mustExecProgramSQL(t, db, "DELETE FROM d_ticket_category WHERE program_id = ?", 10001)

	second, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: 10001})
	if err != nil {
		t.Fatalf("second GetProgramDetail returned error: %v", err)
	}
	if len(second.TicketCategoryVoList) != 2 {
		t.Fatalf("expected second request to hit L1 cache and keep 2 ticket categories, got %+v", second.TicketCategoryVoList)
	}

	svcCtx.ProgramDetailCache.Invalidate(10001)

	third, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: 10001})
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
