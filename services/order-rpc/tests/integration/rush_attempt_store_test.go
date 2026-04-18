package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/rush"
)

func TestAdmitKeepsRejectAndReuseSemantics(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 30, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID + 99
	viewerIDs = viewerIDs[:2]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 6); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	first, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		Now:              now,
	})
	if err != nil {
		t.Fatalf("Admit(first) error = %v", err)
	}
	if first.Decision != rush.AdmitDecisionAccepted || first.OrderNumber != orderNumbers[0] {
		t.Fatalf("unexpected first admission result: %+v", first)
	}

	reused, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		Now:              now.Add(200 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Admit(reused) error = %v", err)
	}
	if reused.Decision != rush.AdmitDecisionReused || reused.OrderNumber != orderNumbers[0] {
		t.Fatalf("unexpected reused admission result: %+v", reused)
	}

	rejected, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[1],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		Now:              now.Add(400 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Admit(rejected) error = %v", err)
	}
	if rejected.Decision != rush.AdmitDecisionRejected || rejected.RejectCode != rush.AdmitRejectUserInflightConflict {
		t.Fatalf("unexpected rejected admission result: %+v", rejected)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, showTimeID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("unexpected quota snapshot: ok=%t available=%d", ok, available)
	}
}

func TestAdmitCreatesPendingAttempt(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID + 100
	viewerIDs = viewerIDs[:2]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	record, err := store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if record.State != rush.AttemptStatePending {
		t.Fatalf("expected pending attempt, got %+v", record)
	}
}

func TestFailBeforeProcessingTransitionsAcceptedToFailedOnce(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 31, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID + 101
	viewerIDs = viewerIDs[:2]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	record, err := store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	outcome, err := store.FailBeforeProcessing(ctx, record, rush.AttemptReasonSeatExhausted, now.Add(time.Second))
	if err != nil {
		t.Fatalf("FailBeforeProcessing() error = %v", err)
	}
	if outcome != rush.AttemptTransitioned {
		t.Fatalf("unexpected first FailBeforeProcessing outcome: %s", outcome)
	}

	record, err = store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() after fail error = %v", err)
	}
	if record.State != rush.AttemptStateFailed || record.ReasonCode != rush.AttemptReasonSeatExhausted {
		t.Fatalf("unexpected failed record: %+v", record)
	}

	outcome, err = store.FailBeforeProcessing(ctx, record, rush.AttemptReasonSeatExhausted, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("FailBeforeProcessing(second) error = %v", err)
	}
	if outcome != rush.AttemptAlreadyFailed {
		t.Fatalf("unexpected second FailBeforeProcessing outcome: %s", outcome)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, showTimeID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("expected quota restored once to 4, got ok=%t available=%d", ok, available)
	}

	userInflightRedisKey := rushTestScopedKey(prefix, showTimeID, "user_inflight", userID)
	viewerInflightRedisKey := rushTestScopedKey(prefix, showTimeID, "viewer_inflight", viewerIDs[0])
	userInflightFields, err := svcCtx.Redis.HgetallCtx(ctx, userInflightRedisKey)
	if err != nil {
		t.Fatalf("HgetallCtx(user_inflight) error = %v", err)
	}
	if _, ok := userInflightFields[rushTestHashField(userID)]; ok {
		t.Fatalf("expected user_inflight field to be removed, got %+v", userInflightFields)
	}
	viewerInflightFields, err := svcCtx.Redis.HgetallCtx(ctx, viewerInflightRedisKey)
	if err != nil {
		t.Fatalf("HgetallCtx(viewer_inflight) error = %v", err)
	}
	if _, ok := viewerInflightFields[rushTestHashField(viewerIDs[0])]; ok {
		t.Fatalf("expected viewer_inflight field to be removed, got %+v", viewerInflightFields)
	}
}

func TestRefreshProcessingLeaseRequiresProcessingState(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 32, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID + 102
	viewerIDs = viewerIDs[:1]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 2); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	orderNumber := orderNumbers[0]
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(500*time.Millisecond))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume() error = %v", err)
	}
	if !shouldProcess || record == nil || record.State != rush.AttemptStateProcessing {
		t.Fatalf("expected processing claim, got shouldProcess=%t record=%+v", shouldProcess, record)
	}

	ok, err := store.RefreshProcessingLease(ctx, showTimeID, orderNumber, now.Add(time.Second))
	if err != nil {
		t.Fatalf("RefreshProcessingLease(processing) error = %v", err)
	}
	if !ok {
		t.Fatalf("expected lease refresh while processing to succeed")
	}

	if err := store.FinalizeSuccess(ctx, record, now.Add(2*time.Second)); err != nil {
		t.Fatalf("FinalizeSuccess() error = %v", err)
	}

	ok, err = store.RefreshProcessingLease(ctx, showTimeID, orderNumber, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("RefreshProcessingLease(success) error = %v", err)
	}
	if ok {
		t.Fatalf("expected lease refresh after terminal state to fail")
	}
}

func TestPrepareAttemptForConsumeOnlyClaimsPendingOnce(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 18, 10, 5, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID + 200
	viewerIDs = viewerIDs[:1]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		SaleWindowEndAt:  now.Add(-3 * time.Hour),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(time.Millisecond))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume() error = %v", err)
	}
	if !shouldProcess || record == nil || record.State != rush.AttemptStateProcessing {
		t.Fatalf("expected first claim to processing, got shouldProcess=%t record=%+v", shouldProcess, record)
	}

	second, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(2*time.Millisecond))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume(second) error = %v", err)
	}
	if shouldProcess || second == nil || second.State != rush.AttemptStateProcessing {
		t.Fatalf("expected second claim to lose ownership, got shouldProcess=%t record=%+v", shouldProcess, second)
	}
}

func TestPrepareAttemptForConsumeDoesNotReviveAttemptWithoutTTL(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 18, 10, 6, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID + 201
	viewerIDs = viewerIDs[:1]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	attemptKey := rushTestScopedKey(prefix, showTimeID, "attempt", orderNumber)
	if _, err := svcCtx.Redis.PersistCtx(ctx, attemptKey); err != nil {
		t.Fatalf("PersistCtx(attempt) error = %v", err)
	}

	record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(time.Millisecond))
	if !errors.Is(err, xerr.ErrOrderNotFound) {
		t.Fatalf("expected missing/expired error, got record=%+v shouldProcess=%t err=%v", record, shouldProcess, err)
	}
	if shouldProcess {
		t.Fatalf("attempt without TTL must not be claimed")
	}
}

func TestExpiredAttemptCannotBeClaimedOrRevived(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   time.Second,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 18, 10, 10, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID + 300
	viewerIDs = viewerIDs[:1]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 2); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	attemptKey := rushTestScopedKey(prefix, showTimeID, "attempt", orderNumber)
	if err := svcCtx.Redis.ExpireCtx(ctx, attemptKey, 1); err != nil {
		t.Fatalf("ExpireCtx(attempt) error = %v", err)
	}
	time.Sleep(1200 * time.Millisecond)

	_, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, time.Now())
	if !errors.Is(err, xerr.ErrOrderNotFound) && shouldProcess {
		t.Fatalf("expired attempt must not be claimed, shouldProcess=%t err=%v", shouldProcess, err)
	}

	ok, err := store.RefreshProcessingLease(ctx, showTimeID, orderNumber, time.Now())
	if err != nil {
		t.Fatalf("RefreshProcessingLease(expired) error = %v", err)
	}
	if ok {
		t.Fatalf("expired attempt must not be revived")
	}
}

func TestFinalizeFailureDoesNotDoubleCompensate(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 33, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID + 202
	viewerIDs = viewerIDs[:2]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      2,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(100*time.Millisecond))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume() error = %v", err)
	}
	if !shouldProcess || record == nil || record.State != rush.AttemptStateProcessing {
		t.Fatalf("expected processing claim, got shouldProcess=%t record=%+v", shouldProcess, record)
	}
	outcome, err := store.FinalizeFailure(ctx, record, rush.AttemptReasonSeatExhausted, now.Add(time.Second))
	if err != nil {
		t.Fatalf("FinalizeFailure() error = %v", err)
	}
	if outcome != rush.AttemptTransitioned {
		t.Fatalf("unexpected first FinalizeFailure outcome: %s", outcome)
	}

	record, err = store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() after finalize failure error = %v", err)
	}
	outcome, err = store.FinalizeFailure(ctx, record, rush.AttemptReasonSeatExhausted, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("FinalizeFailure(second) error = %v", err)
	}
	if outcome != rush.AttemptAlreadyFailed {
		t.Fatalf("unexpected second FinalizeFailure outcome: %s", outcome)
	}

	record, err = store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() final error = %v", err)
	}
	if record.State != rush.AttemptStateFailed || record.ReasonCode != rush.AttemptReasonSeatExhausted {
		t.Fatalf("unexpected final failed record: %+v", record)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, showTimeID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("expected quota restored once to 4, got ok=%t available=%d", ok, available)
	}
}

func TestFinalizeClosedOrderReleasesActiveProjectionOnce(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 34, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID + 303
	viewerIDs = viewerIDs[:2]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(100*time.Millisecond))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume() error = %v", err)
	}
	if !shouldProcess || record == nil {
		t.Fatalf("expected claim success, got shouldProcess=%t record=%+v", shouldProcess, record)
	}
	if err := store.FinalizeSuccess(ctx, record, now.Add(time.Second)); err != nil {
		t.Fatalf("FinalizeSuccess() error = %v", err)
	}

	record, err = store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() after finalize success error = %v", err)
	}
	if record.State != rush.AttemptStateSuccess {
		t.Fatalf("expected success state, got %+v", record)
	}

	userActiveRedisKey := rushTestScopedKey(prefix, showTimeID, "user_active", userID)
	viewerActiveRedisKey := rushTestScopedKey(prefix, showTimeID, "viewer_active", viewerIDs[0])

	userActiveOrderNo, err := svcCtx.Redis.HgetCtx(ctx, userActiveRedisKey, rushTestHashField(userID))
	if err != nil {
		t.Fatalf("HgetCtx(user_active) error = %v", err)
	}
	if userActiveOrderNo != strconv.FormatInt(orderNumber, 10) {
		t.Fatalf("expected user_active order %d, got %s", orderNumber, userActiveOrderNo)
	}

	outcome, err := store.FinalizeClosedOrder(ctx, record, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("FinalizeClosedOrder() error = %v", err)
	}
	if outcome != rush.AttemptTransitioned {
		t.Fatalf("unexpected first FinalizeClosedOrder outcome: %s", outcome)
	}

	record, err = store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() after close finalize error = %v", err)
	}
	if record.State != rush.AttemptStateFailed || record.ReasonCode != rush.AttemptReasonClosedOrderReleased {
		t.Fatalf("unexpected closed-order failed record: %+v", record)
	}

	outcome, err = store.FinalizeClosedOrder(ctx, record, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("FinalizeClosedOrder(second) error = %v", err)
	}
	if outcome != rush.AttemptAlreadyFailed {
		t.Fatalf("unexpected second FinalizeClosedOrder outcome: %s", outcome)
	}

	userActiveFields, err := svcCtx.Redis.HgetallCtx(ctx, userActiveRedisKey)
	if err != nil {
		t.Fatalf("HgetallCtx(user_active) error = %v", err)
	}
	if _, ok := userActiveFields[rushTestHashField(userID)]; ok {
		t.Fatalf("expected user_active field to be removed, got %+v", userActiveFields)
	}
	viewerActiveFields, err := svcCtx.Redis.HgetallCtx(ctx, viewerActiveRedisKey)
	if err != nil {
		t.Fatalf("HgetallCtx(viewer_active) error = %v", err)
	}
	if _, ok := viewerActiveFields[rushTestHashField(viewerIDs[0])]; ok {
		t.Fatalf("expected viewer_active field to be removed, got %+v", viewerActiveFields)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, showTimeID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("expected quota restored once to 4, got ok=%t available=%d", ok, available)
	}
}

func TestRushAttemptStorePrimeClearsOnlyTargetShowTimeTransientKeys(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 35, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	targetShowTimeID := programID + 401
	otherShowTimeID := programID + 402
	viewerIDs = viewerIDs[:1]
	targetUserInflightKey := rushTestScopedKey(prefix, targetShowTimeID, "user_inflight", userID)
	targetViewerInflightKey := rushTestScopedKey(prefix, targetShowTimeID, "viewer_inflight", viewerIDs[0])
	otherUserInflightKey := rushTestScopedKey(prefix, otherShowTimeID, "user_inflight", userID+1)
	otherViewerInflightKey := rushTestScopedKey(prefix, otherShowTimeID, "viewer_inflight", viewerIDs[0]+1)

	if err := store.SetQuotaAvailable(ctx, targetShowTimeID, ticketCategoryID, 2); err != nil {
		t.Fatalf("SetQuotaAvailable(target) error = %v", err)
	}
	if err := store.SetQuotaAvailable(ctx, otherShowTimeID, ticketCategoryID, 2); err != nil {
		t.Fatalf("SetQuotaAvailable(other) error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       targetShowTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit(target) error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[1],
		UserID:           userID + 1,
		ProgramID:        programID,
		ShowTimeID:       otherShowTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        []int64{viewerIDs[0] + 1},
		TicketCount:      1,
		Now:              now.Add(time.Second),
	}); err != nil {
		t.Fatalf("Admit(other) error = %v", err)
	}

	if err := store.ClearUserInflightByShowTime(ctx, targetShowTimeID); err != nil {
		t.Fatalf("ClearUserInflightByShowTime() error = %v", err)
	}
	if err := store.ClearViewerInflightByShowTime(ctx, targetShowTimeID); err != nil {
		t.Fatalf("ClearViewerInflightByShowTime() error = %v", err)
	}
	targetUserInflightFields, err := svcCtx.Redis.HgetallCtx(ctx, targetUserInflightKey)
	if err != nil {
		t.Fatalf("HgetallCtx(target user_inflight) error = %v", err)
	}
	if _, ok := targetUserInflightFields[rushTestHashField(userID)]; ok {
		t.Fatalf("expected target user_inflight field removed, got %+v", targetUserInflightFields)
	}
	targetViewerInflightFields, err := svcCtx.Redis.HgetallCtx(ctx, targetViewerInflightKey)
	if err != nil {
		t.Fatalf("HgetallCtx(target viewer_inflight) error = %v", err)
	}
	if _, ok := targetViewerInflightFields[rushTestHashField(viewerIDs[0])]; ok {
		t.Fatalf("expected target viewer_inflight field removed, got %+v", targetViewerInflightFields)
	}

	otherUserInflightValue, err := svcCtx.Redis.HgetCtx(ctx, otherUserInflightKey, rushTestHashField(userID+1))
	if err != nil {
		t.Fatalf("HgetCtx(other user_inflight) error = %v", err)
	}
	if otherUserInflightValue != fmt.Sprintf("%d", orderNumbers[1]) {
		t.Fatalf("expected other showTime user_inflight to remain, got %s", otherUserInflightValue)
	}
	otherViewerInflightValue, err := svcCtx.Redis.HgetCtx(ctx, otherViewerInflightKey, rushTestHashField(viewerIDs[0]+1))
	if err != nil {
		t.Fatalf("HgetCtx(other viewer_inflight) error = %v", err)
	}
	if otherViewerInflightValue != fmt.Sprintf("%d", orderNumbers[1]) {
		t.Fatalf("expected other showTime viewer_inflight to remain, got %s", otherViewerInflightValue)
	}
}

func TestRushAttemptStorePrimeReplacesActiveProjectionByShowTime(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	targetShowTimeID := int64(99001)
	otherShowTimeID := int64(99002)
	targetUserActiveKey := rushTestScopedKey(prefix, targetShowTimeID, "user_active", 3001)
	targetViewerActiveKey := rushTestScopedKey(prefix, targetShowTimeID, "viewer_active", 4001)
	staleTargetUserActiveKey := rushTestScopedKey(prefix, targetShowTimeID, "user_active", 3999)
	staleTargetViewerActiveKey := rushTestScopedKey(prefix, targetShowTimeID, "viewer_active", 4999)
	otherUserActiveKey := rushTestScopedKey(prefix, otherShowTimeID, "user_active", 3002)
	otherViewerActiveKey := rushTestScopedKey(prefix, otherShowTimeID, "viewer_active", 4002)
	targetQuotaKey := rushTestScopedKey(prefix, targetShowTimeID, "quota", 5001)

	if err := svcCtx.Redis.HsetCtx(ctx, targetUserActiveKey, rushTestHashField(3001), "old-order"); err != nil {
		t.Fatalf("HsetCtx(target user_active) error = %v", err)
	}
	if err := svcCtx.Redis.HsetCtx(ctx, targetViewerActiveKey, rushTestHashField(4001), "old-order"); err != nil {
		t.Fatalf("HsetCtx(target viewer_active) error = %v", err)
	}
	if err := svcCtx.Redis.HsetCtx(ctx, staleTargetUserActiveKey, rushTestHashField(3999), "stale-user-order"); err != nil {
		t.Fatalf("HsetCtx(stale target user_active) error = %v", err)
	}
	if err := svcCtx.Redis.HsetCtx(ctx, staleTargetViewerActiveKey, rushTestHashField(4999), "stale-viewer-order"); err != nil {
		t.Fatalf("HsetCtx(stale target viewer_active) error = %v", err)
	}
	if err := svcCtx.Redis.HsetCtx(ctx, otherUserActiveKey, rushTestHashField(3002), "other-order"); err != nil {
		t.Fatalf("HsetCtx(other user_active) error = %v", err)
	}
	if err := svcCtx.Redis.HsetCtx(ctx, otherViewerActiveKey, rushTestHashField(4002), "other-order"); err != nil {
		t.Fatalf("HsetCtx(other viewer_active) error = %v", err)
	}
	if err := store.SetQuotaAvailable(ctx, targetShowTimeID, 5001, 9); err != nil {
		t.Fatalf("SetQuotaAvailable(target) error = %v", err)
	}

	if err := store.ReplaceUserActiveByShowTime(ctx, targetShowTimeID, map[int64]int64{3001: 91001}, 60); err != nil {
		t.Fatalf("ReplaceUserActiveByShowTime() error = %v", err)
	}
	if err := store.ReplaceViewerActiveByShowTime(ctx, targetShowTimeID, map[int64]int64{4001: 91001}, 60); err != nil {
		t.Fatalf("ReplaceViewerActiveByShowTime() error = %v", err)
	}

	targetUserActiveValue, err := svcCtx.Redis.HgetCtx(ctx, targetUserActiveKey, rushTestHashField(3001))
	if err != nil {
		t.Fatalf("HgetCtx(target user_active) error = %v", err)
	}
	if targetUserActiveValue != "91001" {
		t.Fatalf("expected target user_active to be replaced, got %s", targetUserActiveValue)
	}
	targetViewerActiveValue, err := svcCtx.Redis.HgetCtx(ctx, targetViewerActiveKey, rushTestHashField(4001))
	if err != nil {
		t.Fatalf("HgetCtx(target viewer_active) error = %v", err)
	}
	if targetViewerActiveValue != "91001" {
		t.Fatalf("expected target viewer_active to be replaced, got %s", targetViewerActiveValue)
	}
	staleTargetUserFields, err := svcCtx.Redis.HgetallCtx(ctx, staleTargetUserActiveKey)
	if err != nil {
		t.Fatalf("HgetallCtx(stale target user_active) error = %v", err)
	}
	if _, ok := staleTargetUserFields[rushTestHashField(3999)]; ok {
		t.Fatalf("expected stale target user_active field removed, got %+v", staleTargetUserFields)
	}
	staleTargetViewerFields, err := svcCtx.Redis.HgetallCtx(ctx, staleTargetViewerActiveKey)
	if err != nil {
		t.Fatalf("HgetallCtx(stale target viewer_active) error = %v", err)
	}
	if _, ok := staleTargetViewerFields[rushTestHashField(4999)]; ok {
		t.Fatalf("expected stale target viewer_active field removed, got %+v", staleTargetViewerFields)
	}
	otherUserActiveValue, err := svcCtx.Redis.HgetCtx(ctx, otherUserActiveKey, rushTestHashField(3002))
	if err != nil {
		t.Fatalf("HgetCtx(other user_active) error = %v", err)
	}
	if otherUserActiveValue != "other-order" {
		t.Fatalf("expected other showTime user_active to remain, got %s", otherUserActiveValue)
	}
	otherViewerActiveValue, err := svcCtx.Redis.HgetCtx(ctx, otherViewerActiveKey, rushTestHashField(4002))
	if err != nil {
		t.Fatalf("HgetCtx(other viewer_active) error = %v", err)
	}
	if otherViewerActiveValue != "other-order" {
		t.Fatalf("expected other showTime viewer_active to remain, got %s", otherViewerActiveValue)
	}

	quotaValue, err := svcCtx.Redis.HgetCtx(ctx, targetQuotaKey, rushTestHashField(5001))
	if err != nil {
		t.Fatalf("HgetCtx(target quota) error = %v", err)
	}
	if quotaValue != "9" {
		t.Fatalf("expected quota key to remain unchanged, got %s", quotaValue)
	}
}

func TestPrepareAttemptForConsumeTransitionsPendingToProcessingOnce(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 13, 22, 0, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID
	viewerIDs = viewerIDs[:1]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(time.Second))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume() error = %v", err)
	}
	if !shouldProcess {
		t.Fatalf("expected shouldProcess=true, got shouldProcess=%t", shouldProcess)
	}
	if record == nil || record.State != rush.AttemptStateProcessing {
		t.Fatalf("expected processing record, got %+v", record)
	}
	if record.OrderNumber != orderNumber || record.ShowTimeID != showTimeID || record.UserID != userID {
		t.Fatalf("unexpected returned record: %+v", record)
	}

	record, shouldProcess, err = store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume(second) error = %v", err)
	}
	if shouldProcess {
		t.Fatalf("expected second prepare to skip processing, got shouldProcess=true record=%+v", record)
	}
	if record == nil || record.State != rush.AttemptStateProcessing {
		t.Fatalf("expected processing record on second prepare, got %+v", record)
	}
}

func TestPrepareAttemptForConsumeSkipsTerminalStates(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 13, 22, 5, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID + 10
	viewerIDs = viewerIDs[:1]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(time.Second))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume(first) error = %v", err)
	}
	if !shouldProcess || record == nil {
		t.Fatalf("expected first prepare to claim, got shouldProcess=%t record=%+v", shouldProcess, record)
	}
	if err := store.FinalizeSuccess(ctx, record, now.Add(2*time.Second)); err != nil {
		t.Fatalf("FinalizeSuccess() error = %v", err)
	}

	record, shouldProcess, err = store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume(after success) error = %v", err)
	}
	if shouldProcess {
		t.Fatalf("expected success state to skip processing, got shouldProcess=true record=%+v", record)
	}
	if record == nil || record.State != rush.AttemptStateSuccess {
		t.Fatalf("expected success record, got %+v", record)
	}
}

func TestRefreshProcessingLeaseDoesNotReviveAttemptWithoutTTL(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	if svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 18, 10, 7, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	showTimeID := programID + 202
	viewerIDs = viewerIDs[:1]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(time.Millisecond))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume() error = %v", err)
	}
	if !shouldProcess || record == nil || record.State != rush.AttemptStateProcessing {
		t.Fatalf("expected processing record, got shouldProcess=%t record=%+v", shouldProcess, record)
	}

	attemptKey := rushTestScopedKey(prefix, showTimeID, "attempt", orderNumber)
	if _, err := svcCtx.Redis.PersistCtx(ctx, attemptKey); err != nil {
		t.Fatalf("PersistCtx(attempt) error = %v", err)
	}

	ok, err := store.RefreshProcessingLease(ctx, showTimeID, orderNumber, now.Add(2*time.Millisecond))
	if err != nil {
		t.Fatalf("RefreshProcessingLease() error = %v", err)
	}
	if ok {
		t.Fatalf("processing attempt without TTL must not be revived")
	}
}

var rushAttemptStoreTestSequence atomic.Int64

func nextRushAttemptStoreTestIDs() (userID, programID, ticketCategoryID int64, viewerIDs []int64, orderNumbers []int64) {
	seed := rushAttemptStoreTestSequence.Add(1)
	userID = 410000 + seed
	programID = 510000 + seed
	ticketCategoryID = 610000 + seed
	viewerIDs = []int64{
		710000 + seed*10 + 1,
		710000 + seed*10 + 2,
		710000 + seed*10 + 3,
	}
	orderNumbers = []int64{
		910000 + seed*10 + 1,
		910000 + seed*10 + 2,
	}
	return
}

func rushTestScopedKey(prefix string, showTimeID int64, kind string, entityID int64) string {
	if kind == "attempt" {
		return fmt.Sprintf("%s:attempt:{st:%d}:%d", prefix, showTimeID, entityID)
	}
	return fmt.Sprintf("%s:%s:{st:%d}", prefix, kind, showTimeID)
}

func rushTestHashField(entityID int64) string {
	return strconv.FormatInt(entityID, 10)
}
