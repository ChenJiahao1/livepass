package integration_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	logicpkg "livepass/services/order-rpc/internal/logic"
	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/pb"
	"livepass/services/order-rpc/repository"
)

func TestGetOrderCacheReturnsOrderNumberWhenAttemptProcessing(t *testing.T) {
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
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	resp, err := logicpkg.NewGetOrderCacheLogic(ctx, svcCtx).GetOrderCache(&pb.GetOrderCacheReq{
		OrderNumber: orderNumber,
		ShowTimeId:  programID,
	})
	if err != nil {
		t.Fatalf("GetOrderCache() error = %v", err)
	}
	if resp.GetCache() != strconv.FormatInt(orderNumber, 10) {
		t.Fatalf("expected processing projection cache to be orderNumber, got %+v", resp)
	}
}

func TestGetOrderCacheReturnsEmptyWhenAttemptMissingButDBOrderExists(t *testing.T) {
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
			FreezeToken:             "freeze-cache-attempt-miss",
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

	l := logicpkg.NewGetOrderCacheLogic(ctx, svcCtx)
	resp, err := l.GetOrderCache(&pb.GetOrderCacheReq{
		OrderNumber: orderNumber,
		ShowTimeId:  programID,
	})
	if err != nil {
		t.Fatalf("GetOrderCache() error = %v", err)
	}
	if resp.GetCache() != "" {
		t.Fatalf("expected empty cache when attempt missing and DB hit, got %+v", resp)
	}
}

func TestGetOrderCacheReturnsEmptyWhenAttemptAndDBMissing(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	ctx := context.Background()
	_, programID, _, _, orderNumbers := nextRushTestIDs()
	orderNumber := orderNumbers[0]

	l := logicpkg.NewGetOrderCacheLogic(ctx, svcCtx)
	resp, err := l.GetOrderCache(&pb.GetOrderCacheReq{
		OrderNumber: orderNumber,
		ShowTimeId:  programID,
	})
	if err != nil {
		t.Fatalf("GetOrderCache() error = %v", err)
	}
	if resp.GetCache() != "" {
		t.Fatalf("expected empty cache when attempt/db both missing, got %+v", resp)
	}
}
