package integration_test

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"livepass/services/order-rpc/internal/rush"
)

func TestAdmitKeepsRejectAndReuseSemantics(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	store := svcCtx.AttemptStore
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 30, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	viewerIDs = viewerIDs[:2]
	fingerprint := rush.BuildTokenFingerprint(orderNumbers[0], userID, programID, ticketCategoryID, viewerIDs, "express", "paper")

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 6); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	first, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		TokenFingerprint: fingerprint,
		Now:              now,
	})
	if err != nil {
		t.Fatalf("Admit(first) error = %v", err)
	}
	if first.Decision != rush.AdmitDecisionAccepted || first.OrderNumber != orderNumbers[0] {
		t.Fatalf("unexpected first admission result: %+v", first)
	}

	reused, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[1],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		TokenFingerprint: fingerprint,
		Now:              now.Add(200 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Admit(reused) error = %v", err)
	}
	if reused.Decision != rush.AdmitDecisionReused || reused.OrderNumber != orderNumbers[0] {
		t.Fatalf("unexpected reused admission result: %+v", reused)
	}

	rejected, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[1] + 1,
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		TokenFingerprint: fingerprint + "-new",
		Now:              now.Add(400 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Admit(rejected) error = %v", err)
	}
	if rejected.Decision != rush.AdmitDecisionRejected || rejected.RejectCode != rush.AdmitRejectUserInflightConflict {
		t.Fatalf("unexpected rejected admission result: %+v", rejected)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("unexpected quota snapshot: ok=%t available=%d", ok, available)
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
	generation := rush.BuildRushGeneration(showTimeID)
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
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, showTimeID, ticketCategoryID, viewerIDs, "express", "paper"),
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

	userInflightRedisKey := fmt.Sprintf("%s:{st:%d:g:%s}:user_inflight:%d", prefix, showTimeID, generation, userID)
	viewerInflightRedisKey := fmt.Sprintf("%s:{st:%d:g:%s}:viewer_inflight:%d", prefix, showTimeID, generation, viewerIDs[0])
	userInflightExists, err := svcCtx.Redis.ExistsCtx(ctx, userInflightRedisKey)
	if err != nil {
		t.Fatalf("ExistsCtx(user_inflight) error = %v", err)
	}
	if userInflightExists {
		t.Fatalf("expected user_inflight key to be removed")
	}
	viewerInflightExists, err := svcCtx.Redis.ExistsCtx(ctx, viewerInflightRedisKey)
	if err != nil {
		t.Fatalf("ExistsCtx(viewer_inflight) error = %v", err)
	}
	if viewerInflightExists {
		t.Fatalf("expected viewer_inflight key to be removed")
	}
}

func TestRefreshProcessingLeaseRejectsOtherEpoch(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	store := svcCtx.AttemptStore
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 32, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushAttemptStoreTestIDs()
	viewerIDs = viewerIDs[:1]

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 2); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	orderNumber := orderNumbers[0]
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, programID, ticketCategoryID, viewerIDs, "express", "paper"),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	claimed, epoch, err := store.ClaimProcessing(ctx, orderNumber, now.Add(500*time.Millisecond))
	if err != nil {
		t.Fatalf("ClaimProcessing() error = %v", err)
	}
	if !claimed || epoch <= 0 {
		t.Fatalf("expected claim success with epoch, got claimed=%t epoch=%d", claimed, epoch)
	}

	ok, err := store.RefreshProcessingLease(ctx, orderNumber, epoch, now.Add(time.Second))
	if err != nil {
		t.Fatalf("RefreshProcessingLease(valid) error = %v", err)
	}
	if !ok {
		t.Fatalf("expected lease refresh with current epoch to succeed")
	}

	ok, err = store.RefreshProcessingLease(ctx, orderNumber, epoch+1, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("RefreshProcessingLease(other epoch) error = %v", err)
	}
	if ok {
		t.Fatalf("expected lease refresh with other epoch to fail")
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
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, showTimeID, ticketCategoryID, viewerIDs, "express", "paper"),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	claimed, epoch, err := store.ClaimProcessing(ctx, orderNumber, now.Add(100*time.Millisecond))
	if err != nil {
		t.Fatalf("ClaimProcessing() error = %v", err)
	}
	if !claimed || epoch <= 0 {
		t.Fatalf("expected claim success with epoch, got claimed=%t epoch=%d", claimed, epoch)
	}

	record, err := store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	outcome, err := store.FinalizeFailure(ctx, record, epoch, rush.AttemptReasonSeatExhausted, now.Add(time.Second))
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
	outcome, err = store.FinalizeFailure(ctx, record, epoch, rush.AttemptReasonSeatExhausted, now.Add(2*time.Second))
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
	generation := rush.BuildRushGeneration(showTimeID)
	viewerIDs = viewerIDs[:2]
	orderNumber := orderNumbers[0]
	seatIDs := []int64{810001, 810002}

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
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, showTimeID, ticketCategoryID, viewerIDs, "express", "paper"),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	claimed, epoch, err := store.ClaimProcessing(ctx, orderNumber, now.Add(100*time.Millisecond))
	if err != nil {
		t.Fatalf("ClaimProcessing() error = %v", err)
	}
	if !claimed || epoch <= 0 {
		t.Fatalf("expected claim success with epoch, got claimed=%t epoch=%d", claimed, epoch)
	}

	record, err := store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() before finalize success error = %v", err)
	}
	if err := store.FinalizeSuccess(ctx, record, seatIDs, now.Add(time.Second)); err != nil {
		t.Fatalf("FinalizeSuccess() error = %v", err)
	}

	record, err = store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() after finalize success error = %v", err)
	}
	if record.State != rush.AttemptStateSuccess {
		t.Fatalf("expected success state, got %+v", record)
	}

	userActiveRedisKey := fmt.Sprintf("%s:{st:%d:g:%s}:user_active:%d", prefix, showTimeID, generation, userID)
	viewerActiveRedisKey := fmt.Sprintf("%s:{st:%d:g:%s}:viewer_active:%d", prefix, showTimeID, generation, viewerIDs[0])
	seatOccupiedRedisKey := fmt.Sprintf("%s:{st:%d:g:%s}:seat_occupied:%d", prefix, showTimeID, generation, orderNumber)

	userActiveOrderNo, err := svcCtx.Redis.GetCtx(ctx, userActiveRedisKey)
	if err != nil {
		t.Fatalf("GetCtx(user_active) error = %v", err)
	}
	if userActiveOrderNo != strconv.FormatInt(orderNumber, 10) {
		t.Fatalf("expected user_active order %d, got %s", orderNumber, userActiveOrderNo)
	}
	occupiedSeats, err := svcCtx.Redis.SmembersCtx(ctx, seatOccupiedRedisKey)
	if err != nil {
		t.Fatalf("SmembersCtx(seat_occupied) error = %v", err)
	}
	sort.Strings(occupiedSeats)
	expectedOccupiedSeats := []string{strconv.FormatInt(seatIDs[0], 10), strconv.FormatInt(seatIDs[1], 10)}
	if len(occupiedSeats) != len(expectedOccupiedSeats) {
		t.Fatalf("unexpected occupied seat count: %+v", occupiedSeats)
	}
	for idx := range expectedOccupiedSeats {
		if occupiedSeats[idx] != expectedOccupiedSeats[idx] {
			t.Fatalf("unexpected occupied seats: %+v", occupiedSeats)
		}
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

	userActiveExists, err := svcCtx.Redis.ExistsCtx(ctx, userActiveRedisKey)
	if err != nil {
		t.Fatalf("ExistsCtx(user_active) error = %v", err)
	}
	if userActiveExists {
		t.Fatalf("expected user_active key to be removed")
	}
	viewerActiveExists, err := svcCtx.Redis.ExistsCtx(ctx, viewerActiveRedisKey)
	if err != nil {
		t.Fatalf("ExistsCtx(viewer_active) error = %v", err)
	}
	if viewerActiveExists {
		t.Fatalf("expected viewer_active key to be removed")
	}
	seatOccupiedExists, err := svcCtx.Redis.ExistsCtx(ctx, seatOccupiedRedisKey)
	if err != nil {
		t.Fatalf("ExistsCtx(seat_occupied) error = %v", err)
	}
	if seatOccupiedExists {
		t.Fatalf("expected seat_occupied key to be removed")
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
	targetUserInflightKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:user_inflight:%d", prefix, targetShowTimeID, targetShowTimeID, userID)
	targetViewerInflightKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:viewer_inflight:%d", prefix, targetShowTimeID, targetShowTimeID, viewerIDs[0])
	targetFingerprintKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:fingerprint:%d", prefix, targetShowTimeID, targetShowTimeID, userID)
	otherUserInflightKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:user_inflight:%d", prefix, otherShowTimeID, otherShowTimeID, userID+1)
	otherViewerInflightKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:viewer_inflight:%d", prefix, otherShowTimeID, otherShowTimeID, viewerIDs[0]+1)
	otherFingerprintKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:fingerprint:%d", prefix, otherShowTimeID, otherShowTimeID, userID+1)

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
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumbers[0], userID, targetShowTimeID, ticketCategoryID, viewerIDs, "express", "paper"),
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
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumbers[1], userID+1, otherShowTimeID, ticketCategoryID, []int64{viewerIDs[0] + 1}, "express", "paper"),
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
	if err := store.ClearFingerprintByShowTime(ctx, targetShowTimeID); err != nil {
		t.Fatalf("ClearFingerprintByShowTime() error = %v", err)
	}

	targetUserInflightExists, err := svcCtx.Redis.ExistsCtx(ctx, targetUserInflightKey)
	if err != nil {
		t.Fatalf("ExistsCtx(target user_inflight) error = %v", err)
	}
	if targetUserInflightExists {
		t.Fatalf("expected target user_inflight key to be removed")
	}
	targetViewerInflightExists, err := svcCtx.Redis.ExistsCtx(ctx, targetViewerInflightKey)
	if err != nil {
		t.Fatalf("ExistsCtx(target viewer_inflight) error = %v", err)
	}
	if targetViewerInflightExists {
		t.Fatalf("expected target viewer_inflight key to be removed")
	}
	targetFingerprintExists, err := svcCtx.Redis.ExistsCtx(ctx, targetFingerprintKey)
	if err != nil {
		t.Fatalf("ExistsCtx(target fingerprint) error = %v", err)
	}
	if targetFingerprintExists {
		t.Fatalf("expected target fingerprint key to be removed")
	}

	otherUserInflightExists, err := svcCtx.Redis.ExistsCtx(ctx, otherUserInflightKey)
	if err != nil {
		t.Fatalf("ExistsCtx(other user_inflight) error = %v", err)
	}
	if !otherUserInflightExists {
		t.Fatalf("expected other showTime user_inflight key to remain")
	}
	otherViewerInflightExists, err := svcCtx.Redis.ExistsCtx(ctx, otherViewerInflightKey)
	if err != nil {
		t.Fatalf("ExistsCtx(other viewer_inflight) error = %v", err)
	}
	if !otherViewerInflightExists {
		t.Fatalf("expected other showTime viewer_inflight key to remain")
	}
	otherFingerprintExists, err := svcCtx.Redis.ExistsCtx(ctx, otherFingerprintKey)
	if err != nil {
		t.Fatalf("ExistsCtx(other fingerprint) error = %v", err)
	}
	if !otherFingerprintExists {
		t.Fatalf("expected other showTime fingerprint key to remain")
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
	targetUserActiveKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:user_active:%d", prefix, targetShowTimeID, targetShowTimeID, 3001)
	targetViewerActiveKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:viewer_active:%d", prefix, targetShowTimeID, targetShowTimeID, 4001)
	staleTargetUserActiveKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:user_active:%d", prefix, targetShowTimeID, targetShowTimeID, 3999)
	staleTargetViewerActiveKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:viewer_active:%d", prefix, targetShowTimeID, targetShowTimeID, 4999)
	otherUserActiveKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:user_active:%d", prefix, otherShowTimeID, otherShowTimeID, 3002)
	otherViewerActiveKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:viewer_active:%d", prefix, otherShowTimeID, otherShowTimeID, 4002)
	targetQuotaKey := fmt.Sprintf("%s:{st:%d:g:g-%d}:quota:%d", prefix, targetShowTimeID, targetShowTimeID, 5001)

	if err := svcCtx.Redis.SetCtx(ctx, targetUserActiveKey, "old-order"); err != nil {
		t.Fatalf("SetCtx(target user_active) error = %v", err)
	}
	if err := svcCtx.Redis.SetCtx(ctx, targetViewerActiveKey, "old-order"); err != nil {
		t.Fatalf("SetCtx(target viewer_active) error = %v", err)
	}
	if err := svcCtx.Redis.SetCtx(ctx, staleTargetUserActiveKey, "stale-user-order"); err != nil {
		t.Fatalf("SetCtx(stale target user_active) error = %v", err)
	}
	if err := svcCtx.Redis.SetCtx(ctx, staleTargetViewerActiveKey, "stale-viewer-order"); err != nil {
		t.Fatalf("SetCtx(stale target viewer_active) error = %v", err)
	}
	if err := svcCtx.Redis.SetCtx(ctx, otherUserActiveKey, "other-order"); err != nil {
		t.Fatalf("SetCtx(other user_active) error = %v", err)
	}
	if err := svcCtx.Redis.SetCtx(ctx, otherViewerActiveKey, "other-order"); err != nil {
		t.Fatalf("SetCtx(other viewer_active) error = %v", err)
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

	targetUserActiveValue, err := svcCtx.Redis.GetCtx(ctx, targetUserActiveKey)
	if err != nil {
		t.Fatalf("GetCtx(target user_active) error = %v", err)
	}
	if targetUserActiveValue != "91001" {
		t.Fatalf("expected target user_active to be replaced, got %s", targetUserActiveValue)
	}
	targetViewerActiveValue, err := svcCtx.Redis.GetCtx(ctx, targetViewerActiveKey)
	if err != nil {
		t.Fatalf("GetCtx(target viewer_active) error = %v", err)
	}
	if targetViewerActiveValue != "91001" {
		t.Fatalf("expected target viewer_active to be replaced, got %s", targetViewerActiveValue)
	}
	staleTargetUserExists, err := svcCtx.Redis.ExistsCtx(ctx, staleTargetUserActiveKey)
	if err != nil {
		t.Fatalf("ExistsCtx(stale target user_active) error = %v", err)
	}
	if staleTargetUserExists {
		t.Fatalf("expected stale target user_active key to be removed")
	}
	staleTargetViewerExists, err := svcCtx.Redis.ExistsCtx(ctx, staleTargetViewerActiveKey)
	if err != nil {
		t.Fatalf("ExistsCtx(stale target viewer_active) error = %v", err)
	}
	if staleTargetViewerExists {
		t.Fatalf("expected stale target viewer_active key to be removed")
	}
	otherUserActiveValue, err := svcCtx.Redis.GetCtx(ctx, otherUserActiveKey)
	if err != nil {
		t.Fatalf("GetCtx(other user_active) error = %v", err)
	}
	if otherUserActiveValue != "other-order" {
		t.Fatalf("expected other showTime user_active to remain, got %s", otherUserActiveValue)
	}
	otherViewerActiveValue, err := svcCtx.Redis.GetCtx(ctx, otherViewerActiveKey)
	if err != nil {
		t.Fatalf("GetCtx(other viewer_active) error = %v", err)
	}
	if otherViewerActiveValue != "other-order" {
		t.Fatalf("expected other showTime viewer_active to remain, got %s", otherViewerActiveValue)
	}

	quotaValue, err := svcCtx.Redis.GetCtx(ctx, targetQuotaKey)
	if err != nil {
		t.Fatalf("GetCtx(target quota) error = %v", err)
	}
	if quotaValue != "9" {
		t.Fatalf("expected quota key to remain unchanged, got %s", quotaValue)
	}
}

func TestPrepareAttemptForConsumeTransitionsAcceptedToProcessingOnce(t *testing.T) {
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
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, showTimeID, ticketCategoryID, viewerIDs, "express", "paper"),
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	record, epoch, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(time.Second))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume() error = %v", err)
	}
	if !shouldProcess || epoch <= 0 {
		t.Fatalf("expected shouldProcess=true with epoch, got shouldProcess=%t epoch=%d", shouldProcess, epoch)
	}
	if record == nil || record.State != rush.AttemptStateProcessing {
		t.Fatalf("expected processing record, got %+v", record)
	}
	if record.OrderNumber != orderNumber || record.ShowTimeID != showTimeID || record.UserID != userID {
		t.Fatalf("unexpected returned record: %+v", record)
	}

	record, epoch, shouldProcess, err = store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume(second) error = %v", err)
	}
	if shouldProcess {
		t.Fatalf("expected second prepare to skip processing, got shouldProcess=true epoch=%d record=%+v", epoch, record)
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
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, showTimeID, ticketCategoryID, viewerIDs, "express", "paper"),
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	record, epoch, shouldProcess, err := store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(time.Second))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume(first) error = %v", err)
	}
	if !shouldProcess || epoch <= 0 || record == nil {
		t.Fatalf("expected first prepare to claim, got shouldProcess=%t epoch=%d record=%+v", shouldProcess, epoch, record)
	}
	if err := store.FinalizeSuccess(ctx, record, epoch, []int64{880001}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("FinalizeSuccess() error = %v", err)
	}

	record, epoch, shouldProcess, err = store.PrepareAttemptForConsume(ctx, showTimeID, orderNumber, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume(after success) error = %v", err)
	}
	if shouldProcess {
		t.Fatalf("expected success state to skip processing, got shouldProcess=true epoch=%d record=%+v", epoch, record)
	}
	if record == nil || record.State != rush.AttemptStateSuccess {
		t.Fatalf("expected success record, got %+v", record)
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
	return fmt.Sprintf("%s:{st:%d:g:%s}:%s:%d", prefix, showTimeID, rush.BuildRushGeneration(showTimeID), kind, entityID)
}
