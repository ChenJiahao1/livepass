package integration_test

import (
	"context"
	"testing"
	"time"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/pb"
	"damai-go/services/order-rpc/repository"
)

func TestPollKeepsProcessingWhenAttemptIsNotFinalAndDBIsMissing(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	store := svcCtx.AttemptStore
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Now()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	viewerIDs = viewerIDs[:1]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 2); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
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

	svcCtx.OrderRepository = nil

	resp, err := logicpkg.NewPollOrderProgressLogic(ctx, svcCtx).PollOrderProgress(&pb.PollOrderProgressReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("PollOrderProgress() error = %v", err)
	}
	if resp.GetOrderNumber() != orderNumber || resp.GetOrderStatus() != rush.PollOrderStatusProcessing || resp.GetDone() {
		t.Fatalf("unexpected poll response: %+v", resp)
	}
}

func TestPollKeepsProcessingWhenAttemptExistsEvenIfDBOrderAlreadyExists(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	store := svcCtx.AttemptStore
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Now()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	viewerIDs = viewerIDs[:1]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 2); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, programID, ticketCategoryID, viewerIDs, "express", "paper"),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	err := svcCtx.OrderRepository.TransactByOrderNumber(ctx, orderNumber, func(txCtx context.Context, tx repository.OrderTx) error {
		return tx.InsertOrder(txCtx, &model.DOrder{
			Id:                      1,
			OrderNumber:             orderNumber,
			ProgramId:               programID,
			ShowTimeId:              programID,
			ProgramTitle:            "测试演出",
			ProgramItemPicture:      "",
			ProgramPlace:            "测试场馆",
			ProgramShowTime:         now.Add(2 * time.Hour),
			ProgramPermitChooseSeat: 0,
			UserId:                  userID,
			DistributionMode:        "express",
			TakeTicketMode:          "paper",
			TicketCount:             1,
			OrderPrice:              299,
			OrderStatus:             1,
			FreezeToken:             "freeze-poll-success",
			OrderExpireTime:         now.Add(15 * time.Minute),
			CreateOrderTime:         now,
			CreateTime:              now,
			EditTime:                now,
			Status:                  1,
		})
	})
	if err != nil {
		t.Fatalf("insert order error = %v", err)
	}

	resp, err := logicpkg.NewPollOrderProgressLogic(ctx, svcCtx).PollOrderProgress(&pb.PollOrderProgressReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("PollOrderProgress() error = %v", err)
	}
	if resp.GetOrderStatus() != rush.PollOrderStatusProcessing || resp.GetDone() {
		t.Fatalf("expected attempt projection to keep processing, got %+v", resp)
	}
	if resp.GetReasonCode() != "" {
		t.Fatalf("expected processing poll response to have empty reasonCode, got %+v", resp)
	}
}

func TestPollOrderProgressReturnsReasonCodeWhenAttemptFailed(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	store := svcCtx.AttemptStore
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Now()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	viewerIDs = viewerIDs[:1]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 2); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, programID, ticketCategoryID, viewerIDs, "express", "paper"),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	if err := store.MarkQueued(ctx, orderNumber, now); err != nil {
		t.Fatalf("MarkQueued() error = %v", err)
	}
	if _, _, err := store.ClaimProcessing(ctx, orderNumber, now.Add(time.Millisecond)); err != nil {
		t.Fatalf("ClaimProcessing() error = %v", err)
	}
	record, err := store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := store.Release(ctx, record, rush.AttemptReasonAlreadyHasActiveOrder, now.Add(2*time.Millisecond)); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	resp, err := logicpkg.NewPollOrderProgressLogic(ctx, svcCtx).PollOrderProgress(&pb.PollOrderProgressReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("PollOrderProgress() error = %v", err)
	}
	if resp.GetOrderStatus() != rush.PollOrderStatusFailed || !resp.GetDone() {
		t.Fatalf("expected failed poll response, got %+v", resp)
	}
	if resp.GetReasonCode() != rush.AttemptReasonAlreadyHasActiveOrder {
		t.Fatalf("expected reasonCode %s, got %+v", rush.AttemptReasonAlreadyHasActiveOrder, resp)
	}
}

func TestPollReturnsSuccessWhenAttemptMissingButDBOrderExists(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	ctx := context.Background()
	now := time.Now()
	userID, programID, _, _, orderNumbers := nextRushTestIDs()
	orderNumber := orderNumbers[0]

	err := svcCtx.OrderRepository.TransactByOrderNumber(ctx, orderNumber, func(txCtx context.Context, tx repository.OrderTx) error {
		return tx.InsertOrder(txCtx, &model.DOrder{
			Id:                      1,
			OrderNumber:             orderNumber,
			ProgramId:               programID,
			ShowTimeId:              programID,
			ProgramTitle:            "测试演出",
			ProgramPlace:            "测试场馆",
			ProgramShowTime:         now.Add(2 * time.Hour),
			ProgramPermitChooseSeat: 0,
			UserId:                  userID,
			DistributionMode:        "express",
			TakeTicketMode:          "paper",
			TicketCount:             1,
			OrderPrice:              299,
			OrderStatus:             1,
			FreezeToken:             "freeze-poll-attempt-miss",
			OrderExpireTime:         now.Add(15 * time.Minute),
			CreateOrderTime:         now,
			CreateTime:              now,
			EditTime:                now,
			Status:                  1,
		})
	})
	if err != nil {
		t.Fatalf("insert order error = %v", err)
	}

	resp, err := logicpkg.NewPollOrderProgressLogic(ctx, svcCtx).PollOrderProgress(&pb.PollOrderProgressReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("PollOrderProgress() error = %v", err)
	}
	if resp.GetOrderStatus() != rush.PollOrderStatusSuccess || !resp.GetDone() {
		t.Fatalf("expected DB fallback to success when attempt missing, got %+v", resp)
	}
}

func TestPollReturnsFailedWhenAttemptMissingAndDBOrderMissing(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	ctx := context.Background()
	userID, _, _, _, orderNumbers := nextRushTestIDs()
	orderNumber := orderNumbers[0]

	resp, err := logicpkg.NewPollOrderProgressLogic(ctx, svcCtx).PollOrderProgress(&pb.PollOrderProgressReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("PollOrderProgress() error = %v", err)
	}
	if resp.GetOrderStatus() != rush.PollOrderStatusFailed || !resp.GetDone() {
		t.Fatalf("expected failed when attempt/db both missing, got %+v", resp)
	}
	if resp.GetReasonCode() != "" {
		t.Fatalf("expected empty reasonCode when attempt/db both missing, got %+v", resp)
	}
}
