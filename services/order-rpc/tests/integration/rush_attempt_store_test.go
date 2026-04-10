package integration_test

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"damai-go/services/order-rpc/internal/rush"
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

	prefix := fmt.Sprintf("damai-go:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
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
		Generation:       generation,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, showTimeID, ticketCategoryID, viewerIDs, "express", "paper", generation),
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

	prefix := fmt.Sprintf("damai-go:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
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
		TicketCount:      2,
		Generation:       generation,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, showTimeID, ticketCategoryID, viewerIDs, "express", "paper", generation),
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

	prefix := fmt.Sprintf("damai-go:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
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
		Generation:       generation,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, showTimeID, ticketCategoryID, viewerIDs, "express", "paper", generation),
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
