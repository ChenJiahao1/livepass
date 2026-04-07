package integration_test

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/rush"
)

func TestAdmissionReturnsSameOrderNumberForSameFingerprint(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	store := svcCtx.AttemptStore
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 30, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	viewerIDs = viewerIDs[:2]
	fingerprint := rush.BuildTokenFingerprint(userID, programID, ticketCategoryID, viewerIDs, "express", "paper")

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
		CommitCutoffAt:   now.Add(10 * time.Second),
		UserDeadlineAt:   now.Add(15 * time.Second),
		Now:              now,
	})
	if err != nil {
		t.Fatalf("Admit(first) error = %v", err)
	}
	if first.Decision != rush.AdmitDecisionAccepted || first.OrderNumber != orderNumbers[0] {
		t.Fatalf("unexpected first admission result: %+v", first)
	}

	second, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[1],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		TokenFingerprint: fingerprint,
		CommitCutoffAt:   now.Add(10 * time.Second),
		UserDeadlineAt:   now.Add(15 * time.Second),
		Now:              now.Add(200 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Admit(second) error = %v", err)
	}
	if second.Decision != rush.AdmitDecisionReused || second.OrderNumber != orderNumbers[0] {
		t.Fatalf("unexpected second admission result: %+v", second)
	}

	record, err := store.Get(ctx, orderNumbers[0])
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if record.State != rush.AttemptStatePendingPublish || record.TokenFingerprint != fingerprint {
		t.Fatalf("unexpected attempt record: %+v", record)
	}

	if _, err := store.Get(ctx, orderNumbers[1]); err == nil || err.Error() != xerr.ErrOrderNotFound.Error() {
		t.Fatalf("expected order 991002 to stay absent, got err=%v", err)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("unexpected quota snapshot: ok=%t available=%d", ok, available)
	}
}

func TestAdmissionRejectsDifferentFingerprintWhenUserInflight(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	store := svcCtx.AttemptStore
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 31, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	viewerIDs = viewerIDs[:1]

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 5); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	firstFingerprint := rush.BuildTokenFingerprint(userID, programID, ticketCategoryID, viewerIDs, "express", "paper")
	first, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		TokenFingerprint: firstFingerprint,
		CommitCutoffAt:   now.Add(10 * time.Second),
		UserDeadlineAt:   now.Add(15 * time.Second),
		Now:              now,
	})
	if err != nil {
		t.Fatalf("Admit(first) error = %v", err)
	}
	if first.Decision != rush.AdmitDecisionAccepted {
		t.Fatalf("unexpected first admission result: %+v", first)
	}

	second, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[1],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		TokenFingerprint: firstFingerprint + "-different",
		CommitCutoffAt:   now.Add(10 * time.Second),
		UserDeadlineAt:   now.Add(15 * time.Second),
		Now:              now.Add(100 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Admit(second) error = %v", err)
	}
	if second.Decision != rush.AdmitDecisionRejected || second.RejectCode != rush.AdmitRejectUserInflightConflict {
		t.Fatalf("unexpected second admission result: %+v", second)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("unexpected quota snapshot after reject: ok=%t available=%d", ok, available)
	}
}

func TestAdmissionRejectsUserAndViewerWhenAlreadyActive(t *testing.T) {
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
	now := time.Date(2026, 4, 5, 18, 32, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	showTimeID := programID + 101
	generation := rush.BuildRushGeneration(showTimeID)
	viewerIDs = viewerIDs[:1]

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 5); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	firstFingerprint := rush.BuildTokenFingerprint(userID, showTimeID, ticketCategoryID, viewerIDs, "express", "paper", generation)
	first, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		Generation:       generation,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		TokenFingerprint: firstFingerprint,
		CommitCutoffAt:   now.Add(10 * time.Second),
		UserDeadlineAt:   now.Add(15 * time.Second),
		Now:              now,
	})
	if err != nil {
		t.Fatalf("Admit(first) error = %v", err)
	}
	if first.Decision != rush.AdmitDecisionAccepted {
		t.Fatalf("unexpected first admission result: %+v", first)
	}

	record, err := store.Get(ctx, orderNumbers[0])
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := store.CommitProjection(ctx, record, []int64{900001}, now); err != nil {
		t.Fatalf("CommitProjection() error = %v", err)
	}

	userConflict, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[1],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        []int64{viewerIDs[0] + 10},
		TicketCount:      1,
		Generation:       generation,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		TokenFingerprint: rush.BuildTokenFingerprint(userID, showTimeID, ticketCategoryID, []int64{viewerIDs[0] + 10}, "express", "paper", generation),
		CommitCutoffAt:   now.Add(20 * time.Second),
		UserDeadlineAt:   now.Add(25 * time.Second),
		Now:              now.Add(100 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Admit(userConflict) error = %v", err)
	}
	if userConflict.Decision != rush.AdmitDecisionRejected || userConflict.RejectCode != rush.AdmitRejectUserInflightConflict {
		t.Fatalf("unexpected user conflict result: %+v", userConflict)
	}

	viewerConflict, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[1] + 1,
		UserID:           userID + 1,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		Generation:       generation,
		SaleWindowEndAt:  now.Add(30 * time.Minute),
		ShowEndAt:        now.Add(2 * time.Hour),
		TokenFingerprint: rush.BuildTokenFingerprint(userID+1, showTimeID, ticketCategoryID, viewerIDs, "express", "paper", generation),
		CommitCutoffAt:   now.Add(20 * time.Second),
		UserDeadlineAt:   now.Add(25 * time.Second),
		Now:              now.Add(200 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Admit(viewerConflict) error = %v", err)
	}
	if viewerConflict.Decision != rush.AdmitDecisionRejected || viewerConflict.RejectCode != rush.AdmitRejectViewerInflightConflict {
		t.Fatalf("unexpected viewer conflict result: %+v", viewerConflict)
	}
}

func TestCommitAndReleaseProjectionManageSeatOccupiedAndTTL(t *testing.T) {
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
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	showTimeID := programID + 202
	generation := rush.BuildRushGeneration(showTimeID)
	viewerIDs = viewerIDs[:2]
	saleWindowEndAt := now.Add(30 * time.Minute)
	showEndAt := now.Add(2 * time.Hour)
	seatIDs := []int64{710001, 710002}

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 4); err != nil {
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
		TicketCount:      int64(len(viewerIDs)),
		Generation:       generation,
		SaleWindowEndAt:  saleWindowEndAt,
		ShowEndAt:        showEndAt,
		TokenFingerprint: rush.BuildTokenFingerprint(userID, showTimeID, ticketCategoryID, viewerIDs, "express", "paper", generation),
		CommitCutoffAt:   now.Add(10 * time.Second),
		UserDeadlineAt:   now.Add(15 * time.Second),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	attemptKey := fmt.Sprintf("%s:{st:%d:g:%s}:attempt:%d", prefix, showTimeID, generation, orderNumber)
	userActiveRedisKey := fmt.Sprintf("%s:{st:%d:g:%s}:user_active:%d", prefix, showTimeID, generation, userID)
	viewerActiveRedisKey := fmt.Sprintf("%s:{st:%d:g:%s}:viewer_active:%d", prefix, showTimeID, generation, viewerIDs[0])
	seatOccupiedRedisKey := fmt.Sprintf("%s:{st:%d:g:%s}:seat_occupied:%d", prefix, showTimeID, generation, orderNumber)

	attemptTTL, err := svcCtx.Redis.TtlCtx(ctx, attemptKey)
	if err != nil {
		t.Fatalf("TtlCtx(attempt) error = %v", err)
	}
	expectedAttemptTTL := int(saleWindowEndAt.Sub(now).Seconds()) + 2*60*60
	if attemptTTL < expectedAttemptTTL-5 {
		t.Fatalf("expected attempt ttl >= %d, got %d", expectedAttemptTTL-5, attemptTTL)
	}

	record, err := store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := store.CommitProjection(ctx, record, seatIDs, now); err != nil {
		t.Fatalf("CommitProjection() error = %v", err)
	}

	userActiveOrderNo, err := svcCtx.Redis.GetCtx(ctx, userActiveRedisKey)
	if err != nil {
		t.Fatalf("GetCtx(user_active) error = %v", err)
	}
	if userActiveOrderNo != strconv.FormatInt(orderNumber, 10) {
		t.Fatalf("expected user_active order %d, got %s", orderNumber, userActiveOrderNo)
	}
	viewerActiveOrderNo, err := svcCtx.Redis.GetCtx(ctx, viewerActiveRedisKey)
	if err != nil {
		t.Fatalf("GetCtx(viewer_active) error = %v", err)
	}
	if viewerActiveOrderNo != strconv.FormatInt(orderNumber, 10) {
		t.Fatalf("expected viewer_active order %d, got %s", orderNumber, viewerActiveOrderNo)
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

	expectedActiveTTL := int(showEndAt.Sub(now).Seconds()) + 7*24*60*60
	userActiveTTL, err := svcCtx.Redis.TtlCtx(ctx, userActiveRedisKey)
	if err != nil {
		t.Fatalf("TtlCtx(user_active) error = %v", err)
	}
	if userActiveTTL < expectedActiveTTL-5 {
		t.Fatalf("expected user_active ttl >= %d, got %d", expectedActiveTTL-5, userActiveTTL)
	}
	seatOccupiedTTL, err := svcCtx.Redis.TtlCtx(ctx, seatOccupiedRedisKey)
	if err != nil {
		t.Fatalf("TtlCtx(seat_occupied) error = %v", err)
	}
	if seatOccupiedTTL < expectedActiveTTL-5 {
		t.Fatalf("expected seat_occupied ttl >= %d, got %d", expectedActiveTTL-5, seatOccupiedTTL)
	}

	if err := store.ReleaseClosedOrderProjection(ctx, record, now.Add(time.Minute)); err != nil {
		t.Fatalf("ReleaseClosedOrderProjection() error = %v", err)
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
		t.Fatalf("expected quota restored to 4, got ok=%t available=%d", ok, available)
	}
}
