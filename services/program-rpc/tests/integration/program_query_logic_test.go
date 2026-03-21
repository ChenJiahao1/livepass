package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/program-rpc/internal/logic"
	"damai-go/services/program-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestListProgramCategoriesReturnsSeededCategories(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	l := logicpkg.NewListProgramCategoriesLogic(context.Background(), svcCtx)
	resp, err := l.ListProgramCategories(&pb.Empty{})
	if err != nil {
		t.Fatalf("ListProgramCategories returned error: %v", err)
	}
	if len(resp.List) != 4 {
		t.Fatalf("expected 4 categories, got %d", len(resp.List))
	}

	expected := []struct {
		id       int64
		parentID int64
		name     string
		tp       int64
	}{
		{id: 1, parentID: 0, name: "演唱会", tp: 1},
		{id: 2, parentID: 0, name: "话剧歌剧", tp: 1},
		{id: 11, parentID: 1, name: "livehouse", tp: 2},
		{id: 12, parentID: 2, name: "话剧", tp: 2},
	}

	for i, want := range expected {
		got := resp.List[i]
		if got.Id != want.id || got.ParentId != want.parentID || got.Name != want.name || got.Type != want.tp {
			t.Fatalf("unexpected category at index %d: %+v", i, got)
		}
	}
}

func TestListHomeProgramsGroupsByRequestedParentCategoryOrder(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	seedProgramFixtures(t, svcCtx, programFixture{
		ProgramID:               10002,
		ProgramGroupID:          20002,
		ParentProgramCategoryID: 2,
		ProgramCategoryID:       12,
		AreaID:                  1,
		Title:                   "Phase1 话剧示例",
		ShowTime:                "2026-11-01 20:00:00",
		ShowDayTime:             "2026-11-01 00:00:00",
		ShowWeekTime:            "周日",
		GroupAreaName:           "上海",
		TicketCategories: []ticketCategoryFixture{
			{ID: 40003, Introduce: "普通票", Price: 199, TotalNumber: 90, RemainNumber: 60},
			{ID: 40004, Introduce: "VIP票", Price: 399, TotalNumber: 20, RemainNumber: 12},
		},
	})

	l := logicpkg.NewListHomeProgramsLogic(context.Background(), svcCtx)
	resp, err := l.ListHomePrograms(&pb.ListHomeProgramsReq{
		ParentProgramCategoryIds: []int64{2, 1},
	})
	if err != nil {
		t.Fatalf("ListHomePrograms returned error: %v", err)
	}
	if len(resp.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(resp.Sections))
	}

	first := resp.Sections[0]
	if first.CategoryId != 2 || first.CategoryName != "话剧歌剧" {
		t.Fatalf("unexpected first section: %+v", first)
	}
	if len(first.ProgramListVoList) != 1 {
		t.Fatalf("expected 1 program in first section, got %d", len(first.ProgramListVoList))
	}
	if first.ProgramListVoList[0].Id != 10002 || first.ProgramListVoList[0].Title != "Phase1 话剧示例" {
		t.Fatalf("unexpected first section program: %+v", first.ProgramListVoList[0])
	}
	if first.ProgramListVoList[0].MinPrice != 199 || first.ProgramListVoList[0].MaxPrice != 399 {
		t.Fatalf("unexpected first section price range: %+v", first.ProgramListVoList[0])
	}

	second := resp.Sections[1]
	if second.CategoryId != 1 || second.CategoryName != "演唱会" {
		t.Fatalf("unexpected second section: %+v", second)
	}
	if len(second.ProgramListVoList) != 1 {
		t.Fatalf("expected 1 program in second section, got %d", len(second.ProgramListVoList))
	}
	if second.ProgramListVoList[0].Id != 10001 || second.ProgramListVoList[0].Title != "Phase1 示例演出" {
		t.Fatalf("unexpected second section program: %+v", second.ProgramListVoList[0])
	}
}

func TestPageProgramsAppliesTimeTypeCategoryAndSortType(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	seedProgramFixtures(
		t,
		svcCtx,
		programFixture{
			ProgramID:               10003,
			ProgramGroupID:          20003,
			ParentProgramCategoryID: 1,
			ProgramCategoryID:       11,
			AreaID:                  2,
			Title:                   "十二月早场",
			HighHeat:                3,
			IssueTime:               "2026-04-01 09:00:00",
			ShowTime:                "2026-12-10 19:30:00",
			ShowDayTime:             "2026-12-10 00:00:00",
			ShowWeekTime:            "周四",
			GroupAreaName:           "北京",
			TicketCategories: []ticketCategoryFixture{
				{ID: 40005, Introduce: "普通票", Price: 180, TotalNumber: 100, RemainNumber: 90},
				{ID: 40006, Introduce: "VIP票", Price: 380, TotalNumber: 50, RemainNumber: 45},
			},
		},
		programFixture{
			ProgramID:               10004,
			ProgramGroupID:          20004,
			ParentProgramCategoryID: 1,
			ProgramCategoryID:       11,
			AreaID:                  2,
			Title:                   "十二月晚场",
			HighHeat:                8,
			IssueTime:               "2026-05-01 09:00:00",
			ShowTime:                "2026-12-20 19:30:00",
			ShowDayTime:             "2026-12-20 00:00:00",
			ShowWeekTime:            "周日",
			GroupAreaName:           "北京",
			TicketCategories: []ticketCategoryFixture{
				{ID: 40007, Introduce: "普通票", Price: 280, TotalNumber: 100, RemainNumber: 80},
				{ID: 40008, Introduce: "VIP票", Price: 480, TotalNumber: 50, RemainNumber: 30},
			},
		},
	)

	l := logicpkg.NewPageProgramsLogic(context.Background(), svcCtx)
	resp, err := l.PagePrograms(&pb.PageProgramsReq{
		PageNumber:        1,
		PageSize:          10,
		ProgramCategoryId: 11,
		TimeType:          5,
		StartDateTime:     "2026-12-01 00:00:00",
		EndDateTime:       "2026-12-25 23:59:59",
		Type:              3,
	})
	if err != nil {
		t.Fatalf("PagePrograms returned error: %v", err)
	}
	if resp.PageNum != 1 || resp.PageSize != 10 || resp.TotalSize != 2 {
		t.Fatalf("unexpected page info: %+v", resp)
	}
	if len(resp.List) != 2 {
		t.Fatalf("expected 2 programs, got %d", len(resp.List))
	}
	if resp.List[0].Id != 10003 || resp.List[1].Id != 10004 {
		t.Fatalf("unexpected program order: %+v", resp.List)
	}
	if resp.List[0].MinPrice != 180 || resp.List[0].MaxPrice != 380 {
		t.Fatalf("unexpected first program price range: %+v", resp.List[0])
	}
	if resp.List[1].ShowTime != "2026-12-20 19:30:00" {
		t.Fatalf("unexpected second program show time: %+v", resp.List[1])
	}
}

func TestGetProgramDetailReturnsComposedProgramInfo(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	l := logicpkg.NewGetProgramDetailLogic(context.Background(), svcCtx)
	resp, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: 10001})
	if err != nil {
		t.Fatalf("GetProgramDetail returned error: %v", err)
	}
	if resp.Id != 10001 || resp.ProgramGroupId != 20001 || resp.Title != "Phase1 示例演出" {
		t.Fatalf("unexpected program detail base fields: %+v", resp)
	}
	if resp.AreaId != 2 || resp.AreaName != "" {
		t.Fatalf("unexpected area fields: %+v", resp)
	}
	if resp.ProgramCategoryName != "livehouse" || resp.ParentProgramCategoryName != "演唱会" {
		t.Fatalf("unexpected category fields: %+v", resp)
	}
	if resp.ShowTime != "2026-12-31 19:30:00" || resp.ShowDayTime != "2026-12-31 00:00:00" || resp.ShowWeekTime != "周四" {
		t.Fatalf("unexpected show time fields: %+v", resp)
	}
	if resp.PermitRefund != 1 || resp.RefundTicketRule != "演出开始前 120 分钟外可退，开演前 24 小时外退 80%，开演前 7 天外退 100%。" || resp.RefundExplain != "请按退票规则办理。" {
		t.Fatalf("unexpected refund fields: %+v", resp)
	}
	if resp.PermitChooseSeat != 0 || resp.ChooseSeatExplain != "本项目不支持自主选座，同一个订单优先连座。" {
		t.Fatalf("unexpected choose seat fields: %+v", resp)
	}
	if resp.ProgramGroupVo == nil || resp.ProgramGroupVo.Id != 20001 || resp.ProgramGroupVo.RecentShowTime != "2026-12-31 19:30:00" {
		t.Fatalf("unexpected program group: %+v", resp.ProgramGroupVo)
	}
	if len(resp.ProgramGroupVo.ProgramSimpleInfoVoList) != 1 {
		t.Fatalf("expected 1 program simple info, got %d", len(resp.ProgramGroupVo.ProgramSimpleInfoVoList))
	}
	if resp.ProgramGroupVo.ProgramSimpleInfoVoList[0].ProgramId != 10001 || resp.ProgramGroupVo.ProgramSimpleInfoVoList[0].AreaIdName != "北京" {
		t.Fatalf("unexpected program simple info: %+v", resp.ProgramGroupVo.ProgramSimpleInfoVoList[0])
	}
	if len(resp.TicketCategoryVoList) != 2 {
		t.Fatalf("expected 2 ticket categories, got %d", len(resp.TicketCategoryVoList))
	}
	if resp.TicketCategoryVoList[0].Id != 40001 || resp.TicketCategoryVoList[0].Price != 299 {
		t.Fatalf("unexpected first ticket category: %+v", resp.TicketCategoryVoList[0])
	}
}

func TestListTicketCategoriesByProgramReturnsRemainNumbers(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	l := logicpkg.NewListTicketCategoriesByProgramLogic(context.Background(), svcCtx)
	resp, err := l.ListTicketCategoriesByProgram(&pb.ListTicketCategoriesByProgramReq{ProgramId: 10001})
	if err != nil {
		t.Fatalf("ListTicketCategoriesByProgram returned error: %v", err)
	}
	if len(resp.List) != 2 {
		t.Fatalf("expected 2 ticket categories, got %d", len(resp.List))
	}
	if resp.List[0].ProgramId != 10001 || resp.List[0].Introduce != "普通票" || resp.List[0].Price != 299 || resp.List[0].RemainNumber != 100 {
		t.Fatalf("unexpected first ticket category detail: %+v", resp.List[0])
	}
	if resp.List[1].Introduce != "VIP票" || resp.List[1].Price != 599 || resp.List[1].RemainNumber != 80 {
		t.Fatalf("unexpected second ticket category detail: %+v", resp.List[1])
	}
}

func TestGetProgramPreorderReturnsLiveRemainNumbersFromSeats(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	clearSeatInventoryByProgram(t, svcCtx, 10001)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 91001, ProgramID: 10001, TicketCategoryID: 40001, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 91002, ProgramID: 10001, TicketCategoryID: 40001, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 91003, ProgramID: 10001, TicketCategoryID: 40001, RowCode: 1, ColCode: 3, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 92001, ProgramID: 10001, TicketCategoryID: 40002, RowCode: 2, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 92002, ProgramID: 10001, TicketCategoryID: 40002, RowCode: 2, ColCode: 2, SeatStatus: testSeatStatusAvailable},
	)

	l := logicpkg.NewGetProgramPreorderLogic(context.Background(), svcCtx)
	resp, err := l.GetProgramPreorder(&pb.GetProgramDetailReq{Id: 10001})
	if err != nil {
		t.Fatalf("GetProgramPreorder returned error: %v", err)
	}
	if resp.Id != 10001 || resp.ProgramGroupId != 20001 || resp.Title != "Phase1 示例演出" {
		t.Fatalf("unexpected preorder base fields: %+v", resp)
	}
	if resp.ShowTime != "2026-12-31 19:30:00" || resp.PermitChooseSeat != 0 {
		t.Fatalf("unexpected preorder show or choose seat fields: %+v", resp)
	}
	if len(resp.TicketCategoryVoList) != 2 {
		t.Fatalf("expected 2 preorder ticket categories, got %d", len(resp.TicketCategoryVoList))
	}

	remainByCategory := map[int64]int64{}
	for _, item := range resp.TicketCategoryVoList {
		remainByCategory[item.Id] = item.RemainNumber
	}
	if remainByCategory[40001] != 3 || remainByCategory[40002] != 2 {
		t.Fatalf("expected live remain counts from d_seat, got %+v", remainByCategory)
	}
}

func TestResetProgramDomainStateSeedsCheckoutInventory(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	l := logicpkg.NewGetProgramPreorderLogic(context.Background(), svcCtx)
	resp, err := l.GetProgramPreorder(&pb.GetProgramDetailReq{Id: 10001})
	if err != nil {
		t.Fatalf("GetProgramPreorder returned error: %v", err)
	}

	remainByCategory := map[int64]int64{}
	for _, item := range resp.TicketCategoryVoList {
		remainByCategory[item.Id] = item.RemainNumber
	}
	if remainByCategory[40001] < 2 || remainByCategory[40002] < 2 {
		t.Fatalf("expected checkout seed inventory for program 10001, got %+v", remainByCategory)
	}
}

func TestGetProgramPreorderExcludesFrozenAndSoldSeats(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	clearSeatInventoryByProgram(t, svcCtx, 10001)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 93001, ProgramID: 10001, TicketCategoryID: 40001, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 93002, ProgramID: 10001, TicketCategoryID: 40001, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusFrozen, FreezeToken: "freeze-preorder-001", FreezeExpireTime: "2026-12-31 18:00:00"},
		seatFixture{ID: 93003, ProgramID: 10001, TicketCategoryID: 40001, RowCode: 1, ColCode: 3, SeatStatus: 3},
		seatFixture{ID: 94001, ProgramID: 10001, TicketCategoryID: 40002, RowCode: 2, ColCode: 1, SeatStatus: testSeatStatusAvailable},
	)

	l := logicpkg.NewGetProgramPreorderLogic(context.Background(), svcCtx)
	resp, err := l.GetProgramPreorder(&pb.GetProgramDetailReq{Id: 10001})
	if err != nil {
		t.Fatalf("GetProgramPreorder returned error: %v", err)
	}

	remainByCategory := map[int64]int64{}
	for _, item := range resp.TicketCategoryVoList {
		remainByCategory[item.Id] = item.RemainNumber
	}
	if remainByCategory[40001] != 1 || remainByCategory[40002] != 1 {
		t.Fatalf("expected frozen and sold seats to be excluded, got %+v", remainByCategory)
	}
}

func TestGetProgramPreorderReturnsNotFoundWhenProgramMissing(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	l := logicpkg.NewGetProgramPreorderLogic(context.Background(), svcCtx)
	_, err := l.GetProgramPreorder(&pb.GetProgramDetailReq{Id: 99999})
	if err == nil {
		t.Fatalf("expected not found error")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected not found code, got %s", status.Code(err))
	}
}

func TestGetProgramPreorderReturnsEmptyTicketCategoryListWhenNoneSeeded(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	seedSeatInventoryProgram(t, svcCtx, 51011, 61011)

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()
	mustExecProgramSQL(t, db, "DELETE FROM d_ticket_category WHERE program_id = ?", 51011)

	l := logicpkg.NewGetProgramPreorderLogic(context.Background(), svcCtx)
	resp, err := l.GetProgramPreorder(&pb.GetProgramDetailReq{Id: 51011})
	if err != nil {
		t.Fatalf("GetProgramPreorder returned error: %v", err)
	}
	if len(resp.TicketCategoryVoList) != 0 {
		t.Fatalf("expected empty ticket category list, got %+v", resp.TicketCategoryVoList)
	}
}

func TestGetProgramDetailReturnsNotFoundWhenProgramMissing(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	l := logicpkg.NewGetProgramDetailLogic(context.Background(), svcCtx)
	_, err := l.GetProgramDetail(&pb.GetProgramDetailReq{Id: 99999})
	if err == nil {
		t.Fatalf("expected not found error")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected not found code, got %s", status.Code(err))
	}
}

func TestPageProgramsRejectsMissingDateRangeForCustomTimeType(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	l := logicpkg.NewPageProgramsLogic(context.Background(), svcCtx)
	_, err := l.PagePrograms(&pb.PageProgramsReq{
		PageNumber:    1,
		PageSize:      10,
		TimeType:      5,
		StartDateTime: "2026-12-01 00:00:00",
		Type:          3,
	})
	if err == nil {
		t.Fatalf("expected invalid argument error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument code, got %s", status.Code(err))
	}
}
