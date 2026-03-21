package integration_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"damai-go/pkg/xerr"
	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/repeatguard"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateOrderCreatesOrderAndSnapshots(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

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
	if countRows(t, svcCtx.Config.MySQL.DataSource, "d_order") != 1 {
		t.Fatalf("expected one order row")
	}
	if countRows(t, svcCtx.Config.MySQL.DataSource, "d_order_ticket_user") != 2 {
		t.Fatalf("expected two order ticket rows")
	}
	if programRPC.lastAutoAssignAndFreezeSeatsReq == nil || !strings.HasPrefix(programRPC.lastAutoAssignAndFreezeSeatsReq.RequestNo, "order-") {
		t.Fatalf("expected freeze request no with order- prefix, got %+v", programRPC.lastAutoAssignAndFreezeSeatsReq)
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
	if countRows(t, svcCtx.Config.MySQL.DataSource, "d_order") != 0 {
		t.Fatalf("expected no order row when freeze fails")
	}
	if countRows(t, svcCtx.Config.MySQL.DataSource, "d_order_ticket_user") != 0 {
		t.Fatalf("expected no order ticket rows when freeze fails")
	}
	if programRPC.releaseSeatFreezeCalls != 0 {
		t.Fatalf("expected no compensation release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
}

func TestCreateOrderReleasesFreezeOnceWhenInsertFails(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

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

	l := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx)
	_, err := l.CreateOrder(&pb.CreateOrderReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})
	if err == nil {
		t.Fatalf("expected insert failure")
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected one compensation release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
	if programRPC.lastReleaseSeatFreezeReq == nil || programRPC.lastReleaseSeatFreezeReq.FreezeToken != "freeze-create-002" {
		t.Fatalf("unexpected release request: %+v", programRPC.lastReleaseSeatFreezeReq)
	}
	if countRows(t, svcCtx.Config.MySQL.DataSource, "d_order") != 0 {
		t.Fatalf("expected rolled back order row")
	}
	if countRows(t, svcCtx.Config.MySQL.DataSource, "d_order_ticket_user") != 0 {
		t.Fatalf("expected rolled back order ticket rows")
	}
}
