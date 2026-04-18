package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	logicpkg "livepass/services/order-rpc/internal/logic"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/server"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"
	programrpc "livepass/services/program-rpc/programrpc"
)

func TestPrimeRushRuntimeClearsTransientKeysAndRebuildsProjection(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	svcCtx.AttemptStore = rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})

	ctx := context.Background()
	programID := int64(82001)
	showTimeID := int64(83001)
	secondShowTimeID := showTimeID + 1
	otherShowTimeID := showTimeID + 10
	ticketCategoryID := int64(84001)
	secondTicketCategoryID := ticketCategoryID + 1
	staleTicketCategoryID := ticketCategoryID + 1
	userID := int64(93001 + showTimeID)
	viewerID := int64(93011 + showTimeID)
	now := time.Date(2026, time.April, 13, 21, 0, 0, 0, time.Local)

	seedPrimeRushRuntimeGuards(t, svcCtx, programID, showTimeID, now)
	seedPrimeRushRuntimeGuards(t, svcCtx, programID, secondShowTimeID, now.Add(time.Second))
	programRPC.listProgramShowTimesForRushResp = &programrpc.ListProgramShowTimesForRushResp{
		List: []*programrpc.ProgramShowTimeForRushInfo{
			{ShowTimeId: showTimeID},
			{ShowTimeId: secondShowTimeID},
		},
	}
	programRPC.getProgramPreorderRespByProgramID = map[int64]*programrpc.ProgramPreorderInfo{
		showTimeID: func() *programrpc.ProgramPreorderInfo {
			resp := buildTestProgramPreorder(programID, ticketCategoryID, 2, 4, 299)
			resp.ShowTimeId = showTimeID
			resp.TicketCategoryVoList[0].AdmissionQuota = 9
			return resp
		}(),
		secondShowTimeID: func() *programrpc.ProgramPreorderInfo {
			resp := buildTestProgramPreorder(programID, secondTicketCategoryID, 2, 4, 399)
			resp.ShowTimeId = secondShowTimeID
			resp.TicketCategoryVoList[0].AdmissionQuota = 7
			return resp
		}(),
	}

	if err := svcCtx.Redis.HsetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "user_inflight", userID), rushTestHashField(userID), "1"); err != nil {
		t.Fatalf("HsetCtx(user_inflight) error = %v", err)
	}
	if err := svcCtx.Redis.HsetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "viewer_inflight", 93002), rushTestHashField(93002), "1"); err != nil {
		t.Fatalf("HsetCtx(viewer_inflight) error = %v", err)
	}
	if err := svcCtx.Redis.HsetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "user_active", userID), rushTestHashField(userID), "stale"); err != nil {
		t.Fatalf("HsetCtx(stale user_active) error = %v", err)
	}
	if err := svcCtx.Redis.HsetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "quota", staleTicketCategoryID), rushTestHashField(staleTicketCategoryID), "99"); err != nil {
		t.Fatalf("HsetCtx(stale quota) error = %v", err)
	}
	if err := svcCtx.Redis.HsetCtx(ctx, rushTestScopedKey(prefix, otherShowTimeID, "quota", staleTicketCategoryID), rushTestHashField(staleTicketCategoryID), "77"); err != nil {
		t.Fatalf("HsetCtx(other quota) error = %v", err)
	}
	if err := svcCtx.Redis.HsetCtx(ctx, rushTestScopedKey(prefix, otherShowTimeID, "user_inflight", userID), rushTestHashField(userID), "keep"); err != nil {
		t.Fatalf("HsetCtx(other user_inflight) error = %v", err)
	}

	if err := logicpkg.PrimeRushRuntime(ctx, svcCtx, programID); err != nil {
		t.Fatalf("PrimeRushRuntime() error = %v", err)
	}

	if fields, err := svcCtx.Redis.HgetallCtx(ctx, rushTestScopedKey(prefix, showTimeID, "user_inflight", userID)); err != nil {
		t.Fatalf("HgetallCtx(user_inflight) error = %v", err)
	} else if _, ok := fields[rushTestHashField(userID)]; ok {
		t.Fatalf("expected showTime user_inflight removed, got %+v", fields)
	}
	if fields, err := svcCtx.Redis.HgetallCtx(ctx, rushTestScopedKey(prefix, showTimeID, "viewer_inflight", 93002)); err != nil {
		t.Fatalf("HgetallCtx(viewer_inflight) error = %v", err)
	} else if _, ok := fields[rushTestHashField(93002)]; ok {
		t.Fatalf("expected showTime viewer_inflight removed, got %+v", fields)
	}
	if got, err := svcCtx.Redis.HgetCtx(ctx, rushTestScopedKey(prefix, otherShowTimeID, "user_inflight", userID), rushTestHashField(userID)); err != nil {
		t.Fatalf("HgetCtx(other user_inflight) error = %v", err)
	} else if got != "keep" {
		t.Fatalf("expected other showTime transient field preserved, got %s", got)
	}

	if got, err := svcCtx.Redis.HgetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "user_active", userID), rushTestHashField(userID)); err != nil {
		t.Fatalf("HgetCtx(user_active) error = %v", err)
	} else if got != fmt.Sprintf("%d", 910001+showTimeID) {
		t.Fatalf("user_active value = %s, want %d", got, 910001+showTimeID)
	}
	if got, err := svcCtx.Redis.HgetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "viewer_active", viewerID), rushTestHashField(viewerID)); err != nil {
		t.Fatalf("HgetCtx(viewer_active) error = %v", err)
	} else if got != fmt.Sprintf("%d", 910001+showTimeID) {
		t.Fatalf("viewer_active value = %s, want %d", got, 910001+showTimeID)
	}
	if fields, err := svcCtx.Redis.HgetallCtx(ctx, rushTestScopedKey(prefix, showTimeID, "quota", staleTicketCategoryID)); err != nil {
		t.Fatalf("HgetallCtx(stale quota) error = %v", err)
	} else if _, ok := fields[rushTestHashField(staleTicketCategoryID)]; ok {
		t.Fatalf("expected stale showTime quota removed, got %+v", fields)
	}
	if got, err := svcCtx.Redis.HgetCtx(ctx, rushTestScopedKey(prefix, otherShowTimeID, "quota", staleTicketCategoryID), rushTestHashField(staleTicketCategoryID)); err != nil {
		t.Fatalf("HgetCtx(other quota) error = %v", err)
	} else if got != "77" {
		t.Fatalf("other quota value = %s, want 77", got)
	}
	if quota, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(ctx, showTimeID, ticketCategoryID); err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	} else if !ok || quota != 9 {
		t.Fatalf("quota = %d ok=%t, want 9/true", quota, ok)
	}
	if quota, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(ctx, secondShowTimeID, secondTicketCategoryID); err != nil {
		t.Fatalf("GetQuotaAvailable(second) error = %v", err)
	} else if !ok || quota != 7 {
		t.Fatalf("second quota = %d ok=%t, want 7/true", quota, ok)
	}
}

func TestPrimeRushRuntimeRPCReturnsSuccess(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programID := int64(82101)
	showTimeID := int64(83101)
	ticketCategoryID := int64(84101)
	now := time.Date(2026, time.April, 13, 21, 5, 0, 0, time.Local)
	seedPrimeRushRuntimeGuards(t, svcCtx, programID, showTimeID, now)

	programRPC.listProgramShowTimesForRushResp = &programrpc.ListProgramShowTimesForRushResp{
		List: []*programrpc.ProgramShowTimeForRushInfo{{ShowTimeId: showTimeID}},
	}
	programRPC.getProgramPreorderRespByProgramID = map[int64]*programrpc.ProgramPreorderInfo{
		showTimeID: func() *programrpc.ProgramPreorderInfo {
			resp := buildTestProgramPreorder(programID, ticketCategoryID, 2, 4, 299)
			resp.ShowTimeId = showTimeID
			return resp
		}(),
	}

	resp, err := server.NewOrderRpcServer(svcCtx).PrimeRushRuntime(context.Background(), &pb.PrimeRushRuntimeReq{
		ProgramId: programID,
	})
	if err != nil {
		t.Fatalf("PrimeRushRuntime RPC error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected PrimeRushRuntime RPC success, got %+v", resp)
	}
}

func seedPrimeRushRuntimeGuards(t *testing.T, svcCtx *svc.ServiceContext, programID, showTimeID int64, now time.Time) {
	t.Helper()

	db := openOrderTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	if _, err := db.Exec(
		"INSERT INTO d_order_user_guard (id, order_number, program_id, show_time_id, user_id, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, 1)",
		990001+showTimeID, 910001+showTimeID, programID, showTimeID, 93001+showTimeID, now, now,
	); err != nil {
		t.Fatalf("insert user guard error: %v", err)
	}
	if _, err := db.Exec(
		"INSERT INTO d_order_viewer_guard (id, order_number, program_id, show_time_id, viewer_id, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, 1)",
		990011+showTimeID, 910001+showTimeID, programID, showTimeID, 93011+showTimeID, now, now,
	); err != nil {
		t.Fatalf("insert viewer guard error: %v", err)
	}
}
