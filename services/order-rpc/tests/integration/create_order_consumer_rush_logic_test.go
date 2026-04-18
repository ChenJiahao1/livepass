package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"livepass/pkg/seatfreeze"
	"livepass/pkg/xerr"
	orderevent "livepass/services/order-rpc/internal/event"
	logicpkg "livepass/services/order-rpc/internal/logic"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/pb"
	programrpc "livepass/services/program-rpc/programrpc"
	userrpc "livepass/services/user-rpc/userrpc"
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
	waitOrderCreateSendCalls(t, producer, 1)

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
	requireDelayTaskOutbox(t, svcCtx.Config.MySQL.DataSource, "d_delay_task_outbox", resp.GetOrderNumber(), "2026-12-31 19:45:00")
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
	if record.State != rush.AttemptStateSuccess {
		t.Fatalf("expected attempt state success, got %+v", record)
	}
	expectedFreezeToken := seatfreeze.FormatToken(programID, ticketCategoryID, resp.GetOrderNumber(), record.ProcessingEpoch)
	if programRPC.lastAutoAssignAndFreezeSeatsReq.GetFreezeToken() != expectedFreezeToken {
		t.Fatalf("expected freezeToken %s, got %+v", expectedFreezeToken, programRPC.lastAutoAssignAndFreezeSeatsReq)
	}
}

func TestCreateOrderConsumerIgnoresOccurredAtAgeWhenEventPayloadValid(t *testing.T) {
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
		FreezeToken: "freeze-create-consumer-occurred-at",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 700000 + viewerIDs[0], TicketCategoryId: ticketCategoryID, RowCode: 1, ColCode: 1, Price: 299},
		},
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
	waitOrderCreateSendCalls(t, producer, 1)
	event, err := orderevent.UnmarshalOrderCreateEvent(producer.lastBody)
	if err != nil {
		t.Fatalf("UnmarshalOrderCreateEvent() error = %v", err)
	}
	event.OccurredAt = time.Now().Add(-2 * time.Hour).Format("2006-01-02 15:04:05")

	if err := logicpkg.NewCreateOrderConsumerLogic(ctx, svcCtx).Consume(mustMarshalOrderCreateEvent(t, event)); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if countShardOrderRows(t, svcCtx.Config.MySQL.DataSource) != 1 {
		t.Fatalf("expected one order row for old occurredAt event")
	}
	if programRPC.autoAssignAndFreezeSeatsCalls != 1 {
		t.Fatalf("expected valid event to freeze seat, got %d calls", programRPC.autoAssignAndFreezeSeatsCalls)
	}

	record, err := svcCtx.AttemptStore.Get(ctx, resp.GetOrderNumber())
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateSuccess || record.ReasonCode != rush.AttemptReasonOrderCommitted {
		t.Fatalf("expected success attempt with committed reason, got %+v", record)
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
	waitOrderCreateSendCalls(t, producer, 1)

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
	waitOrderCreateSendCalls(t, producer, 1)

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
	if record.State != rush.AttemptStateFailed || record.ReasonCode != rush.AttemptReasonSeatExhausted {
		t.Fatalf("expected failed seat-exhausted attempt, got %+v", record)
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

func TestCreateOrderConsumerRechecksSeatFreezeByFreezeTokenAfterTimeout(t *testing.T) {
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
		ShowTimeID:       programID,
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

	freezeTokens := make([]string, 0, 2)
	programRPC.autoAssignAndFreezeSeatsFunc = func(ctx context.Context, in *programrpc.AutoAssignAndFreezeSeatsReq) (*programrpc.AutoAssignAndFreezeSeatsResp, error) {
		freezeTokens = append(freezeTokens, in.GetFreezeToken())
		if len(freezeTokens) == 1 {
			return nil, status.Error(codes.DeadlineExceeded, "freeze timeout")
		}
		return &programrpc.AutoAssignAndFreezeSeatsResp{
			FreezeToken: "freeze-create-consumer-recheck-timeout",
			ExpireTime:  "2026-12-31 19:45:00",
			Seats: []*programrpc.SeatInfo{
				{SeatId: 700000 + viewerIDs[0], TicketCategoryId: ticketCategoryID, RowCode: 1, ColCode: 1, Price: 299},
			},
		}, nil
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
	waitOrderCreateSendCalls(t, producer, 1)

	if err := logicpkg.NewCreateOrderConsumerLogic(ctx, svcCtx).Consume(producer.lastBody); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if programRPC.autoAssignAndFreezeSeatsCalls != 2 {
		t.Fatalf("expected timeout path to recheck freeze once, got %d calls", programRPC.autoAssignAndFreezeSeatsCalls)
	}
	if len(freezeTokens) != 2 || freezeTokens[0] == "" || freezeTokens[0] != freezeTokens[1] {
		t.Fatalf("expected consumer to reuse freezeToken on timeout recheck, got %v", freezeTokens)
	}

	order, err := svcCtx.OrderRepository.FindOrderByNumber(ctx, resp.GetOrderNumber())
	if err != nil {
		t.Fatalf("FindOrderByNumber() error = %v", err)
	}
	if order == nil || order.FreezeToken != "freeze-create-consumer-recheck-timeout" {
		t.Fatalf("expected timeout recheck to persist recovered freeze result, got %+v", order)
	}
}

func TestCreateOrderConsumerRefreshesLeaseDuringSlowProcessing(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	svcCtx.Config.RushOrder.InFlightTTL = 300 * time.Millisecond
	svcCtx.AttemptStore = rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	claims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       programID,
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
	programRPC.autoAssignAndFreezeSeatsFunc = func(ctx context.Context, in *programrpc.AutoAssignAndFreezeSeatsReq) (*programrpc.AutoAssignAndFreezeSeatsResp, error) {
		time.Sleep(450 * time.Millisecond)
		return &programrpc.AutoAssignAndFreezeSeatsResp{
			FreezeToken: "freeze-create-consumer-lease-refresh",
			ExpireTime:  "2026-12-31 19:45:00",
			Seats: []*programrpc.SeatInfo{
				{SeatId: 700000 + viewerIDs[0], TicketCategoryId: ticketCategoryID, RowCode: 1, ColCode: 1, Price: 299},
			},
		}, nil
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
	waitOrderCreateSendCalls(t, producer, 1)

	done := make(chan error, 1)
	go func() {
		done <- logicpkg.NewCreateOrderConsumerLogic(ctx, svcCtx).Consume(producer.lastBody)
	}()

	time.Sleep(350 * time.Millisecond)
	record, err := svcCtx.AttemptStore.Get(ctx, resp.GetOrderNumber())
	if err != nil {
		t.Fatalf("AttemptStore.Get() during slow processing error = %v", err)
	}
	if record.State != rush.AttemptStateProcessing {
		t.Fatalf("expected slow consumer to keep attempt processing, got %+v", record)
	}

	if err := <-done; err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	record, err = svcCtx.AttemptStore.Get(ctx, resp.GetOrderNumber())
	if err != nil {
		t.Fatalf("AttemptStore.Get() after consume error = %v", err)
	}
	if record.State != rush.AttemptStateSuccess {
		t.Fatalf("expected slow consumer to finish with success, got %+v", record)
	}
}

func TestCreateOrderConsumerStopsFinalizeWhenLeaseLost(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}

	prefix := fmt.Sprintf("livepass:test:order:rush:%s:%d", t.Name(), time.Now().UnixNano())
	svcCtx.Config.RushOrder.InFlightTTL = 300 * time.Millisecond
	svcCtx.AttemptStore = rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})

	ctx := context.Background()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	orderNumber := orderNumbers[0]
	claims := rush.PurchaseTokenClaims{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       programID,
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
	attemptKey := fmt.Sprintf("%s:{st:%d}:attempt:%d", prefix, programID, orderNumber)
	programRPC.autoAssignAndFreezeSeatsFunc = func(ctx context.Context, in *programrpc.AutoAssignAndFreezeSeatsReq) (*programrpc.AutoAssignAndFreezeSeatsResp, error) {
		if _, err := svcCtx.Redis.DelCtx(ctx, attemptKey); err != nil {
			t.Fatalf("DelCtx() error = %v", err)
		}
		time.Sleep(250 * time.Millisecond)
		return &programrpc.AutoAssignAndFreezeSeatsResp{
			FreezeToken: "freeze-create-consumer-lease-lost",
			ExpireTime:  "2026-12-31 19:45:00",
			Seats: []*programrpc.SeatInfo{
				{SeatId: 700000 + viewerIDs[0], TicketCategoryId: ticketCategoryID, RowCode: 1, ColCode: 1, Price: 299},
			},
		}, nil
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
	waitOrderCreateSendCalls(t, producer, 1)

	_ = logicpkg.NewCreateOrderConsumerLogic(ctx, svcCtx).Consume(producer.lastBody)

	if countShardOrderRows(t, svcCtx.Config.MySQL.DataSource) != 0 {
		t.Fatalf("expected lease-lost consumer to stop before persisting order")
	}
	if programRPC.releaseSeatFreezeCalls != 0 {
		t.Fatalf("expected lease-lost consumer not to release freeze on its own, got %d", programRPC.releaseSeatFreezeCalls)
	}
}
