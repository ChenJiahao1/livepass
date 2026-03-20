package logic

import (
	"context"
	"strings"
	"testing"

	"damai-go/pkg/xerr"
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

	l := NewCreateOrderLogic(context.Background(), svcCtx)
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

	l := NewCreateOrderLogic(context.Background(), svcCtx)
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

func TestCreateOrderRejectsPerOrderLimitExceeded(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 1, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
		&userrpc.TicketUserInfo{Id: 702, UserId: 3001, RelName: "李四", IdType: 1, IdNumber: "110101199002021234"},
	)

	l := NewCreateOrderLogic(context.Background(), svcCtx)
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

	l := NewCreateOrderLogic(context.Background(), svcCtx)
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

func TestCreateOrderLeavesTablesEmptyWhenSeatFreezeFails(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	programRPC.autoAssignAndFreezeSeatsErr = status.Error(codes.FailedPrecondition, xerr.ErrSeatInventoryInsufficient.Error())
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)

	l := NewCreateOrderLogic(context.Background(), svcCtx)
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

	l := NewCreateOrderLogic(context.Background(), svcCtx)
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
