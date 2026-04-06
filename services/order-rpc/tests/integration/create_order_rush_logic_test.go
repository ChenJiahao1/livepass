package integration_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	orderevent "damai-go/services/order-rpc/internal/event"
	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
)

func TestCreateOrderReturnsPreAllocatedOrderNumberAndDoesNotFreezeSeatsInline(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	claims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    viewerIDs[:2],
		TicketCount:      2,
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	token := mustIssueRushPurchaseToken(t, svcCtx, claims)
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, claims.ProgramID, claims.TicketCategoryID, 6); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	resp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        claims.UserID,
		PurchaseToken: token,
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if resp.GetOrderNumber() != claims.OrderNumber {
		t.Fatalf("expected order number %d, got %d", claims.OrderNumber, resp.GetOrderNumber())
	}
	if producer.sendCalls != 1 {
		t.Fatalf("expected producer send once, got %d", producer.sendCalls)
	}
	if producer.lastKey != strconv.FormatInt(claims.OrderNumber, 10) {
		t.Fatalf("expected kafka key %d, got %s", claims.OrderNumber, producer.lastKey)
	}
	if programRPC.lastAutoAssignAndFreezeSeatsReq != nil {
		t.Fatalf("expected hot path to skip inline seat freeze, got %+v", programRPC.lastAutoAssignAndFreezeSeatsReq)
	}
	if programRPC.lastGetProgramPreorderReq != nil {
		t.Fatalf("expected hot path to skip program rpc lookup, got %+v", programRPC.lastGetProgramPreorderReq)
	}
	if userRPC.lastGetUserAndTicketUserListReq != nil {
		t.Fatalf("expected hot path to skip user rpc lookup, got %+v", userRPC.lastGetUserAndTicketUserListReq)
	}

	event, err := orderevent.UnmarshalOrderCreateEvent(producer.lastBody)
	if err != nil {
		t.Fatalf("UnmarshalOrderCreateEvent() error = %v", err)
	}
	if event.OrderNumber != claims.OrderNumber || event.ProgramID != claims.ProgramID || event.TicketCount != claims.TicketCount {
		t.Fatalf("unexpected event body: %+v", event)
	}

	record, err := svcCtx.AttemptStore.Get(ctx, claims.OrderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateQueued {
		t.Fatalf("expected queued attempt state, got %+v", record)
	}
}

func TestCreateOrderSchedulesVerifyTaskAtUserDeadline(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	asyncCloseClient, ok := svcCtx.AsyncCloseClient.(*fakeAsyncCloseClient)
	if !ok {
		t.Fatalf("expected fake async close client, got %T", svcCtx.AsyncCloseClient)
	}

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	claims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    viewerIDs[:1],
		TicketCount:      1,
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	token := mustIssueRushPurchaseToken(t, svcCtx, claims)
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, claims.ProgramID, claims.TicketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	resp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        claims.UserID,
		PurchaseToken: token,
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if resp.GetOrderNumber() != claims.OrderNumber {
		t.Fatalf("expected order number %d, got %d", claims.OrderNumber, resp.GetOrderNumber())
	}
	if asyncCloseClient.verifyEnqueueCalls != 1 {
		t.Fatalf("expected verify task enqueue once, got %d", asyncCloseClient.verifyEnqueueCalls)
	}
	if asyncCloseClient.verifyLastOrderNumber != claims.OrderNumber {
		t.Fatalf("expected verify task order number %d, got %d", claims.OrderNumber, asyncCloseClient.verifyLastOrderNumber)
	}
	if asyncCloseClient.verifyLastProgramID != claims.ProgramID {
		t.Fatalf("expected verify task program id %d, got %d", claims.ProgramID, asyncCloseClient.verifyLastProgramID)
	}

	record, err := svcCtx.AttemptStore.Get(ctx, claims.OrderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if !asyncCloseClient.verifyLastDueAt.Equal(record.UserDeadlineAt) {
		t.Fatalf("expected verify dueAt %s, got %s", record.UserDeadlineAt, asyncCloseClient.verifyLastDueAt)
	}
}

func TestCreateOrderReturnsExistingOrderNumberForSameTokenFingerprint(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	baseClaims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    viewerIDs[:2],
		TicketCount:      2,
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	secondClaims := baseClaims
	secondClaims.OrderNumber = orderNumbers[1]

	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, baseClaims.ProgramID, baseClaims.TicketCategoryID, 6); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	firstResp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        baseClaims.UserID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, baseClaims),
	})
	if err != nil {
		t.Fatalf("first CreateOrder() error = %v", err)
	}
	secondResp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        secondClaims.UserID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, secondClaims),
	})
	if err != nil {
		t.Fatalf("second CreateOrder() error = %v", err)
	}

	if firstResp.GetOrderNumber() != baseClaims.OrderNumber {
		t.Fatalf("expected first order number %d, got %d", baseClaims.OrderNumber, firstResp.GetOrderNumber())
	}
	if secondResp.GetOrderNumber() != firstResp.GetOrderNumber() {
		t.Fatalf("expected reused order number %d, got %d", firstResp.GetOrderNumber(), secondResp.GetOrderNumber())
	}
	if producer.sendCalls != 1 {
		t.Fatalf("expected kafka publish once for same fingerprint, got %d", producer.sendCalls)
	}
	if _, err := svcCtx.AttemptStore.Get(ctx, secondClaims.OrderNumber); err == nil {
		t.Fatalf("expected no second attempt record for reused fingerprint")
	}
}

func mustIssueRushPurchaseToken(t *testing.T, svcCtx *svc.ServiceContext, claims rush.PurchaseTokenClaims) string {
	t.Helper()

	token, err := svcCtx.PurchaseTokenCodec.Issue(claims)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	return token
}

func nextRushTestIDs() (userID, programID, ticketCategoryID int64, viewerIDs []int64, orderNumbers []int64) {
	base := time.Now().UnixNano() % 1_000_000
	if base < 100_000 {
		base += 100_000
	}

	userID = 30_000 + base
	programID = 40_000 + base
	ticketCategoryID = 50_000 + base
	viewerIDs = []int64{
		60_000 + base,
		60_001 + base,
		60_002 + base,
	}
	orderNumbers = []int64{
		900_000_000_000 + base,
		900_001_000_000 + base,
	}

	return userID, programID, ticketCategoryID, viewerIDs, orderNumbers
}
