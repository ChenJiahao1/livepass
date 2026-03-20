package logic

import (
	"context"
	"testing"

	"damai-go/services/program-api/internal/svc"
	"damai-go/services/program-api/internal/types"
	"damai-go/services/program-rpc/programrpc"
)

func TestListProgramCategoriesMapsResponse(t *testing.T) {
	fake := &fakeProgramRPC{
		listProgramCategoriesResp: &programrpc.ProgramCategoryListResp{
			List: []*programrpc.ProgramCategoryInfo{
				{Id: 1, ParentId: 0, Name: "演唱会", Type: 1},
				{Id: 11, ParentId: 1, Name: "livehouse", Type: 2},
			},
		},
	}
	logic := NewListProgramCategoriesLogic(context.Background(), &svc.ServiceContext{ProgramRpc: fake})

	resp, err := logic.ListProgramCategories(&types.EmptyReq{})
	if err != nil {
		t.Fatalf("ListProgramCategories returned error: %v", err)
	}
	if len(resp.List) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(resp.List))
	}
	if resp.List[0].ID != 1 || resp.List[1].Name != "livehouse" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastListProgramCategoriesReq == nil {
		t.Fatalf("expected rpc request")
	}
}

func TestListHomeProgramsMapsResponse(t *testing.T) {
	fake := &fakeProgramRPC{
		listHomeProgramsResp: &programrpc.ProgramHomeListResp{
			Sections: []*programrpc.ProgramHomeSection{
				{
					CategoryName: "演唱会",
					CategoryId:   1,
					ProgramListVoList: []*programrpc.ProgramListInfo{
						{
							Id:       10001,
							Title:    "Phase1 示例演出",
							MinPrice: 299,
							MaxPrice: 599,
						},
					},
				},
			},
		},
	}
	logic := NewListHomeProgramsLogic(context.Background(), &svc.ServiceContext{ProgramRpc: fake})

	resp, err := logic.ListHomePrograms(&types.ListHomeProgramsReq{ParentProgramCategoryIds: []int64{1, 2}})
	if err != nil {
		t.Fatalf("ListHomePrograms returned error: %v", err)
	}
	if len(resp.Sections) != 1 || resp.Sections[0].CategoryID != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(resp.Sections[0].ProgramListVoList) != 1 || resp.Sections[0].ProgramListVoList[0].Title != "Phase1 示例演出" {
		t.Fatalf("unexpected section programs: %+v", resp.Sections[0].ProgramListVoList)
	}
	if fake.lastListHomeProgramsReq == nil || len(fake.lastListHomeProgramsReq.ParentProgramCategoryIds) != 2 {
		t.Fatalf("unexpected request: %+v", fake.lastListHomeProgramsReq)
	}
}

func TestPageProgramsMapsRequestAndPaginationResponse(t *testing.T) {
	fake := &fakeProgramRPC{
		pageProgramsResp: &programrpc.ProgramPageResp{
			PageNum:   1,
			PageSize:  10,
			TotalSize: 2,
			List: []*programrpc.ProgramListInfo{
				{
					Id:       10003,
					Title:    "十二月早场",
					ShowTime: "2026-12-10 19:30:00",
					MinPrice: 180,
					MaxPrice: 380,
					EsId:     9001,
				},
			},
		},
	}
	logic := NewPageProgramsLogic(context.Background(), &svc.ServiceContext{ProgramRpc: fake})

	resp, err := logic.PagePrograms(&types.PageProgramsReq{
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
		t.Fatalf("unexpected page response: %+v", resp)
	}
	if len(resp.List) != 1 || resp.List[0].EsID != 9001 {
		t.Fatalf("unexpected list response: %+v", resp.List)
	}
	if fake.lastPageProgramsReq == nil || fake.lastPageProgramsReq.ProgramCategoryId != 11 || fake.lastPageProgramsReq.TimeType != 5 {
		t.Fatalf("unexpected request: %+v", fake.lastPageProgramsReq)
	}
}

func TestGetProgramDetailMapsNestedResponse(t *testing.T) {
	fake := &fakeProgramRPC{
		getProgramDetailResp: &programrpc.ProgramDetailInfo{
			Id:                        10001,
			ProgramGroupId:            20001,
			Title:                     "Phase1 示例演出",
			ProgramCategoryName:       "livehouse",
			ParentProgramCategoryName: "演唱会",
			ProgramGroupVo: &programrpc.ProgramGroupInfo{
				Id:             20001,
				RecentShowTime: "2026-12-31 19:30:00",
				ProgramSimpleInfoVoList: []*programrpc.ProgramSimpleInfo{
					{ProgramId: 10001, AreaId: 2, AreaIdName: "北京"},
				},
			},
			TicketCategoryVoList: []*programrpc.TicketCategoryInfo{
				{Id: 40001, Introduce: "普通票", Price: 299},
			},
		},
	}
	logic := NewGetProgramDetailLogic(context.Background(), &svc.ServiceContext{ProgramRpc: fake})

	resp, err := logic.GetProgramDetail(&types.GetProgramDetailReq{ID: 10001})
	if err != nil {
		t.Fatalf("GetProgramDetail returned error: %v", err)
	}
	if resp.ID != 10001 || resp.ProgramGroupID != 20001 || resp.ProgramGroupVo.ID != 20001 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(resp.ProgramGroupVo.ProgramSimpleInfoVoList) != 1 || resp.ProgramGroupVo.ProgramSimpleInfoVoList[0].AreaIDName != "北京" {
		t.Fatalf("unexpected program group response: %+v", resp.ProgramGroupVo)
	}
	if len(resp.TicketCategoryVoList) != 1 || resp.TicketCategoryVoList[0].ID != 40001 {
		t.Fatalf("unexpected ticket category response: %+v", resp.TicketCategoryVoList)
	}
	if fake.lastGetProgramDetailReq == nil || fake.lastGetProgramDetailReq.Id != 10001 {
		t.Fatalf("unexpected request: %+v", fake.lastGetProgramDetailReq)
	}
}

func TestListTicketCategoriesByProgramMapsResponse(t *testing.T) {
	fake := &fakeProgramRPC{
		listTicketCategoriesByProgramResp: &programrpc.TicketCategoryDetailListResp{
			List: []*programrpc.TicketCategoryDetailInfo{
				{ProgramId: 10001, Introduce: "普通票", Price: 299, TotalNumber: 100, RemainNumber: 80},
				{ProgramId: 10001, Introduce: "VIP票", Price: 599, TotalNumber: 50, RemainNumber: 30},
			},
		},
	}
	logic := NewListTicketCategoriesByProgramLogic(context.Background(), &svc.ServiceContext{ProgramRpc: fake})

	resp, err := logic.ListTicketCategoriesByProgram(&types.ListTicketCategoriesByProgramReq{ProgramID: 10001})
	if err != nil {
		t.Fatalf("ListTicketCategoriesByProgram returned error: %v", err)
	}
	if len(resp.List) != 2 || resp.List[0].RemainNumber != 80 || resp.List[1].Price != 599 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastListTicketCategoriesByProgramReq == nil || fake.lastListTicketCategoriesByProgramReq.ProgramId != 10001 {
		t.Fatalf("unexpected request: %+v", fake.lastListTicketCategoriesByProgramReq)
	}
}
