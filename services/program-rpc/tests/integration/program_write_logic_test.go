package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"livepass/jobs/rush-inventory-preheat/taskdef"
	"livepass/pkg/xid"
	logicpkg "livepass/services/program-rpc/internal/logic"
	"livepass/services/program-rpc/internal/programcache"
	"livepass/services/program-rpc/internal/svc"
	"livepass/services/program-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeRushInventoryPreheatClient struct {
	enqueueErr           error
	enqueueCalls         int
	lastProgramID        int64
	lastExpectedOpenTime time.Time
}

func (f *fakeRushInventoryPreheatClient) Enqueue(_ context.Context, programID int64, expectedOpenTime time.Time) error {
	f.enqueueCalls++
	f.lastProgramID = programID
	f.lastExpectedOpenTime = expectedOpenTime
	return f.enqueueErr
}

func markProgramInventoryPreheated(t *testing.T, svcCtx *svc.ServiceContext, programID int64) {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	mustExecProgramSQL(
		t,
		db,
		"UPDATE d_program SET inventory_preheat_status = 2, edit_time = ? WHERE id = ?",
		time.Now().Format(testProgramDateTimeLayout),
		programID,
	)
}

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
		RushSaleOpenTime:                "2026-10-01 18:00:00",
		RushSaleEndTime:                 "2026-10-01 19:00:00",
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
	if !program.RushSaleOpenTime.Valid || program.RushSaleOpenTime.Time.Format(testProgramDateTimeLayout) != "2026-10-01 18:00:00" {
		t.Fatalf("expected rush_sale_open_time to be persisted on program, got %+v", program.RushSaleOpenTime)
	}
	if !program.RushSaleEndTime.Valid || program.RushSaleEndTime.Time.Format(testProgramDateTimeLayout) != "2026-10-01 19:00:00" {
		t.Fatalf("expected rush_sale_end_time to be persisted on program, got %+v", program.RushSaleEndTime)
	}
	if program.InventoryPreheatStatus != 0 {
		t.Fatalf("expected program inventory_preheat_status default 0, got %+v", program)
	}
}

func TestCreateProgramReturnsSuccessWhenCacheInvalidationFails(t *testing.T) {
	const programGroupID int64 = 20001

	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})
	replaceProgramCacheInvalidatorWithFailingRedis(t, svcCtx)

	ctx := context.Background()
	l := logicpkg.NewCreateProgramLogic(ctx, svcCtx)
	resp, err := l.CreateProgram(&pb.CreateProgramReq{
		ProgramGroupId:                  programGroupID,
		AreaId:                          2,
		ProgramCategoryId:               11,
		ParentProgramCategoryId:         1,
		Title:                           "缓存失效失败后仍成功创建",
		Actor:                           "测试艺人",
		Place:                           "上海测试剧场",
		ItemPicture:                     "https://example.com/program-create-cache-fail.jpg",
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
		RushSaleOpenTime:                "2026-10-01 18:00:00",
		RushSaleEndTime:                 "2026-10-01 19:00:00",
	})
	if err != nil {
		t.Fatalf("CreateProgram returned error: %v", err)
	}
	if resp.GetId() <= 0 {
		t.Fatalf("expected CreateProgram to return generated id, got %+v", resp)
	}

	program, err := svcCtx.DProgramModel.FindOne(ctx, resp.GetId())
	if err != nil {
		t.Fatalf("DProgramModel.FindOne returned error: %v", err)
	}
	if program.ProgramGroupId != programGroupID {
		t.Fatalf("expected program_group_id=%d, got %+v", programGroupID, program)
	}
	if program.Title != "缓存失效失败后仍成功创建" {
		t.Fatalf("expected title to be persisted, got %+v", program)
	}
}

func TestInvalidProgramMarksProgramOffShelfAndInvalidatesCache(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	ctx := context.Background()
	detailLogic := logicpkg.NewGetProgramDetailViewLogic(ctx, svcCtx)
	if _, err := detailLogic.GetProgramDetailView(&pb.GetProgramDetailViewReq{Id: 10001}); err != nil {
		t.Fatalf("prime detail cache error: %v", err)
	}

	l := logicpkg.NewInvalidProgramLogic(ctx, svcCtx)
	resp, err := l.InvalidProgram(&pb.ProgramInvalidReq{Id: 10001})
	if err != nil {
		t.Fatalf("InvalidProgram returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success, got %+v", resp)
	}
	assertProgramRelatedCacheKeysMissing(t, svcCtx, 10001, 20001)

	program, err := svcCtx.DProgramModel.FindOne(ctx, 10001)
	if err != nil {
		t.Fatalf("find program error: %v", err)
	}
	if program.ProgramStatus != 0 {
		t.Fatalf("expected program off shelf, got %+v", program)
	}
}

func TestResetProgramRestoresSeatStatusAndRemainNumber(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	ctx := context.Background()
	clearSeatInventoryByProgram(t, svcCtx, 10001)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 91001, ProgramID: 10001, TicketCategoryID: 40001, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusFrozen, FreezeToken: "freeze-1", FreezeExpireTime: "2026-12-31 18:00:00"},
		seatFixture{ID: 91002, ProgramID: 10001, TicketCategoryID: 40001, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusSold},
	)
	updateTicketCategoryRemainNumber(t, svcCtx, 40001, 2)

	l := logicpkg.NewResetProgramLogic(ctx, svcCtx)
	resp, err := l.ResetProgram(&pb.ProgramResetReq{ProgramId: 10001})
	if err != nil {
		t.Fatalf("ResetProgram returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success, got %+v", resp)
	}
}

func TestBatchCreateProgramCategoriesRejectsDuplicateEntries(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	l := logicpkg.NewBatchCreateProgramCategoriesLogic(context.Background(), svcCtx)
	_, err := l.BatchCreateProgramCategories(&pb.ProgramCategoryBatchSaveReq{
		List: []*pb.ProgramCategoryBatchItem{
			{ParentId: 1, Name: "livehouse", Type: 2},
		},
	})
	if err == nil {
		t.Fatalf("expected duplicate category error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument code, got %s", status.Code(err))
	}
}

func TestBatchCreateProgramCategoriesReturnsSuccessWhenBroadcastFails(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	if svcCtx.ProgramCacheInvalidator == nil {
		t.Fatalf("expected ProgramCacheInvalidator to be configured")
	}
	svcCtx.ProgramCacheInvalidator.SetPublisher(failingInvalidationPublisher{})

	nameSuffix := xid.New()
	l := logicpkg.NewBatchCreateProgramCategoriesLogic(context.Background(), svcCtx)
	resp, err := l.BatchCreateProgramCategories(&pb.ProgramCategoryBatchSaveReq{
		List: []*pb.ProgramCategoryBatchItem{
			{ParentId: 0, Name: fmt.Sprintf("broadcast-fail-%d", nameSuffix), Type: 1},
			{ParentId: 0, Name: fmt.Sprintf("broadcast-fail-child-%d", nameSuffix), Type: 2},
		},
	})
	if err != nil {
		t.Fatalf("BatchCreateProgramCategories returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success, got %+v", resp)
	}
}

func TestBatchCreateProgramCategoriesReturnsSuccessWhenInvalidatorMissing(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	svcCtx.ProgramCacheInvalidator = nil

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("expected no panic, got %v", recovered)
		}
	}()

	nameSuffix := xid.New()
	l := logicpkg.NewBatchCreateProgramCategoriesLogic(context.Background(), svcCtx)
	resp, err := l.BatchCreateProgramCategories(&pb.ProgramCategoryBatchSaveReq{
		List: []*pb.ProgramCategoryBatchItem{
			{ParentId: 0, Name: fmt.Sprintf("invalidator-missing-%d", nameSuffix), Type: 1},
			{ParentId: 0, Name: fmt.Sprintf("invalidator-missing-child-%d", nameSuffix), Type: 2},
		},
	})
	if err != nil {
		t.Fatalf("BatchCreateProgramCategories returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success, got %+v", resp)
	}
}

func TestCreateProgramShowTimePersistsRecordAndRefreshesGroupRecentShowTime(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	ctx := context.Background()
	l := logicpkg.NewCreateProgramShowTimeLogic(context.Background(), svcCtx)
	resp, err := l.CreateProgramShowTime(&pb.ProgramShowTimeAddReq{
		ProgramId:    10001,
		ShowTime:     "2026-12-31 19:15:00",
		ShowDayTime:  "2026-12-31 00:00:00",
		ShowWeekTime: "周四",
		ShowEndTime:  "2026-12-31 22:15:00",
	})
	if err != nil {
		t.Fatalf("CreateProgramShowTime returned error: %v", err)
	}
	if resp.GetId() <= 0 {
		t.Fatalf("expected generated id, got %+v", resp)
	}

	showTime, err := svcCtx.DProgramShowTimeModel.FindOne(ctx, resp.GetId())
	if err != nil {
		t.Fatalf("DProgramShowTimeModel.FindOne returned error: %v", err)
	}
	if !showTime.ShowEndTime.Valid || showTime.ShowEndTime.Time.Format(testProgramDateTimeLayout) != "2026-12-31 22:15:00" {
		t.Fatalf("expected show_end_time to be persisted, got %+v", showTime.ShowEndTime)
	}
}

func TestUpdateProgramShowTimePersistsSaleConfigChanges(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	ctx := context.Background()
	l := logicpkg.NewUpdateProgramShowTimeLogic(ctx, svcCtx)
	resp, err := l.UpdateProgramShowTime(&pb.UpdateProgramShowTimeReq{
		Id:           30001,
		ShowWeekTime: "周五",
		ShowEndTime:  "2026-12-31 22:30:00",
	})
	if err != nil {
		t.Fatalf("UpdateProgramShowTime returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success, got %+v", resp)
	}

	showTime, err := svcCtx.DProgramShowTimeModel.FindOne(ctx, 30001)
	if err != nil {
		t.Fatalf("DProgramShowTimeModel.FindOne returned error: %v", err)
	}
	if showTime.ShowWeekTime != "周五" {
		t.Fatalf("expected show_week_time updated, got %+v", showTime)
	}
	if !showTime.ShowEndTime.Valid || showTime.ShowEndTime.Time.Format(testProgramDateTimeLayout) != "2026-12-31 22:30:00" {
		t.Fatalf("expected show_end_time updated, got %+v", showTime.ShowEndTime)
	}
}

func TestCreateProgramShowTimeRejectsShowTimeBeforeRushSaleOpenTime(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	l := logicpkg.NewCreateProgramShowTimeLogic(context.Background(), svcCtx)
	_, err := l.CreateProgramShowTime(&pb.ProgramShowTimeAddReq{
		ProgramId:    10001,
		ShowTime:     "2026-12-31 17:30:00",
		ShowDayTime:  "2026-12-31 00:00:00",
		ShowWeekTime: "周四",
		ShowEndTime:  "2026-12-31 22:30:00",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument when show time is earlier than rush sale open time, got %v", err)
	}
}

func TestUpdateProgramShowTimeRejectsWhenProgramRushSaleOpenTimeIsAfterShowTime(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()
	mustExecProgramSQL(
		t,
		db,
		"UPDATE d_program SET rush_sale_open_time = ?, edit_time = ? WHERE id = ?",
		"2026-12-31 20:00:00",
		time.Now().Format(testProgramDateTimeLayout),
		10001,
	)

	l := logicpkg.NewUpdateProgramShowTimeLogic(context.Background(), svcCtx)
	_, err := l.UpdateProgramShowTime(&pb.UpdateProgramShowTimeReq{
		Id:           30001,
		ShowWeekTime: "周五",
		ShowEndTime:  "2026-12-31 22:30:00",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument when show time is earlier than program rush sale open time, got %v", err)
	}
}

func TestCreateProgramShowTimeEnqueuesRushInventoryPreheatAndMarksStatusScheduled(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	fakeClient := &fakeRushInventoryPreheatClient{}
	svcCtx.RushInventoryPreheatClient = fakeClient

	ctx := context.Background()
	l := logicpkg.NewCreateProgramShowTimeLogic(ctx, svcCtx)
	_, err := l.CreateProgramShowTime(&pb.ProgramShowTimeAddReq{
		ProgramId:    10001,
		ShowTime:     "2026-12-31 19:35:00",
		ShowDayTime:  "2026-12-31 00:00:00",
		ShowWeekTime: "周四",
		ShowEndTime:  "2026-12-31 22:35:00",
	})
	if err != nil {
		t.Fatalf("CreateProgramShowTime returned error: %v", err)
	}

	if fakeClient.enqueueCalls != 1 {
		t.Fatalf("expected enqueue once, got %d", fakeClient.enqueueCalls)
	}
	if fakeClient.lastProgramID != 10001 {
		t.Fatalf("expected enqueue programId 10001, got %d", fakeClient.lastProgramID)
	}
	if fakeClient.lastExpectedOpenTime.Format(testProgramDateTimeLayout) != "2026-12-31 18:00:00" {
		t.Fatalf("expected expected open time 2026-12-31 18:00:00, got %s", fakeClient.lastExpectedOpenTime.Format(testProgramDateTimeLayout))
	}

	program, err := svcCtx.DProgramModel.FindOne(ctx, 10001)
	if err != nil {
		t.Fatalf("DProgramModel.FindOne returned error: %v", err)
	}
	if program.InventoryPreheatStatus != 1 {
		t.Fatalf("expected program inventory_preheat_status 1 after enqueue, got %+v", program)
	}
}

func TestUpdateProgramShowTimeDoesNotEnqueueRushInventoryPreheat(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	fakeClient := &fakeRushInventoryPreheatClient{}
	svcCtx.RushInventoryPreheatClient = fakeClient

	ctx := context.Background()
	l := logicpkg.NewUpdateProgramShowTimeLogic(ctx, svcCtx)
	resp, err := l.UpdateProgramShowTime(&pb.UpdateProgramShowTimeReq{
		Id:           30001,
		ShowWeekTime: "周五",
		ShowEndTime:  "2026-12-31 22:30:00",
	})
	if err != nil {
		t.Fatalf("UpdateProgramShowTime returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success, got %+v", resp)
	}
	if fakeClient.enqueueCalls != 0 {
		t.Fatalf("expected no enqueue when updating show time metadata, got %d", fakeClient.enqueueCalls)
	}

	program, err := svcCtx.DProgramModel.FindOne(ctx, 10001)
	if err != nil {
		t.Fatalf("DProgramModel.FindOne returned error: %v", err)
	}
	if program.InventoryPreheatStatus != 0 {
		t.Fatalf("expected program inventory_preheat_status to remain unchanged, got %+v", program)
	}
}

func TestCreateProgramShowTimeWritesRushInventoryPreheatDelayTaskOutbox(t *testing.T) {
	svcCtx := newProgramTestServiceContextWithRushInventoryPreheat(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	ctx := context.Background()
	l := logicpkg.NewCreateProgramShowTimeLogic(ctx, svcCtx)
	_, err := l.CreateProgramShowTime(&pb.ProgramShowTimeAddReq{
		ProgramId:    10001,
		ShowTime:     "2026-12-31 19:35:00",
		ShowDayTime:  "2026-12-31 00:00:00",
		ShowWeekTime: "周四",
		ShowEndTime:  "2026-12-31 22:35:00",
	})
	if err != nil {
		t.Fatalf("CreateProgramShowTime returned error: %v", err)
	}

	program, err := svcCtx.DProgramModel.FindOne(ctx, 10001)
	if err != nil {
		t.Fatalf("DProgramModel.FindOne returned error: %v", err)
	}
	if program.InventoryPreheatStatus != 1 {
		t.Fatalf("expected program inventory_preheat_status 1 after scheduling, got %+v", program)
	}

	expectedOpenTime := time.Date(2026, 12, 31, 18, 0, 0, 0, time.Local)
	taskKey := taskdef.TaskKey(10001, expectedOpenTime)
	requireProgramDelayTaskOutbox(
		t,
		svcCtx.Config.MySQL.DataSource,
		taskdef.TaskTypeRushInventoryPreheat,
		taskKey,
		"2026-12-31 17:55:00",
	)

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()
	mustExecProgramSQL(
		t,
		db,
		`UPDATE d_delay_task_outbox
		SET task_status = 3, publish_attempts = 5, consume_attempts = 2,
			last_consume_error = 'preheat failed', published_time = ?,
			processed_time = ?, edit_time = ?
		WHERE task_key = ?`,
		time.Now(),
		time.Now(),
		time.Now(),
		taskKey,
	)

	_, err = l.CreateProgramShowTime(&pb.ProgramShowTimeAddReq{
		ProgramId:    10001,
		ShowTime:     "2027-01-01 19:35:00",
		ShowDayTime:  "2027-01-01 00:00:00",
		ShowWeekTime: "周五",
		ShowEndTime:  "2027-01-01 22:35:00",
	})
	if err != nil {
		t.Fatalf("second CreateProgramShowTime returned error: %v", err)
	}
	requireProgramDelayTaskOutbox(
		t,
		svcCtx.Config.MySQL.DataSource,
		taskdef.TaskTypeRushInventoryPreheat,
		taskKey,
		"2026-12-31 17:55:00",
	)
}

func TestUpdateProgramShowTimeDoesNotWriteRushInventoryPreheatDelayTaskOutbox(t *testing.T) {
	svcCtx := newProgramTestServiceContextWithRushInventoryPreheat(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	ctx := context.Background()
	l := logicpkg.NewUpdateProgramShowTimeLogic(ctx, svcCtx)
	resp, err := l.UpdateProgramShowTime(&pb.UpdateProgramShowTimeReq{
		Id:           30001,
		ShowWeekTime: "周五",
		ShowEndTime:  "2026-12-31 22:30:00",
	})
	if err != nil {
		t.Fatalf("UpdateProgramShowTime returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success, got %+v", resp)
	}

	program, err := svcCtx.DProgramModel.FindOne(ctx, 10001)
	if err != nil {
		t.Fatalf("DProgramModel.FindOne returned error: %v", err)
	}
	if program.InventoryPreheatStatus != 0 {
		t.Fatalf("expected program inventory_preheat_status to remain unchanged, got %+v", program)
	}

	expectedOpenTime := time.Date(2026, 12, 31, 18, 0, 0, 0, time.Local)
	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM d_delay_task_outbox WHERE task_type = ? AND task_key = ?",
		taskdef.TaskTypeRushInventoryPreheat,
		taskdef.TaskKey(10001, expectedOpenTime),
	).Scan(&count); err != nil {
		t.Fatalf("query outbox count error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no outbox written for update show time, got %d", count)
	}
}

func TestCreateProgramShowTimeRollsBackWhenRushInventoryPreheatOutboxInsertFails(t *testing.T) {
	svcCtx := newProgramTestServiceContextWithRushInventoryPreheat(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	mustExecProgramSQL(t, db, "DROP TABLE d_delay_task_outbox")

	var beforeCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM d_program_show_time WHERE program_id = ? AND status = 1", 10001).Scan(&beforeCount); err != nil {
		t.Fatalf("query show time count before create error: %v", err)
	}

	l := logicpkg.NewCreateProgramShowTimeLogic(context.Background(), svcCtx)
	_, err := l.CreateProgramShowTime(&pb.ProgramShowTimeAddReq{
		ProgramId:    10001,
		ShowTime:     "2027-01-02 19:30:00",
		ShowDayTime:  "2027-01-02 00:00:00",
		ShowWeekTime: "周五",
		ShowEndTime:  "2027-01-02 22:30:00",
	})
	if err == nil {
		t.Fatalf("expected create show time to fail when outbox insert fails")
	}

	var afterCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM d_program_show_time WHERE program_id = ? AND status = 1", 10001).Scan(&afterCount); err != nil {
		t.Fatalf("query show time count after create error: %v", err)
	}
	if afterCount != beforeCount {
		t.Fatalf("expected show time count rollback, before=%d after=%d", beforeCount, afterCount)
	}

	var createdCount int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM d_program_show_time WHERE program_id = ? AND show_week_time = ? AND status = 1",
		10001,
		"周五",
	).Scan(&createdCount); err != nil {
		t.Fatalf("query created show time count error: %v", err)
	}
	if createdCount != 0 {
		t.Fatalf("expected created show time to be rolled back, got %d rows", createdCount)
	}
}

func TestUpdateProgramShowTimeSucceedsWithoutRushInventoryPreheatOutboxInsert(t *testing.T) {
	svcCtx := newProgramTestServiceContextWithRushInventoryPreheat(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	var beforePreheatStatus int64
	if err := db.QueryRow(
		`SELECT show_week_time,
		        DATE_FORMAT(show_end_time, '%Y-%m-%d %H:%i:%s')
		FROM d_program_show_time
		WHERE id = ?`,
		30001,
	).Scan(new(string), new(string)); err != nil {
		t.Fatalf("query show time before update error: %v", err)
	}
	if err := db.QueryRow("SELECT inventory_preheat_status FROM d_program WHERE id = ?", 10001).Scan(&beforePreheatStatus); err != nil {
		t.Fatalf("query program before update error: %v", err)
	}

	mustExecProgramSQL(t, db, "DROP TABLE d_delay_task_outbox")

	l := logicpkg.NewUpdateProgramShowTimeLogic(context.Background(), svcCtx)
	resp, err := l.UpdateProgramShowTime(&pb.UpdateProgramShowTimeReq{
		Id:           30001,
		ShowWeekTime: "周六",
		ShowEndTime:  "2026-12-31 22:45:00",
	})
	if err != nil {
		t.Fatalf("expected update show time to succeed without outbox write, got %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success, got %+v", resp)
	}

	var afterWeek, afterShowEndTime string
	var afterPreheatStatus int64
	if err := db.QueryRow(
		`SELECT show_week_time,
		        DATE_FORMAT(show_end_time, '%Y-%m-%d %H:%i:%s')
		FROM d_program_show_time
		WHERE id = ?`,
		30001,
	).Scan(&afterWeek, &afterShowEndTime); err != nil {
		t.Fatalf("query show time after update error: %v", err)
	}
	if err := db.QueryRow("SELECT inventory_preheat_status FROM d_program WHERE id = ?", 10001).Scan(&afterPreheatStatus); err != nil {
		t.Fatalf("query program after update error: %v", err)
	}

	if afterWeek != "周六" || afterShowEndTime != "2026-12-31 22:45:00" {
		t.Fatalf("expected update to persist without outbox, got after=(%s,%s)", afterWeek, afterShowEndTime)
	}
	if afterPreheatStatus != beforePreheatStatus {
		t.Fatalf("expected inventory_preheat_status unchanged, before=%d after=%d", beforePreheatStatus, afterPreheatStatus)
	}
}

func TestCreateTicketCategoryPersistsRecordAndInvalidatesProgramDetailCache(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	ctx := context.Background()
	detailLogic := logicpkg.NewGetProgramDetailViewLogic(ctx, svcCtx)
	if _, err := detailLogic.GetProgramDetailView(&pb.GetProgramDetailViewReq{Id: 10001}); err != nil {
		t.Fatalf("prime detail cache error: %v", err)
	}

	l := logicpkg.NewCreateTicketCategoryLogic(ctx, svcCtx)
	resp, err := l.CreateTicketCategory(&pb.TicketCategoryAddReq{
		ProgramId:    10001,
		Introduce:    "至尊票",
		Price:        999,
		TotalNumber:  20,
		RemainNumber: 20,
	})
	if err != nil {
		t.Fatalf("CreateTicketCategory returned error: %v", err)
	}
	if resp.GetId() <= 0 {
		t.Fatalf("expected generated id, got %+v", resp)
	}
}

func TestCreateTicketCategoryRejectsInventoryMutationAfterPreheat(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})
	markProgramInventoryPreheated(t, svcCtx, 10001)

	l := logicpkg.NewCreateTicketCategoryLogic(context.Background(), svcCtx)
	_, err := l.CreateTicketCategory(&pb.TicketCategoryAddReq{
		ProgramId:    10001,
		Introduce:    "预热后新增票档",
		Price:        999,
		TotalNumber:  20,
		RemainNumber: 20,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition after inventory preheat, got err=%v", err)
	}
}

func TestProgramSchemaUsesProgramDimensionForRushInventory(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	requireProgramTableColumn(t, db, "d_program", "rush_sale_open_time")
	requireProgramTableColumn(t, db, "d_program", "rush_sale_end_time")
	requireProgramTableColumn(t, db, "d_program", "inventory_preheat_status")
	requireProgramTableColumn(t, db, "d_program_show_time", "show_end_time")
	requireProgramTableColumn(t, db, "d_ticket_category", "show_time_id")
	requireProgramTableColumn(t, db, "d_seat", "show_time_id")
	requireProgramIndex(t, db, "d_seat", "uk_show_time_row_col")
}

func TestGetTicketCategoryDetailReturnsRecord(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	l := logicpkg.NewGetTicketCategoryDetailLogic(context.Background(), svcCtx)
	resp, err := l.GetTicketCategoryDetail(&pb.TicketCategoryReq{Id: 40001})
	if err != nil {
		t.Fatalf("GetTicketCategoryDetail returned error: %v", err)
	}
	if resp.ProgramId != 10001 || resp.Price != 299 || resp.RemainNumber != 100 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestCreateSeatRejectsDuplicateSeatCoordinate(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	l := logicpkg.NewCreateSeatLogic(context.Background(), svcCtx)
	_, err := l.CreateSeat(&pb.SeatAddReq{
		ProgramId:        10001,
		TicketCategoryId: 40001,
		RowCode:          1,
		ColCode:          1,
		SeatType:         1,
		Price:            299,
	})
	if err == nil {
		t.Fatalf("expected duplicate seat error")
	}
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected already exists code, got %s", status.Code(err))
	}
}

func TestCreateSeatRejectsInventoryMutationAfterPreheat(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})
	markProgramInventoryPreheated(t, svcCtx, 10001)

	l := logicpkg.NewCreateSeatLogic(context.Background(), svcCtx)
	_, err := l.CreateSeat(&pb.SeatAddReq{
		ProgramId:        10001,
		TicketCategoryId: 40001,
		RowCode:          20,
		ColCode:          1,
		SeatType:         1,
		Price:            299,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition after inventory preheat, got err=%v", err)
	}
}

func TestBatchCreateSeatsGeneratesExpectedSeatRows(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	clearSeatInventoryByProgram(t, svcCtx, 10001)
	l := logicpkg.NewBatchCreateSeatsLogic(context.Background(), svcCtx)
	resp, err := l.BatchCreateSeats(&pb.SeatBatchAddReq{
		ProgramId: 10001,
		SeatBatchRelateInfoAddDtoList: []*pb.SeatBatchRelateInfoAddReq{
			{TicketCategoryId: 40001, Price: 299, Count: 20},
		},
	})
	if err != nil {
		t.Fatalf("BatchCreateSeats returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success, got %+v", resp)
	}
}

func TestBatchCreateSeatsRejectsInventoryMutationAfterPreheat(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})
	clearSeatInventoryByProgram(t, svcCtx, 10001)
	markProgramInventoryPreheated(t, svcCtx, 10001)

	l := logicpkg.NewBatchCreateSeatsLogic(context.Background(), svcCtx)
	_, err := l.BatchCreateSeats(&pb.SeatBatchAddReq{
		ProgramId: 10001,
		SeatBatchRelateInfoAddDtoList: []*pb.SeatBatchRelateInfoAddReq{
			{TicketCategoryId: 40001, Price: 299, Count: 20},
		},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition after inventory preheat, got err=%v", err)
	}
}

func TestUpdateProgramShowTimeDoesNotRescheduleRushInventoryPreheat(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})
	markProgramInventoryPreheated(t, svcCtx, 10001)

	fakeClient := &fakeRushInventoryPreheatClient{}
	svcCtx.RushInventoryPreheatClient = fakeClient

	l := logicpkg.NewUpdateProgramShowTimeLogic(context.Background(), svcCtx)
	resp, err := l.UpdateProgramShowTime(&pb.UpdateProgramShowTimeReq{
		Id:           30001,
		ShowWeekTime: "周五",
		ShowEndTime:  "2026-12-31 22:15:00",
	})
	if err != nil {
		t.Fatalf("UpdateProgramShowTime returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success, got %+v", resp)
	}
	if fakeClient.enqueueCalls != 0 {
		t.Fatalf("expected no enqueue when only updating show time metadata, got %d", fakeClient.enqueueCalls)
	}

	program, err := svcCtx.DProgramModel.FindOne(context.Background(), 10001)
	if err != nil {
		t.Fatalf("DProgramModel.FindOne returned error: %v", err)
	}
	if program.InventoryPreheatStatus != 2 {
		t.Fatalf("expected program inventory_preheat_status to remain preheated, got %+v", program)
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
	detailLogic := logicpkg.NewGetProgramDetailViewLogic(ctx, svcCtx)
	initial, err := detailLogic.GetProgramDetailView(&pb.GetProgramDetailViewReq{Id: programID})
	if err != nil {
		t.Fatalf("GetProgramDetailView returned error: %v", err)
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
		RushSaleOpenTime:                "2026-12-31 18:00:00",
		RushSaleEndTime:                 "2026-12-31 19:00:00",
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

	updatedDetail, err := detailLogic.GetProgramDetailView(&pb.GetProgramDetailViewReq{Id: programID})
	if err != nil {
		t.Fatalf("GetProgramDetailView after update returned error: %v", err)
	}
	if updatedDetail.GetTitle() != "更新后的节目标题" {
		t.Fatalf("expected detail cache to be refreshed, got %+v", updatedDetail)
	}
	if updatedDetail.GetProgramGroupId() != newProgramGroup {
		t.Fatalf("expected updated detail to reflect new group id, got %+v", updatedDetail)
	}
	if updatedDetail.GetRushSaleOpenTime() != "2026-12-31 18:00:00" || updatedDetail.GetRushSaleEndTime() != "2026-12-31 19:00:00" {
		t.Fatalf("expected updated detail to include rush sale fields, got %+v", updatedDetail)
	}
}

func TestUpdateProgramReturnsSuccessWhenCacheInvalidationFails(t *testing.T) {
	const (
		programID      int64 = 10001
		programGroupID int64 = 20001
	)

	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})
	replaceProgramCacheInvalidatorWithFailingRedis(t, svcCtx)

	ctx := context.Background()
	l := logicpkg.NewUpdateProgramLogic(ctx, svcCtx)
	resp, err := l.UpdateProgram(&pb.UpdateProgramReq{
		Id:                              programID,
		ProgramGroupId:                  programGroupID,
		Prime:                           1,
		AreaId:                          2,
		ProgramCategoryId:               11,
		ParentProgramCategoryId:         1,
		Title:                           "缓存失效失败后仍成功更新",
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
		RushSaleOpenTime:                "2026-12-31 18:00:00",
		RushSaleEndTime:                 "2026-12-31 19:00:00",
		Status:                          1,
	})
	if err != nil {
		t.Fatalf("UpdateProgram returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected UpdateProgram to return success, got %+v", resp)
	}

	program, err := svcCtx.DProgramModel.FindOne(ctx, programID)
	if err != nil {
		t.Fatalf("DProgramModel.FindOne returned error: %v", err)
	}
	if program.Title != "缓存失效失败后仍成功更新" {
		t.Fatalf("expected title to be updated, got %+v", program)
	}
}

func TestUpdateProgramRejectsMissingRushSaleWindow(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	l := logicpkg.NewUpdateProgramLogic(context.Background(), svcCtx)
	_, err := l.UpdateProgram(&pb.UpdateProgramReq{
		Id:                           10001,
		ProgramGroupId:               20001,
		Prime:                        1,
		AreaId:                       2,
		ProgramCategoryId:            11,
		ParentProgramCategoryId:      1,
		Title:                        "缺少售卖窗口",
		Detail:                       "<p>updated detail</p>",
		PerOrderLimitPurchaseCount:   8,
		PerAccountLimitPurchaseCount: 10,
		PermitRefund:                 2,
		PermitChooseSeat:             0,
		ElectronicDeliveryTicket:     1,
		ElectronicInvoice:            0,
		ProgramStatus:                1,
		IssueTime:                    "2026-09-01 09:00:00",
		Status:                       1,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument when rush sale window is missing, got %v", err)
	}
}

func TestUpdateProgramRejectsRushSaleOpenTimeAfterExistingShowTime(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)
	t.Cleanup(func() {
		resetProgramDomainState(t)
	})

	l := logicpkg.NewUpdateProgramLogic(context.Background(), svcCtx)
	_, err := l.UpdateProgram(&pb.UpdateProgramReq{
		Id:                              10001,
		ProgramGroupId:                  20001,
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
		RushSaleOpenTime:                "2026-12-31 20:00:00",
		RushSaleEndTime:                 "2026-12-31 21:00:00",
		Status:                          1,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument when rush sale open time is later than existing show time, got %v", err)
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

func replaceProgramCacheInvalidatorWithFailingRedis(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()

	redis := svcCtx.Redis
	if redis == nil {
		t.Fatal("expected redis client to be configured")
	}
	redis.Type = "invalid"

	svcCtx.ProgramCacheInvalidator = programcache.NewProgramCacheInvalidator(redis, svcCtx.ProgramDetailViewCache)
}

type failingInvalidationPublisher struct{}

func (failingInvalidationPublisher) Publish(context.Context, programcache.InvalidationMessage) error {
	return fmt.Errorf("publish failed")
}

func requireProgramTableColumn(t *testing.T, db *sql.DB, tableName, columnName string) {
	t.Helper()

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
		tableName,
		columnName,
	).Scan(&count); err != nil {
		t.Fatalf("query table column metadata error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected column %s.%s to exist", tableName, columnName)
	}
}

func requireProgramIndex(t *testing.T, db *sql.DB, tableName, indexName string) {
	t.Helper()

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(1) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?`,
		tableName,
		indexName,
	).Scan(&count); err != nil {
		t.Fatalf("query table index metadata error: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected index %s on %s to exist", indexName, tableName)
	}
}
