package integration_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"damai-go/pkg/xerr"
	orderevent "damai-go/services/order-rpc/internal/event"
	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/repeatguard"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	"damai-go/services/order-rpc/sharding"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBuildOrderCreateEventCarriesSeatAndProgramSnapshots(t *testing.T) {
	mustInitOrderTestXid(t)

	preorder := buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	userResp := buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
		&userrpc.TicketUserInfo{Id: 702, UserId: 3001, RelName: "李四", IdType: 1, IdNumber: "110101199002021234"},
	)
	freezeResp := &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-001",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
			{SeatId: 502, TicketCategoryId: 40001, RowCode: 1, ColCode: 2, Price: 299},
		},
	}

	event, err := logicpkg.BuildOrderCreateEvent(
		9001,
		&pb.CreateOrderReq{
			UserId:           3001,
			ProgramId:        10001,
			TicketCategoryId: 40001,
			TicketUserIds:    []int64{701, 702},
			DistributionMode: "express",
			TakeTicketMode:   "paper",
		},
		preorder,
		userResp,
		freezeResp,
		time.Date(2026, 3, 21, 20, 0, 0, 0, time.Local),
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("BuildOrderCreateEvent returned error: %v", err)
	}
	if event.OrderNumber == 0 || event.FreezeToken == "" {
		t.Fatalf("unexpected event: %+v", event)
	}
	if len(event.SeatSnapshot) != 2 || event.ProgramSnapshot.Title == "" {
		t.Fatalf("event snapshot incomplete: %+v", event)
	}
}

func TestCreateOrderReturnsOrderNumberAfterKafkaSendSucceeds(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-001",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
			{SeatId: 502, TicketCategoryId: 40001, RowCode: 1, ColCode: 2, Price: 299},
		},
	}
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
		&userrpc.TicketUserInfo{Id: 702, UserId: 3001, RelName: "李四", IdType: 1, IdNumber: "110101199002021234"},
	)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)

	l := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx)
	resp, err := l.CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701, 702},
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if resp.OrderNumber <= 0 {
		t.Fatalf("expected generated order number, got %d", resp.OrderNumber)
	}
	parts, err := sharding.ParseOrderNumber(resp.OrderNumber)
	if err != nil {
		t.Fatalf("ParseOrderNumber returned error: %v", err)
	}
	if parts.LogicSlot() != sharding.LogicSlotByUserID(3001) {
		t.Fatalf("expected logic slot %d, got %d", sharding.LogicSlotByUserID(3001), parts.LogicSlot())
	}
	if producer.sendCalls != 1 {
		t.Fatalf("expected producer send once, got %d", producer.sendCalls)
	}
	if producer.lastKey != strconv.FormatInt(resp.OrderNumber, 10) {
		t.Fatalf("expected producer key to be order number, got %s", producer.lastKey)
	}
	orderEvent, err := orderevent.UnmarshalOrderCreateEvent(producer.lastBody)
	if err != nil {
		t.Fatalf("UnmarshalOrderCreateEvent returned error: %v", err)
	}
	if orderEvent.OrderNumber != resp.OrderNumber || orderEvent.FreezeToken != "freeze-create-001" {
		t.Fatalf("unexpected event body: %+v", orderEvent)
	}
	if programRPC.lastAutoAssignAndFreezeSeatsReq == nil || !strings.HasPrefix(programRPC.lastAutoAssignAndFreezeSeatsReq.RequestNo, "order-") {
		t.Fatalf("expected freeze request no with order- prefix, got %+v", programRPC.lastAutoAssignAndFreezeSeatsReq)
	}
}

func TestCreateOrderRejectsWhenPurchaseLimitLedgerMissing(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	clearPurchaseLimitLedger(t, svcCtx, 3001, 10001)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)

	_, err := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
	if status.Convert(err).Message() != xerr.ErrOrderLimitLedgerNotReady.Error() {
		t.Fatalf("unexpected error message: %s", status.Convert(err).Message())
	}
	if programRPC.lastAutoAssignAndFreezeSeatsReq != nil {
		t.Fatalf("expected seat freeze to be skipped when purchase limit ledger is missing")
	}
	waitPurchaseLimitLedgerReady(t, svcCtx, 3001, 10001, 0)
}

func TestCreateOrderKeepsSeatFreezeWhenKafkaSendFails(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}
	producer.sendErr = errors.New("kafka send failed")

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-send-failed",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)

	_, err := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if err == nil {
		t.Fatalf("expected kafka send error")
	}
	if programRPC.releaseSeatFreezeCalls != 0 {
		t.Fatalf("expected seat freeze to remain held after kafka send failure, got %d releases", programRPC.releaseSeatFreezeCalls)
	}
}

func TestCreateOrderKeepsPurchaseLimitWhenKafkaSendFails(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}
	producer.sendErr = errors.New("kafka send failed")

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-rollback-send-failed",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)

	_, err := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})
	if err == nil {
		t.Fatalf("expected kafka send error")
	}

	snapshot := requirePurchaseLimitSnapshot(t, svcCtx, 3001, 10001)
	if snapshot.ActiveCount != 1 || snapshot.Reservations == nil || snapshot.Reservations[findOnlyOrderNumber(t, snapshot.Reservations)] != 1 {
		t.Fatalf("expected purchase limit ledger reservation to remain after kafka send failure, got %+v", snapshot)
	}
}

func TestCreateOrderDoesNotInsertOrderRowsSynchronously(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-async-window",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)

	resp, err := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if resp.OrderNumber <= 0 {
		t.Fatalf("expected generated order number, got %d", resp.OrderNumber)
	}
	if countShardOrderRows(t, svcCtx.Config.MySQL.DataSource) != 0 {
		t.Fatalf("expected no order row before consumer writes database")
	}
	if countShardOrderTicketRows(t, svcCtx.Config.MySQL.DataSource) != 0 {
		t.Fatalf("expected no order ticket rows before consumer writes database")
	}
}

func TestCreateOrderRejectsTicketUsersNotOwnedByCurrentUser(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3002, RelName: "越权购票人", IdType: 1, IdNumber: "110101199001011234"},
	)

	l := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx)
	_, err := l.CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})
	if err == nil {
		t.Fatalf("expected ticket user ownership error")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %s", status.Code(err))
	}
	if status.Convert(err).Message() != "order ticket user invalid" {
		t.Fatalf("unexpected error message: %s", status.Convert(err).Message())
	}
}

func TestCreateOrderRejectsDuplicateSubmissionWhenGuardReturnsLocked(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	svcCtx.RepeatGuard = &fakeOrderRepeatGuard{lockErr: repeatguard.ErrLocked}
	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)

	_, err := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})

	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected resource exhausted, got %v", err)
	}
	if status.Convert(err).Message() != xerr.ErrOrderSubmitTooFrequent.Error() {
		t.Fatalf("unexpected error message: %s", status.Convert(err).Message())
	}
}

func TestCreateOrderRejectsPerOrderLimitExceeded(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 1, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
		&userrpc.TicketUserInfo{Id: 702, UserId: 3001, RelName: "李四", IdType: 1, IdNumber: "110101199002021234"},
	)
	primePurchaseLimitLedgerFromDB(t, svcCtx, 3001, 10001)

	l := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx)
	_, err := l.CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701, 702},
	})
	if err == nil {
		t.Fatalf("expected purchase limit error")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %s", status.Code(err))
	}
}

func TestCreateOrderRejectsPerAccountLimitExceeded(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:          8001,
		OrderNumber: 9001,
		ProgramID:   10001,
		UserID:      3001,
		TicketCount: 2,
		OrderStatus: testOrderStatusUnpaid,
		FreezeToken: "freeze-existing-001",
	})

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 3, 3, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
		&userrpc.TicketUserInfo{Id: 702, UserId: 3001, RelName: "李四", IdType: 1, IdNumber: "110101199002021234"},
	)
	primePurchaseLimitLedgerFromDB(t, svcCtx, 3001, 10001)

	l := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx)
	_, err := l.CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701, 702},
	})
	if err == nil {
		t.Fatalf("expected account purchase limit error")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %s", status.Code(err))
	}
}

func TestCreateOrderRejectsPerAccountLimitExceededByPaidOrders(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:           8002,
		OrderNumber:  9002,
		ProgramID:    10001,
		UserID:       3001,
		TicketCount:  2,
		OrderStatus:  testOrderStatusPaid,
		FreezeToken:  "freeze-existing-paid-001",
		PayOrderTime: "2026-01-01 01:00:00",
	})

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 3, 3, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
		&userrpc.TicketUserInfo{Id: 702, UserId: 3001, RelName: "李四", IdType: 1, IdNumber: "110101199002021234"},
	)

	l := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx)
	_, err := l.CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701, 702},
	})
	if err == nil {
		t.Fatalf("expected account purchase limit error from paid tickets")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %s", status.Code(err))
	}
}

func TestCreateOrderRejectsConcurrentDuplicateSubmission(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	svcCtx.RepeatGuard = newTestEtcdRepeatGuardFromConfig(t, svcCtx.Config)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-concurrent",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)

	req := &pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	}

	startCh := make(chan struct{})
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(2)
	done.Add(2)

	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer done.Done()
			ready.Done()
			<-startCh
			_, err := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(req)
			errs <- err
		}()
	}

	ready.Wait()
	close(startCh)
	done.Wait()
	close(errs)

	var successCount int
	var duplicateCount int
	for err := range errs {
		switch status.Code(err) {
		case codes.OK:
			successCount++
		case codes.ResourceExhausted:
			duplicateCount++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if successCount != 1 || duplicateCount != 1 {
		t.Fatalf("expected 1 success and 1 duplicate rejection, got success=%d duplicate=%d", successCount, duplicateCount)
	}
}

func runCreateOrderConcurrently(t *testing.T, svcCtx *svc.ServiceContext, reqs ...*pb.CreateOrderReq) []error {
	t.Helper()

	startCh := make(chan struct{})
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(len(reqs))
	done.Add(len(reqs))

	errs := make([]error, len(reqs))
	for i, req := range reqs {
		i, req := i, req
		go func() {
			defer done.Done()
			ready.Done()
			<-startCh
			_, errs[i] = logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(req)
		}()
	}

	ready.Wait()
	close(startCh)
	done.Wait()
	return errs
}

func TestCreateOrderAllowsDifferentProgramsConcurrently(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	svcCtx.RepeatGuard = newTestEtcdRepeatGuardFromConfig(t, svcCtx.Config)

	programRPC.getProgramPreorderRespByProgramID = map[int64]*programrpc.ProgramPreorderInfo{
		10001: buildTestProgramPreorder(10001, 40001, 2, 4, 299),
		10002: buildTestProgramPreorder(10002, 40001, 2, 4, 399),
	}
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-different-programs",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}
	userRPC.getUserAndTicketUserListRespByUserID = map[int64]*userrpc.GetUserAndTicketUserListResp{
		3001: buildTestUserAndTicketUsers(
			3001,
			&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
		),
	}
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10002, 0, nil)

	req1 := &pb.CreateOrderReq{UserId: 3001, ProgramId: 10001, TicketCategoryId: 40001, TicketUserIds: []int64{701}}
	req2 := &pb.CreateOrderReq{UserId: 3001, ProgramId: 10002, TicketCategoryId: 40001, TicketUserIds: []int64{701}}

	errs := runCreateOrderConcurrently(t, svcCtx, req1, req2)
	for _, err := range errs {
		if status.Code(err) != codes.OK {
			t.Fatalf("expected both requests to succeed, got %v", err)
		}
	}
}

func TestCreateOrderAllowsDifferentUsersConcurrently(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	svcCtx.RepeatGuard = newTestEtcdRepeatGuardFromConfig(t, svcCtx.Config)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-different-users",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}
	userRPC.getUserAndTicketUserListRespByUserID = map[int64]*userrpc.GetUserAndTicketUserListResp{
		3001: buildTestUserAndTicketUsers(
			3001,
			&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
		),
		3002: buildTestUserAndTicketUsers(
			3002,
			&userrpc.TicketUserInfo{Id: 702, UserId: 3002, RelName: "李四", IdType: 1, IdNumber: "110101199202021234"},
		),
	}
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)
	seedPurchaseLimitLedger(t, svcCtx, 3002, 10001, 0, nil)

	req1 := &pb.CreateOrderReq{UserId: 3001, ProgramId: 10001, TicketCategoryId: 40001, TicketUserIds: []int64{701}}
	req2 := &pb.CreateOrderReq{UserId: 3002, ProgramId: 10001, TicketCategoryId: 40001, TicketUserIds: []int64{702}}

	errs := runCreateOrderConcurrently(t, svcCtx, req1, req2)
	for _, err := range errs {
		if status.Code(err) != codes.OK {
			t.Fatalf("expected both requests to succeed, got %v", err)
		}
	}
}

func TestCreateOrderReturnsUnavailableWhenGuardUnavailable(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	svcCtx.RepeatGuard = &fakeOrderRepeatGuard{lockErr: status.Error(codes.Unavailable, "repeat guard unavailable")}
	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)

	_, err := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})

	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

func TestCreateOrderLeavesTablesEmptyWhenSeatFreezeFails(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsErr = status.Error(codes.FailedPrecondition, xerr.ErrSeatInventoryInsufficient.Error())
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)

	l := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx)
	_, err := l.CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})
	if err == nil {
		t.Fatalf("expected seat freeze error")
	}
	if countShardOrderRows(t, svcCtx.Config.MySQL.DataSource) != 0 {
		t.Fatalf("expected no order row when freeze fails")
	}
	if countShardOrderTicketRows(t, svcCtx.Config.MySQL.DataSource) != 0 {
		t.Fatalf("expected no order ticket rows when freeze fails")
	}
	if programRPC.releaseSeatFreezeCalls != 0 {
		t.Fatalf("expected no compensation release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
}

func TestCreateOrderRollsBackPurchaseLimitWhenProgramFreezeFails(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsErr = status.Error(codes.FailedPrecondition, xerr.ErrSeatInventoryInsufficient.Error())
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)

	_, err := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})
	if err == nil {
		t.Fatalf("expected seat freeze error")
	}

	snapshot := requirePurchaseLimitSnapshot(t, svcCtx, 3001, 10001)
	if snapshot.ActiveCount != 0 || len(snapshot.Reservations) != 0 {
		t.Fatalf("expected purchase limit ledger rollback after seat freeze failure, got %+v", snapshot)
	}
}

func TestCreateOrderReleasesRecoveredSeatFreezeWhenProgramFreezeTimesOutAfterFreezeSucceeds(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)

	var freezeReqs []*programrpc.AutoAssignAndFreezeSeatsReq
	programRPC.autoAssignAndFreezeSeatsFunc = func(ctx context.Context, in *programrpc.AutoAssignAndFreezeSeatsReq) (*programrpc.AutoAssignAndFreezeSeatsResp, error) {
		freezeReqs = append(freezeReqs, in)
		if len(freezeReqs) == 1 {
			return nil, status.Error(codes.DeadlineExceeded, "context deadline exceeded")
		}
		return &programrpc.AutoAssignAndFreezeSeatsResp{
			FreezeToken: "freeze-recovered-after-timeout",
			ExpireTime:  "2026-12-31 19:45:00",
			Seats: []*programrpc.SeatInfo{
				{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
			},
		}, nil
	}

	_, err := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if len(freezeReqs) != 2 {
		t.Fatalf("expected two freeze attempts for recovery, got %d", len(freezeReqs))
	}
	if freezeReqs[0].RequestNo == "" || freezeReqs[1].RequestNo != freezeReqs[0].RequestNo {
		t.Fatalf("expected recovery to reuse request no, got first=%+v second=%+v", freezeReqs[0], freezeReqs[1])
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected recovered freeze to be released once, got %d", programRPC.releaseSeatFreezeCalls)
	}
	if programRPC.lastReleaseSeatFreezeReq == nil || programRPC.lastReleaseSeatFreezeReq.FreezeToken != "freeze-recovered-after-timeout" {
		t.Fatalf("unexpected release seat freeze request: %+v", programRPC.lastReleaseSeatFreezeReq)
	}

	snapshot := requirePurchaseLimitSnapshot(t, svcCtx, 3001, 10001)
	if snapshot.ActiveCount != 0 || len(snapshot.Reservations) != 0 {
		t.Fatalf("expected purchase limit ledger rollback after timeout recovery, got %+v", snapshot)
	}
}

func TestCreateOrderDoesNotCreatePurchaseLimitLedgerWhenAccountLimitDisabledAndKafkaSendFails(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}
	producer.sendErr = errors.New("kafka send failed")

	clearPurchaseLimitLedger(t, svcCtx, 3001, 10001)
	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 0, 299)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-no-account-limit-kafka-failed",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)

	_, err := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})
	if err == nil {
		t.Fatalf("expected kafka send error")
	}

	requirePurchaseLimitLedgerAbsentFor(t, svcCtx, 3001, 10001, 500*time.Millisecond)
}

func findOnlyOrderNumber(t *testing.T, reservations map[int64]int64) int64 {
	t.Helper()

	if len(reservations) != 1 {
		t.Fatalf("expected exactly one reservation, got %+v", reservations)
	}

	for orderNumber := range reservations {
		return orderNumber
	}

	t.Fatalf("expected exactly one reservation, got %+v", reservations)
	return 0
}

func TestCreateOrderDoesNotFailBeforeAsyncPersistence(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-002",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
		},
	}
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{
			Id:       701,
			UserId:   3001,
			RelName:  strings.Repeat("超长姓名", 40),
			IdType:   1,
			IdNumber: "110101199001011234",
		},
	)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)

	l := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx)
	resp, err := l.CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})
	if err != nil {
		t.Fatalf("expected async create to succeed before consumer persistence, got %v", err)
	}
	if resp.OrderNumber <= 0 {
		t.Fatalf("expected generated order number, got %+v", resp)
	}
	if producer.sendCalls != 1 {
		t.Fatalf("expected producer send once, got %d", producer.sendCalls)
	}
	if programRPC.releaseSeatFreezeCalls != 0 {
		t.Fatalf("expected no compensation release call before async persistence, got %d", programRPC.releaseSeatFreezeCalls)
	}
	if countShardOrderRows(t, svcCtx.Config.MySQL.DataSource) != 0 {
		t.Fatalf("expected no order row before consumer persistence")
	}
	if countShardOrderTicketRows(t, svcCtx.Config.MySQL.DataSource) != 0 {
		t.Fatalf("expected no order ticket rows before consumer persistence")
	}
}

func TestCreateOrderUsesReservedOrderNumberForPurchaseLimitLedger(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-create-ledger-order-number",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 501, TicketCategoryId: 40001, RowCode: 1, ColCode: 1, Price: 299},
			{SeatId: 502, TicketCategoryId: 40001, RowCode: 1, ColCode: 2, Price: 299},
		},
	}
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
		&userrpc.TicketUserInfo{Id: 702, UserId: 3001, RelName: "李四", IdType: 1, IdNumber: "110101199002021234"},
	)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 0, nil)

	resp, err := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701, 702},
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	snapshot := requirePurchaseLimitSnapshot(t, svcCtx, 3001, 10001)
	if snapshot.ActiveCount != 2 {
		t.Fatalf("expected active count 2, got %d", snapshot.ActiveCount)
	}
	if snapshot.Reservations[resp.OrderNumber] != 2 {
		t.Fatalf("expected reservation to use response order number, got %+v", snapshot.Reservations)
	}
}
