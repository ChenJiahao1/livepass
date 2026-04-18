package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	orderevent "livepass/services/order-rpc/internal/event"
	logicpkg "livepass/services/order-rpc/internal/logic"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateOrderRushReturnsPreAllocatedOrderNumberAndDoesNotFreezeSeatsInline(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	showTimeID := programID + 101
	claims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    viewerIDs[:2],
		TicketCount:      2,
		SaleWindowEndAt:  time.Now().Add(30 * time.Minute).Unix(),
		ShowEndAt:        time.Now().Add(2 * time.Hour).Unix(),
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	token := mustIssueRushPurchaseToken(t, svcCtx, claims)
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, claims.ShowTimeID, claims.TicketCategoryID, 6); err != nil {
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
	waitOrderCreateSendCalls(t, producer, 1)
	if producer.lastKey != fmt.Sprintf("%d", claims.OrderNumber) {
		t.Fatalf("expected kafka partition key %d, got %s", claims.OrderNumber, producer.lastKey)
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
	if event.OrderNumber != claims.OrderNumber || event.ProgramID != claims.ProgramID || event.ShowTimeID != claims.ShowTimeID || event.TicketCount != claims.TicketCount {
		t.Fatalf("unexpected event body: %+v", event)
	}
	if event.SaleWindowEndAt == "" || event.ShowEndAt == "" {
		t.Fatalf("expected show time window fields without generation, got %+v", event)
	}
	if strings.Contains(string(producer.lastBody), `"generation"`) {
		t.Fatalf("expected new event payload without generation, got %s", string(producer.lastBody))
	}

	record, err := svcCtx.AttemptStore.Get(ctx, claims.OrderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateAccepted {
		t.Fatalf("expected accepted attempt state, got %+v", record)
	}
}

func TestCreateOrderRushDoesNotEnqueueVerifyTaskAfterAdmission(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	showTimeID := programID + 102
	claims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    viewerIDs[:1],
		TicketCount:      1,
		SaleWindowEndAt:  time.Now().Add(30 * time.Minute).Unix(),
		ShowEndAt:        time.Now().Add(2 * time.Hour).Unix(),
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	token := mustIssueRushPurchaseToken(t, svcCtx, claims)
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, claims.ShowTimeID, claims.TicketCategoryID, 4); err != nil {
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
	record, err := svcCtx.AttemptStore.Get(ctx, claims.OrderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateAccepted {
		t.Fatalf("expected accepted attempt state, got %+v", record)
	}
}

func TestCreateOrderRushReturnsExistingOrderNumberForSameTokenFingerprint(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	showTimeID := programID + 103
	baseClaims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    viewerIDs[:2],
		TicketCount:      2,
		SaleWindowEndAt:  time.Now().Add(30 * time.Minute).Unix(),
		ShowEndAt:        time.Now().Add(2 * time.Hour).Unix(),
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	secondClaims := baseClaims
	secondClaims.OrderNumber = orderNumbers[1]

	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, baseClaims.ShowTimeID, baseClaims.TicketCategoryID, 6); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	sameToken := mustIssueRushPurchaseToken(t, svcCtx, baseClaims)
	firstResp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        baseClaims.UserID,
		PurchaseToken: sameToken,
	})
	if err != nil {
		t.Fatalf("first CreateOrder() error = %v", err)
	}
	secondResp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        baseClaims.UserID,
		PurchaseToken: sameToken,
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
	waitOrderCreateSendCalls(t, producer, 1)
	if producer.SendCalls() != 1 {
		t.Fatalf("expected kafka publish once for same fingerprint, got %d", producer.SendCalls())
	}
	if _, err := svcCtx.AttemptStore.Get(ctx, secondClaims.OrderNumber); err == nil {
		t.Fatalf("expected no second attempt record for reused fingerprint")
	}
}

func TestCreateOrderRushCreatesNewOrderNumberAfterClosedOrderReleaseWithNewToken(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	showTimeID := programID + 105
	baseClaims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    viewerIDs[:1],
		TicketCount:      1,
		SaleWindowEndAt:  time.Now().Add(30 * time.Minute).Unix(),
		ShowEndAt:        time.Now().Add(2 * time.Hour).Unix(),
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	nextClaims := baseClaims
	nextClaims.OrderNumber = orderNumbers[1]

	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, baseClaims.ShowTimeID, baseClaims.TicketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	firstResp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        baseClaims.UserID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, baseClaims),
	})
	if err != nil {
		t.Fatalf("first CreateOrder() error = %v", err)
	}
	if firstResp.GetOrderNumber() != baseClaims.OrderNumber {
		t.Fatalf("expected first order number %d, got %d", baseClaims.OrderNumber, firstResp.GetOrderNumber())
	}

	record, err := svcCtx.AttemptStore.Get(ctx, baseClaims.OrderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get(first) error = %v", err)
	}
	claimed, epoch, err := svcCtx.AttemptStore.ClaimProcessing(ctx, baseClaims.OrderNumber, time.Now().Add(time.Millisecond))
	if err != nil {
		t.Fatalf("ClaimProcessing() error = %v", err)
	}
	if !claimed || epoch <= 0 {
		t.Fatalf("expected claim success, got claimed=%t epoch=%d", claimed, epoch)
	}
	if err := svcCtx.AttemptStore.FinalizeSuccess(ctx, record, epoch, []int64{70101}, time.Now().Add(2*time.Millisecond)); err != nil {
		t.Fatalf("FinalizeSuccess() error = %v", err)
	}

	record, err = svcCtx.AttemptStore.Get(ctx, baseClaims.OrderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get(after success) error = %v", err)
	}
	outcome, err := svcCtx.AttemptStore.FinalizeClosedOrder(ctx, record, time.Now().Add(3*time.Millisecond))
	if err != nil {
		t.Fatalf("FinalizeClosedOrder() error = %v", err)
	}
	if outcome != rush.AttemptTransitioned {
		t.Fatalf("expected closed order transition, got %s", outcome)
	}

	secondResp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        nextClaims.UserID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, nextClaims),
	})
	if err != nil {
		t.Fatalf("second CreateOrder() error = %v", err)
	}

	if secondResp.GetOrderNumber() != nextClaims.OrderNumber {
		t.Fatalf("expected new order number %d after closed release, got %d", nextClaims.OrderNumber, secondResp.GetOrderNumber())
	}
	waitOrderCreateSendCalls(t, producer, 2)
}

func TestCreateOrderFailsWhenKafkaHandoffFailsAndProducerWins(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}
	producer.sendErr = errors.New("publish handoff failed")

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	showTimeID := programID + 104
	claims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    viewerIDs[:1],
		TicketCount:      1,
		SaleWindowEndAt:  time.Now().Add(30 * time.Minute).Unix(),
		ShowEndAt:        time.Now().Add(2 * time.Hour).Unix(),
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	const initialQuota = int64(4)
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, claims.ShowTimeID, claims.TicketCategoryID, initialQuota); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	resp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        claims.UserID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, claims),
	})
	if err == nil {
		t.Fatalf("expected CreateOrder() fast fail error")
	}
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", status.Code(err))
	}
	if resp != nil {
		t.Fatalf("expected nil response when producer wins, got %+v", resp)
	}
	waitOrderCreateSendCalls(t, producer, 1)

	record, err := svcCtx.AttemptStore.Get(ctx, claims.OrderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateFailed {
		t.Fatalf("expected failed attempt state after kafka handoff failure, got %+v", record)
	}
	if record.ReasonCode == "" {
		t.Fatalf("expected failed attempt reason code")
	}
	quota, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(ctx, claims.ShowTimeID, claims.TicketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected quota key exists")
	}
	if quota != initialQuota {
		t.Fatalf("expected quota restored to %d, got %d", initialQuota, quota)
	}
}

func TestCreateOrderReturnsOrderNumberWhenKafkaHandoffFailsButConsumerAlreadyClaimed(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	showTimeID := programID + 105
	claims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    viewerIDs[:1],
		TicketCount:      1,
		SaleWindowEndAt:  time.Now().Add(30 * time.Minute).Unix(),
		ShowEndAt:        time.Now().Add(2 * time.Hour).Unix(),
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	const initialQuota = int64(4)
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, claims.ShowTimeID, claims.TicketCategoryID, initialQuota); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	var (
		claimErr error
		claimed  bool
	)
	producer.sendFunc = func(_ context.Context, _ string, _ []byte) error {
		claimed, _, claimErr = svcCtx.AttemptStore.ClaimProcessing(context.Background(), claims.OrderNumber, time.Now())
		return context.DeadlineExceeded
	}

	resp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        claims.UserID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, claims),
	})
	if claimErr != nil {
		t.Fatalf("ClaimProcessing() error = %v", claimErr)
	}
	if !claimed {
		t.Fatalf("expected consumer to claim processing before fail-before-processing")
	}
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if resp.GetOrderNumber() != claims.OrderNumber {
		t.Fatalf("expected order number %d, got %d", claims.OrderNumber, resp.GetOrderNumber())
	}
	waitOrderCreateSendCalls(t, producer, 1)

	record, err := svcCtx.AttemptStore.Get(ctx, claims.OrderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateProcessing {
		t.Fatalf("expected processing attempt state when consumer already claimed, got %+v", record)
	}
	quota, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(ctx, claims.ShowTimeID, claims.TicketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected quota key exists")
	}
	if quota != initialQuota-claims.TicketCount {
		t.Fatalf("expected quota deducted to %d, got %d", initialQuota-claims.TicketCount, quota)
	}
}

func TestCreateOrderDoesNotDoubleCompensateWhenFailBeforeProcessingRepeats(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	showTimeID := programID + 106
	claims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    viewerIDs[:1],
		TicketCount:      1,
		SaleWindowEndAt:  time.Now().Add(30 * time.Minute).Unix(),
		ShowEndAt:        time.Now().Add(2 * time.Hour).Unix(),
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	const initialQuota = int64(4)
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, claims.ShowTimeID, claims.TicketCategoryID, initialQuota); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	var preCompensateErr error
	producer.sendFunc = func(_ context.Context, _ string, _ []byte) error {
		record, err := svcCtx.AttemptStore.Get(context.Background(), claims.OrderNumber)
		if err != nil {
			preCompensateErr = err
			return errors.New("publish handoff failed")
		}
		preCompensateErr = svcCtx.AttemptStore.Release(
			context.Background(),
			record,
			"KAFKA_HANDOFF_ERROR",
			time.Now(),
		)
		return errors.New("publish handoff failed")
	}

	resp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        claims.UserID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, claims),
	})
	if preCompensateErr != nil {
		t.Fatalf("pre-compensate release error = %v", preCompensateErr)
	}
	if err == nil {
		t.Fatalf("expected CreateOrder() error when kafka handoff fails repeatedly")
	}
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", status.Code(err))
	}
	if resp != nil {
		t.Fatalf("expected nil response after repeated fail-before-processing, got %+v", resp)
	}
	waitOrderCreateSendCalls(t, producer, 1)

	record, err := svcCtx.AttemptStore.Get(ctx, claims.OrderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateFailed {
		t.Fatalf("expected failed attempt state, got %+v", record)
	}
	quota, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(ctx, claims.ShowTimeID, claims.TicketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected quota key exists")
	}
	if quota != initialQuota {
		t.Fatalf("expected quota stay at %d after repeated compensation, got %d", initialQuota, quota)
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

func waitOrderCreateSendCalls(t *testing.T, producer *fakeOrderCreateProducer, expected int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if producer.SendCalls() == expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("expected producer send calls %d, got %d", expected, producer.SendCalls())
}

func TestUnmarshalOrderCreateEventIgnoresLegacyGenerationPayload(t *testing.T) {
	body := []byte(`{"eventId":"evt-1","version":"v1","orderNumber":91001,"requestNo":"order-create-91001","occurredAt":"2026-04-05 18:00:00","userId":3001,"programId":10001,"showTimeId":20001,"ticketCategoryId":40001,"ticketUserIds":[701,702],"ticketCount":2,"generation":"g-20001","distributionMode":"express","takeTicketMode":"paper","saleWindowEndAt":"2026-04-05 18:30:00","showEndAt":"2026-04-05 20:00:00"}`)

	event, err := orderevent.UnmarshalOrderCreateEvent(body)
	if err != nil {
		t.Fatalf("UnmarshalOrderCreateEvent() error = %v", err)
	}
	if event.OrderNumber != 91001 || event.ShowTimeID != 20001 || event.TicketCount != 2 {
		t.Fatalf("expected legacy generation payload to remain decodable, got %+v", event)
	}
}
