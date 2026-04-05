//go:build legacy_create_order_contract
// +build legacy_create_order_contract

package integration_test

import (
	"context"
	"testing"
	"time"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"
)

func TestCreateOrderConsumerPersistsOrderAndTickets(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
		&userrpc.TicketUserInfo{Id: 702, UserId: 3001, RelName: "李四", IdType: 1, IdNumber: "110101199002021234"},
	)
	event := mustBuildOrderCreateEventForTest(t, programRPC.getProgramPreorderResp, userRPC.getUserAndTicketUserListResp, &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-consumer-001",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
			{SeatId: 502, TicketCategoryId: 40001, RowCode: 1, ColCode: 2, Price: 299},
		},
	}, time.Now())

	body, err := event.Marshal()
	if err != nil {
		t.Fatalf("event.Marshal returned error: %v", err)
	}

	err = logicpkg.NewCreateOrderConsumerLogic(context.Background(), svcCtx).Consume(body)
	if err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}
	route, err := svcCtx.OrderRepository.RouteByUserID(context.Background(), 3001)
	if err != nil {
		t.Fatalf("RouteByUserID returned error: %v", err)
	}
	if countRows(t, svcCtx.Config.MySQL.DataSource, "d_order_"+route.TableSuffix) != 1 {
		t.Fatalf("expected one order row")
	}
	if countRows(t, svcCtx.Config.MySQL.DataSource, "d_order_ticket_user_"+route.TableSuffix) != 2 {
		t.Fatalf("expected two order ticket rows")
	}
}

func TestCreateOrderConsumerEnqueuesCloseTimeoutAfterPersist(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	event := mustBuildOrderCreateEventForTest(t, programRPC.getProgramPreorderResp, userRPC.getUserAndTicketUserListResp, &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-consumer-enqueue",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}, time.Now())

	body, err := event.Marshal()
	if err != nil {
		t.Fatalf("event.Marshal returned error: %v", err)
	}

	err = logicpkg.NewCreateOrderConsumerLogic(context.Background(), svcCtx).Consume(body)
	if err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}

	asyncCloseClient, ok := svcCtx.AsyncCloseClient.(*fakeAsyncCloseClient)
	if !ok {
		t.Fatalf("expected fake async close client, got %T", svcCtx.AsyncCloseClient)
	}
	if asyncCloseClient.enqueueCalls != 1 {
		t.Fatalf("expected enqueue once, got %d", asyncCloseClient.enqueueCalls)
	}
	if asyncCloseClient.lastOrderNumber != 9001 {
		t.Fatalf("expected enqueue order number 9001, got %d", asyncCloseClient.lastOrderNumber)
	}
	if asyncCloseClient.lastExpireAt.IsZero() {
		t.Fatalf("expected enqueue expire time to be recorded")
	}
}

func TestCreateOrderConsumerTreatsDuplicateOrderNumberAsIdempotentSuccess(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	event := mustBuildOrderCreateEventForTest(t, programRPC.getProgramPreorderResp, userRPC.getUserAndTicketUserListResp, &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-consumer-duplicate",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}, time.Now())

	body, err := event.Marshal()
	if err != nil {
		t.Fatalf("event.Marshal returned error: %v", err)
	}

	consumer := logicpkg.NewCreateOrderConsumerLogic(context.Background(), svcCtx)
	if err := consumer.Consume(body); err != nil {
		t.Fatalf("first Consume returned error: %v", err)
	}
	if err := consumer.Consume(body); err != nil {
		t.Fatalf("second Consume returned error: %v", err)
	}
	route, err := svcCtx.OrderRepository.RouteByUserID(context.Background(), 3001)
	if err != nil {
		t.Fatalf("RouteByUserID returned error: %v", err)
	}
	if countRows(t, svcCtx.Config.MySQL.DataSource, "d_order_"+route.TableSuffix) != 1 {
		t.Fatalf("expected one order row after duplicate consume")
	}
	if countRows(t, svcCtx.Config.MySQL.DataSource, "d_order_ticket_user_"+route.TableSuffix) != 1 {
		t.Fatalf("expected one order ticket row after duplicate consume")
	}
}

func TestCreateOrderConsumerSkipsExpiredMessageAndReleasesFreeze(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	svcCtx.Config.Kafka.MaxMessageDelay = 5 * time.Second

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 1, map[int64]int64{9001: 1})
	event := mustBuildOrderCreateEventForTest(t, programRPC.getProgramPreorderResp, userRPC.getUserAndTicketUserListResp, &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-consumer-expired",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}, time.Now().Add(-time.Minute))

	body, err := event.Marshal()
	if err != nil {
		t.Fatalf("event.Marshal returned error: %v", err)
	}

	err = logicpkg.NewCreateOrderConsumerLogic(context.Background(), svcCtx).Consume(body)
	if err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected expired message to release freeze once")
	}
	snapshot := requirePurchaseLimitSnapshot(t, svcCtx, 3001, 10001)
	if snapshot.ActiveCount != 0 || len(snapshot.Reservations) != 0 {
		t.Fatalf("expected expired message to rollback purchase limit ledger, got %+v", snapshot)
	}
}

func TestCreateOrderConsumerDoesNotCreatePurchaseLimitLedgerWhenAccountLimitDisabledAndMessageExpired(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	svcCtx.Config.Kafka.MaxMessageDelay = 5 * time.Second

	clearPurchaseLimitLedger(t, svcCtx, 3001, 10001)
	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 0, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	event := mustBuildOrderCreateEventForTest(t, programRPC.getProgramPreorderResp, userRPC.getUserAndTicketUserListResp, &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-consumer-expired-no-account-limit",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}, time.Now().Add(-time.Minute))

	body, err := event.Marshal()
	if err != nil {
		t.Fatalf("event.Marshal returned error: %v", err)
	}

	err = logicpkg.NewCreateOrderConsumerLogic(context.Background(), svcCtx).Consume(body)
	if err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected expired message to release freeze once")
	}
	requirePurchaseLimitLedgerAbsentFor(t, svcCtx, 3001, 10001, 500*time.Millisecond)
}

func mustBuildOrderCreateEventForTest(
	t *testing.T,
	preorder *programrpc.ProgramPreorderInfo,
	userResp *userrpc.GetUserAndTicketUserListResp,
	freezeResp *programrpc.AutoAssignAndFreezeSeatsResp,
	now time.Time,
) interface{ Marshal() ([]byte, error) } {
	t.Helper()

	event, err := logicpkg.BuildOrderCreateEvent(9001, &pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    collectTicketUserIDs(userResp),
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	}, preorder, userResp, freezeResp, now, 15*time.Minute)
	if err != nil {
		t.Fatalf("BuildOrderCreateEvent returned error: %v", err)
	}

	return event
}

func collectTicketUserIDs(userResp *userrpc.GetUserAndTicketUserListResp) []int64 {
	ids := make([]int64, 0, len(userResp.GetTicketUserVoList()))
	for _, ticketUser := range userResp.GetTicketUserVoList() {
		if ticketUser == nil {
			continue
		}
		ids = append(ids, ticketUser.GetId())
	}

	return ids
}
