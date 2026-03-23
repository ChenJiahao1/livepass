package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/program-rpc/internal/logic"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"
)

func TestCreateProgramPersistsProgramRecord(t *testing.T) {
	const programGroupID int64 = 20001

	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	ctx := context.Background()
	if _, err := svcCtx.DProgramGroupModel.FindOne(ctx, programGroupID); err != nil {
		t.Fatalf("prime DProgramGroupModel.FindOne returned error: %v", err)
	}

	l := logicpkg.NewCreateProgramLogic(ctx, svcCtx)
	resp, err := l.CreateProgram(&pb.CreateProgramReq{
		ProgramGroupId:                  programGroupID,
		AreaId:                          2,
		ProgramCategoryId:               11,
		ParentProgramCategoryId:         1,
		Title:                           "新增节目写链路测试",
		Actor:                           "测试艺人",
		Place:                           "上海测试剧场",
		ItemPicture:                     "https://example.com/program-create.jpg",
		Detail:                          "<p>create detail</p>",
		PerOrderLimitPurchaseCount:      4,
		PerAccountLimitPurchaseCount:    6,
		PermitRefund:                    1,
		RefundTicketRule:                "演出开始前 48 小时可退",
		RefundExplain:                   "请按规则退票",
		PermitChooseSeat:                0,
		ElectronicDeliveryTicket:        1,
		ElectronicDeliveryTicketExplain: "电子票入场",
		ElectronicInvoice:               1,
		ElectronicInvoiceExplain:        "邮件发送电子发票",
		HighHeat:                        1,
		ProgramStatus:                   1,
		IssueTime:                       "2026-10-01 10:00:00",
	})
	if err != nil {
		t.Fatalf("CreateProgram returned error: %v", err)
	}
	if resp.GetId() <= 0 {
		t.Fatalf("expected CreateProgram to return generated id, got %+v", resp)
	}

	assertProgramRelatedCacheKeysMissing(t, svcCtx, resp.GetId(), programGroupID)

	program, err := svcCtx.DProgramModel.FindOne(ctx, resp.GetId())
	if err != nil {
		t.Fatalf("DProgramModel.FindOne returned error: %v", err)
	}
	if program.ProgramGroupId != programGroupID {
		t.Fatalf("expected program_group_id=%d, got %+v", programGroupID, program)
	}
	if program.Title != "新增节目写链路测试" {
		t.Fatalf("expected title to be persisted, got %+v", program)
	}
	if program.PerOrderLimitPurchaseCount != 4 || program.PerAccountLimitPurchaseCount != 6 {
		t.Fatalf("expected purchase limits to be persisted, got %+v", program)
	}
	if !program.IssueTime.Valid || program.IssueTime.Time.Format("2006-01-02 15:04:05") != "2026-10-01 10:00:00" {
		t.Fatalf("expected issue time to be persisted, got %+v", program.IssueTime)
	}
}

func TestUpdateProgramRefreshesDetailCacheAndInvalidatesGroupKeys(t *testing.T) {
	const (
		programID       int64 = 10001
		oldProgramGroup int64 = 20001
		newProgramGroup int64 = 20002
	)

	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})
	seedProgramGroupFixture(t, svcCtx, newProgramGroup, `[{"programId":10001,"areaId":2,"areaIdName":"上海"}]`, "2026-12-31 19:30:00")

	ctx := context.Background()
	detailLogic := logicpkg.NewGetProgramDetailLogic(ctx, svcCtx)
	initial, err := detailLogic.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID})
	if err != nil {
		t.Fatalf("GetProgramDetail returned error: %v", err)
	}
	if initial.GetTitle() != "Phase1 示例演出" {
		t.Fatalf("unexpected initial detail payload: %+v", initial)
	}

	if _, err := svcCtx.DProgramGroupModel.FindOne(ctx, oldProgramGroup); err != nil {
		t.Fatalf("prime old group cache error: %v", err)
	}
	if _, err := svcCtx.DProgramGroupModel.FindOne(ctx, newProgramGroup); err != nil {
		t.Fatalf("prime new group cache error: %v", err)
	}
	if _, err := svcCtx.DProgramModel.FindOne(ctx, programID); err != nil {
		t.Fatalf("prime program cache error: %v", err)
	}
	if _, err := svcCtx.DProgramShowTimeModel.FindFirstByProgramId(ctx, programID); err != nil {
		t.Fatalf("prime first show time cache error: %v", err)
	}

	l := logicpkg.NewUpdateProgramLogic(ctx, svcCtx)
	resp, err := l.UpdateProgram(&pb.UpdateProgramReq{
		Id:                              programID,
		ProgramGroupId:                  newProgramGroup,
		Prime:                           1,
		AreaId:                          2,
		ProgramCategoryId:               11,
		ParentProgramCategoryId:         1,
		Title:                           "更新后的节目标题",
		Actor:                           "更新艺人",
		Place:                           "上海大剧院",
		ItemPicture:                     "https://example.com/program-updated.jpg",
		PreSell:                         0,
		PreSellInstruction:              "",
		ImportantNotice:                 "更新后的注意事项",
		Detail:                          "<p>updated detail</p>",
		PerOrderLimitPurchaseCount:      8,
		PerAccountLimitPurchaseCount:    10,
		RefundTicketRule:                "演出开始前 72 小时可退",
		DeliveryInstruction:             "现场取票",
		EntryRule:                       "凭证件入场",
		ChildPurchase:                   "儿童需持票",
		InvoiceSpecification:            "演出后统一开票",
		RealTicketPurchaseRule:          "一个订单一个证件",
		AbnormalOrderDescription:        "异常订单将人工复核",
		KindReminder:                    "请提前到场",
		PerformanceDuration:             "约150分钟",
		EntryTime:                       "提前45分钟入场",
		MinPerformanceCount:             18,
		MainActor:                       "更新主演",
		MinPerformanceDuration:          "约150分钟",
		ProhibitedItem:                  "禁止携带专业摄像设备",
		DepositSpecification:            "可寄存大件行李",
		TotalCount:                      1200,
		PermitRefund:                    2,
		RefundExplain:                   "满足条件可全额退",
		RefundRuleJson:                  `{"version":2}`,
		RelNameTicketEntrance:           1,
		RelNameTicketEntranceExplain:    "需实名入场",
		PermitChooseSeat:                0,
		ChooseSeatExplain:               "系统自动连座",
		ElectronicDeliveryTicket:        1,
		ElectronicDeliveryTicketExplain: "电子票扫码入场",
		ElectronicInvoice:               0,
		ElectronicInvoiceExplain:        "",
		HighHeat:                        0,
		ProgramStatus:                   1,
		IssueTime:                       "2026-09-01 09:00:00",
		Status:                          1,
	})
	if err != nil {
		t.Fatalf("UpdateProgram returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected UpdateProgram to return success, got %+v", resp)
	}

	assertProgramRelatedCacheKeysMissing(t, svcCtx, programID, oldProgramGroup)
	assertProgramRelatedCacheKeysMissing(t, svcCtx, programID, newProgramGroup)

	program, err := svcCtx.DProgramModel.FindOne(ctx, programID)
	if err != nil {
		t.Fatalf("DProgramModel.FindOne returned error: %v", err)
	}
	if program.ProgramGroupId != newProgramGroup {
		t.Fatalf("expected program_group_id=%d, got %+v", newProgramGroup, program)
	}
	if program.Title != "更新后的节目标题" {
		t.Fatalf("expected title to be updated, got %+v", program)
	}

	updatedDetail, err := detailLogic.GetProgramDetail(&pb.GetProgramDetailReq{Id: programID})
	if err != nil {
		t.Fatalf("GetProgramDetail after update returned error: %v", err)
	}
	if updatedDetail.GetTitle() != "更新后的节目标题" {
		t.Fatalf("expected detail cache to be refreshed, got %+v", updatedDetail)
	}
	if updatedDetail.GetProgramGroupId() != newProgramGroup {
		t.Fatalf("expected updated detail to reflect new group id, got %+v", updatedDetail)
	}
}

func seedProgramGroupFixture(t *testing.T, svcCtx *svc.ServiceContext, groupID int64, programJSON string, recentShowTime string) {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	mustExecProgramSQL(
		t,
		db,
		`INSERT INTO d_program_group (id, program_json, recent_show_time, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?)`,
		groupID,
		programJSON,
		recentShowTime,
		"2026-01-01 00:00:00",
		"2026-01-01 00:00:00",
		1,
	)
}
