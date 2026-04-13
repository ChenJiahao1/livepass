package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/internal/server"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
)

func TestPrimeRushRuntimeClearsTransientKeysAndRebuildsProjection(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	prefix := fmt.Sprintf("damai-go:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	svcCtx.AttemptStore = rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})

	ctx := context.Background()
	showTimeID := int64(83001)
	otherShowTimeID := showTimeID + 1
	ticketCategoryID := int64(84001)
	now := time.Date(2026, time.April, 13, 21, 0, 0, 0, time.Local)

	seedPrimeRushRuntimeGuards(t, svcCtx, showTimeID, now)
	programRPC.getProgramPreorderResp = buildTestProgramPreorder(showTimeID, ticketCategoryID, 2, 4, 299)
	programRPC.getProgramPreorderResp.ShowTimeId = showTimeID
	programRPC.getProgramPreorderResp.TicketCategoryVoList[0].AdmissionQuota = 9

	if err := svcCtx.Redis.SetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "user_inflight", 93001), "1"); err != nil {
		t.Fatalf("SetCtx(user_inflight) error = %v", err)
	}
	if err := svcCtx.Redis.SetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "viewer_inflight", 93002), "1"); err != nil {
		t.Fatalf("SetCtx(viewer_inflight) error = %v", err)
	}
	if err := svcCtx.Redis.SetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "fingerprint", 93001), "old"); err != nil {
		t.Fatalf("SetCtx(fingerprint) error = %v", err)
	}
	if err := svcCtx.Redis.SetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "user_active", 93001), "stale"); err != nil {
		t.Fatalf("SetCtx(stale user_active) error = %v", err)
	}
	if err := svcCtx.Redis.SetCtx(ctx, rushTestScopedKey(prefix, otherShowTimeID, "user_inflight", 93001), "keep"); err != nil {
		t.Fatalf("SetCtx(other user_inflight) error = %v", err)
	}

	if err := logicpkg.PrimeRushRuntime(ctx, svcCtx, showTimeID); err != nil {
		t.Fatalf("PrimeRushRuntime() error = %v", err)
	}

	if exists, err := svcCtx.Redis.ExistsCtx(ctx, rushTestScopedKey(prefix, showTimeID, "user_inflight", 93001)); err != nil {
		t.Fatalf("ExistsCtx(user_inflight) error = %v", err)
	} else if exists {
		t.Fatalf("expected showTime user_inflight removed")
	}
	if exists, err := svcCtx.Redis.ExistsCtx(ctx, rushTestScopedKey(prefix, showTimeID, "viewer_inflight", 93002)); err != nil {
		t.Fatalf("ExistsCtx(viewer_inflight) error = %v", err)
	} else if exists {
		t.Fatalf("expected showTime viewer_inflight removed")
	}
	if exists, err := svcCtx.Redis.ExistsCtx(ctx, rushTestScopedKey(prefix, showTimeID, "fingerprint", 93001)); err != nil {
		t.Fatalf("ExistsCtx(fingerprint) error = %v", err)
	} else if exists {
		t.Fatalf("expected showTime fingerprint removed")
	}
	if exists, err := svcCtx.Redis.ExistsCtx(ctx, rushTestScopedKey(prefix, otherShowTimeID, "user_inflight", 93001)); err != nil {
		t.Fatalf("ExistsCtx(other user_inflight) error = %v", err)
	} else if !exists {
		t.Fatalf("expected other showTime transient key preserved")
	}

	if got, err := svcCtx.Redis.GetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "user_active", 93001)); err != nil {
		t.Fatalf("GetCtx(user_active) error = %v", err)
	} else if got != "910001" {
		t.Fatalf("user_active value = %s, want 910001", got)
	}
	if got, err := svcCtx.Redis.GetCtx(ctx, rushTestScopedKey(prefix, showTimeID, "viewer_active", 93011)); err != nil {
		t.Fatalf("GetCtx(viewer_active) error = %v", err)
	} else if got != "910001" {
		t.Fatalf("viewer_active value = %s, want 910001", got)
	}
	if quota, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(ctx, showTimeID, ticketCategoryID); err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	} else if !ok || quota != 9 {
		t.Fatalf("quota = %d ok=%t, want 9/true", quota, ok)
	}
}

func TestPrimeRushRuntimeRPCReturnsSuccess(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	showTimeID := int64(83101)
	ticketCategoryID := int64(84101)
	now := time.Date(2026, time.April, 13, 21, 5, 0, 0, time.Local)
	seedPrimeRushRuntimeGuards(t, svcCtx, showTimeID, now)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(showTimeID, ticketCategoryID, 2, 4, 299)
	programRPC.getProgramPreorderResp.ShowTimeId = showTimeID

	resp, err := server.NewOrderRpcServer(svcCtx).PrimeRushRuntime(context.Background(), &pb.PrimeRushRuntimeReq{
		ShowTimeId: showTimeID,
	})
	if err != nil {
		t.Fatalf("PrimeRushRuntime RPC error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected PrimeRushRuntime RPC success, got %+v", resp)
	}
}

func seedPrimeRushRuntimeGuards(t *testing.T, svcCtx *svc.ServiceContext, showTimeID int64, now time.Time) {
	t.Helper()

	db := openOrderTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	if _, err := db.Exec(
		"INSERT INTO d_order_user_guard (id, order_number, program_id, show_time_id, user_id, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, 1)",
		990001, 910001, showTimeID, showTimeID, 93001, now, now,
	); err != nil {
		t.Fatalf("insert user guard error: %v", err)
	}
	if _, err := db.Exec(
		"INSERT INTO d_order_viewer_guard (id, order_number, program_id, show_time_id, viewer_id, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, 1)",
		990011, 910001, showTimeID, showTimeID, 93011, now, now,
	); err != nil {
		t.Fatalf("insert viewer guard error: %v", err)
	}
}
