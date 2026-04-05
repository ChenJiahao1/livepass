package integration_test

import (
	"context"
	"testing"
	"time"

	"damai-go/pkg/xerr"
	orderevent "damai-go/services/order-rpc/internal/event"
	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/pb"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateOrderConsumerPersistsOrderFromRushEvent(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}
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
		TicketUserIDs:    viewerIDs[:2],
		TicketCount:      2,
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	programRPC.getProgramPreorderResp = buildTestProgramPreorder(programID, ticketCategoryID, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		userID,
		&userrpc.TicketUserInfo{Id: viewerIDs[0], UserId: userID, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
		&userrpc.TicketUserInfo{Id: viewerIDs[1], UserId: userID, RelName: "李四", IdType: 1, IdNumber: "110101199002021234"},
	)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-consumer-rush",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 700000 + viewerIDs[0], TicketCategoryId: ticketCategoryID, RowCode: 1, ColCode: 1, Price: 299},
			{SeatId: 700000 + viewerIDs[1], TicketCategoryId: ticketCategoryID, RowCode: 1, ColCode: 2, Price: 299},
		},
	}
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, programID, ticketCategoryID, 6); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	resp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        userID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, claims),
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if producer.sendCalls != 1 {
		t.Fatalf("expected producer send once, got %d", producer.sendCalls)
	}

	err = logicpkg.NewCreateOrderConsumerLogic(ctx, svcCtx).Consume(producer.lastBody)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}

	if countShardOrderRows(t, svcCtx.Config.MySQL.DataSource) != 1 {
		t.Fatalf("expected one order row")
	}
	if countShardOrderTicketRows(t, svcCtx.Config.MySQL.DataSource) != 2 {
		t.Fatalf("expected two order ticket rows")
	}

	order, err := svcCtx.OrderRepository.FindOrderByNumber(ctx, resp.GetOrderNumber())
	if err != nil {
		t.Fatalf("FindOrderByNumber() error = %v", err)
	}
	if order.FreezeToken != "freeze-create-consumer-rush" {
		t.Fatalf("expected freeze token freeze-create-consumer-rush, got %s", order.FreezeToken)
	}
	if asyncCloseClient.verifyEnqueueCalls != 1 {
		t.Fatalf("expected verify task enqueue once from hot path, got %d", asyncCloseClient.verifyEnqueueCalls)
	}
	if asyncCloseClient.enqueueCalls != 1 {
		t.Fatalf("expected close timeout enqueue once from consumer, got %d", asyncCloseClient.enqueueCalls)
	}
	if programRPC.lastGetProgramPreorderReq == nil {
		t.Fatalf("expected consumer to load preorder snapshot")
	}
	if userRPC.lastGetUserAndTicketUserListReq == nil {
		t.Fatalf("expected consumer to load ticket users")
	}
	if programRPC.lastAutoAssignAndFreezeSeatsReq == nil {
		t.Fatalf("expected consumer to freeze seats")
	}

	record, err := svcCtx.AttemptStore.Get(ctx, resp.GetOrderNumber())
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateCommitted {
		t.Fatalf("expected attempt state committed, got %+v", record)
	}
}

func TestCreateOrderConsumerReleasesExpiredRushEventWithoutFreezingSeats(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	svcCtx.Config.Kafka.MaxMessageDelay = 5 * time.Second

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
		TicketUserIDs:    viewerIDs[:1],
		TicketCount:      1,
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, programID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	resp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        userID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, claims),
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	event, err := orderevent.UnmarshalOrderCreateEvent(producer.lastBody)
	if err != nil {
		t.Fatalf("UnmarshalOrderCreateEvent() error = %v", err)
	}
	event.OccurredAt = time.Now().Add(-time.Minute).Format("2006-01-02 15:04:05")

	if err := logicpkg.NewCreateOrderConsumerLogic(ctx, svcCtx).Consume(mustMarshalOrderCreateEvent(t, event)); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if countShardOrderRows(t, svcCtx.Config.MySQL.DataSource) != 0 {
		t.Fatalf("expected no order rows for expired event")
	}
	if programRPC.autoAssignAndFreezeSeatsCalls != 0 {
		t.Fatalf("expected expired event to skip seat freeze, got %d calls", programRPC.autoAssignAndFreezeSeatsCalls)
	}

	record, err := svcCtx.AttemptStore.Get(ctx, resp.GetOrderNumber())
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateReleased || record.ReasonCode != rush.AttemptReasonCommitCutoffExceed {
		t.Fatalf("expected released attempt with cutoff reason, got %+v", record)
	}
	available, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(ctx, programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("expected quota restored to 4, got ok=%t available=%d", ok, available)
	}
}

func TestCreateOrderConsumerTreatsExistingCommittedOrderAsIdempotent(t *testing.T) {
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
		TicketUserIDs:    viewerIDs[:1],
		TicketCount:      1,
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	programRPC.getProgramPreorderResp = buildTestProgramPreorder(programID, ticketCategoryID, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		userID,
		&userrpc.TicketUserInfo{Id: viewerIDs[0], UserId: userID, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-consumer-idempotent",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 700000 + viewerIDs[0], TicketCategoryId: ticketCategoryID, RowCode: 1, ColCode: 1, Price: 299},
		},
	}
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, programID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	if _, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        userID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, claims),
	}); err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	consumer := logicpkg.NewCreateOrderConsumerLogic(ctx, svcCtx)
	if err := consumer.Consume(producer.lastBody); err != nil {
		t.Fatalf("first Consume() error = %v", err)
	}
	if err := consumer.Consume(producer.lastBody); err != nil {
		t.Fatalf("second Consume() error = %v", err)
	}
	if countShardOrderRows(t, svcCtx.Config.MySQL.DataSource) != 1 {
		t.Fatalf("expected one order row after duplicate consume")
	}
	if programRPC.autoAssignAndFreezeSeatsCalls != 1 {
		t.Fatalf("expected seat freeze once after duplicate consume, got %d", programRPC.autoAssignAndFreezeSeatsCalls)
	}
}

func TestCreateOrderConsumerReleasesAttemptWhenSeatFreezeFails(t *testing.T) {
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
		TicketUserIDs:    viewerIDs[:1],
		TicketCount:      1,
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	programRPC.getProgramPreorderResp = buildTestProgramPreorder(programID, ticketCategoryID, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsErr = status.Error(codes.FailedPrecondition, xerr.ErrSeatInventoryInsufficient.Error())
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		userID,
		&userrpc.TicketUserInfo{Id: viewerIDs[0], UserId: userID, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, programID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	resp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        userID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, claims),
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}

	if err := logicpkg.NewCreateOrderConsumerLogic(ctx, svcCtx).Consume(producer.lastBody); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if countShardOrderRows(t, svcCtx.Config.MySQL.DataSource) != 0 {
		t.Fatalf("expected no order rows when seat freeze fails")
	}

	record, err := svcCtx.AttemptStore.Get(ctx, resp.GetOrderNumber())
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateReleased || record.ReasonCode != rush.AttemptReasonSeatExhausted {
		t.Fatalf("expected released seat-exhausted attempt, got %+v", record)
	}
	available, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(ctx, programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("expected quota restored to 4, got ok=%t available=%d", ok, available)
	}
	if programRPC.releaseSeatFreezeCalls != 0 {
		t.Fatalf("expected no release call when freeze was never created, got %d", programRPC.releaseSeatFreezeCalls)
	}
}
