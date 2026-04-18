package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	logicpkg "livepass/services/order-rpc/internal/logic"
	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/pb"
	"livepass/services/order-rpc/repository"
)

func TestPollKeepsProcessingWhenAttemptIsPendingOrProcessingAndDBIsMissing(t *testing.T) {
	for _, tc := range []struct {
		name             string
		moveToProcessing bool
	}{
		{name: "pending"},
		{name: "processing", moveToProcessing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
				ShowTimeID:       programID,
				TicketCategoryID: ticketCategoryID,
				ViewerIDs:        viewerIDs,
				TicketCount:      1,
				TokenFingerprint: rush.BuildTokenFingerprint(orderNumber, userID, programID, ticketCategoryID, viewerIDs, "express", "paper"),
				Now:              now,
			}); err != nil {
				t.Fatalf("Admit() error = %v", err)
			}
			if tc.moveToProcessing {
				record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, programID, orderNumber, now.Add(time.Millisecond))
				if err != nil {
					t.Fatalf("PrepareAttemptForConsume() error = %v", err)
				}
				if !shouldProcess || record == nil || record.State != rush.AttemptStateProcessing {
					t.Fatalf("expected processing attempt, shouldProcess=%t record=%+v", shouldProcess, record)
				}
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
		})
	}
}

func TestPollPrefersAttemptProjectionWhileAttemptExistsEvenIfDBOrderAlreadyExists(t *testing.T) {
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

func TestPollReturnsSuccessWhenAttemptSucceeded(t *testing.T) {
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
	record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, programID, orderNumber, now.Add(time.Millisecond))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume() error = %v", err)
	}
	if !shouldProcess || record == nil {
		t.Fatalf("PrepareAttemptForConsume() shouldProcess=%t record=%+v", shouldProcess, record)
	}
	if err := store.FinalizeSuccess(ctx, record, now.Add(2*time.Millisecond)); err != nil {
		t.Fatalf("FinalizeSuccess() error = %v", err)
	}

	svcCtx.OrderRepository = nil

	resp, err := logicpkg.NewPollOrderProgressLogic(ctx, svcCtx).PollOrderProgress(&pb.PollOrderProgressReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("PollOrderProgress() error = %v", err)
	}
	if resp.GetOrderStatus() != rush.PollOrderStatusSuccess || !resp.GetDone() {
		t.Fatalf("expected success poll response, got %+v", resp)
	}
	if resp.GetReasonCode() != "" {
		t.Fatalf("expected success poll response to have empty reasonCode, got %+v", resp)
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
	record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, programID, orderNumber, now.Add(time.Millisecond))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume() error = %v", err)
	}
	if !shouldProcess || record == nil {
		t.Fatalf("PrepareAttemptForConsume() shouldProcess=%t record=%+v", shouldProcess, record)
	}
	outcome, err := store.FinalizeFailure(ctx, record, rush.AttemptReasonAlreadyHasActiveOrder, now.Add(2*time.Millisecond))
	if err != nil {
		t.Fatalf("FinalizeFailure() error = %v", err)
	}
	if outcome != rush.AttemptTransitioned {
		t.Fatalf("FinalizeFailure() outcome = %s", outcome)
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

func TestPollReturnsErrorWhenAttemptMissingAndDBLookupTimeouts(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	baseRepo := svcCtx.OrderRepository
	svcCtx.OrderRepository = &timeoutOrderRepository{
		OrderRepository: baseRepo,
		findErr:         context.DeadlineExceeded,
	}

	ctx := context.Background()
	userID, _, _, _, orderNumbers := nextRushTestIDs()
	orderNumber := orderNumbers[0]

	resp, err := logicpkg.NewPollOrderProgressLogic(ctx, svcCtx).PollOrderProgress(&pb.PollOrderProgressReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err == nil {
		t.Fatalf("PollOrderProgress() error = nil, want timeout")
	}
	if resp != nil {
		t.Fatalf("expected nil response on DB timeout, got %+v", resp)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestPollReturnsInternalWhenAttemptMissingAndRepositoryUnavailable(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	svcCtx.OrderRepository = nil

	ctx := context.Background()
	userID, _, _, _, orderNumbers := nextRushTestIDs()
	orderNumber := orderNumbers[0]

	resp, err := logicpkg.NewPollOrderProgressLogic(ctx, svcCtx).PollOrderProgress(&pb.PollOrderProgressReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err == nil {
		t.Fatalf("PollOrderProgress() error = nil, want internal")
	}
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected internal code, got %v", err)
	}
}

type timeoutOrderRepository struct {
	repository.OrderRepository
	findErr error
}

func (r *timeoutOrderRepository) FindOrderByNumber(ctx context.Context, orderNumber int64) (*model.DOrder, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.OrderRepository.FindOrderByNumber(ctx, orderNumber)
}
