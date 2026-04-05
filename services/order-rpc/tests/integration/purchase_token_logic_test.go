package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
	userrpc "damai-go/services/user-rpc/userrpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPurchaseTokenPrecheckAndCreateOrderTokenOnly(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(10001, 40001, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3999, RelName: "非本人", IdType: 1, IdNumber: "110101199001011111"},
	)

	createPurchaseTokenLogic := logicpkg.NewCreatePurchaseTokenLogic(context.Background(), svcCtx)
	_, err := createPurchaseTokenLogic.CreatePurchaseToken(&pb.CreatePurchaseTokenReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument for precheck failure, got err=%v", err)
	}

	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		3001,
		&userrpc.TicketUserInfo{Id: 701, UserId: 3001, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	tokenResp, err := createPurchaseTokenLogic.CreatePurchaseToken(&pb.CreatePurchaseTokenReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if err != nil {
		t.Fatalf("CreatePurchaseToken() error = %v", err)
	}
	if tokenResp.GetPurchaseToken() == "" {
		t.Fatalf("expected non-empty token")
	}
	claims, err := svcCtx.PurchaseTokenCodec.Verify(tokenResp.GetPurchaseToken())
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.UserID != 3001 || claims.ProgramID != 10001 || claims.TicketCategoryID != 40001 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.DistributionMode != "express" || claims.TakeTicketMode != "paper" {
		t.Fatalf("unexpected transport claims: %+v", claims)
	}

	programRPC.getProgramPreorderErr = status.Error(codes.Unavailable, "program rpc unavailable")
	userRPC.getUserAndTicketUserListErr = status.Error(codes.Unavailable, "user rpc unavailable")
	programRPC.lastGetProgramPreorderReq = nil
	userRPC.lastGetUserAndTicketUserListReq = nil

	createOrderLogic := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx)
	orderResp, err := createOrderLogic.CreateOrder(&pb.CreateOrderReq{
		UserId:        3001,
		PurchaseToken: tokenResp.GetPurchaseToken(),
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if orderResp.GetOrderNumber() <= 0 {
		t.Fatalf("expected positive order number, got %d", orderResp.GetOrderNumber())
	}
	if orderResp.GetOrderNumber() != claims.OrderNumber {
		t.Fatalf("expected order number %d, got %d", claims.OrderNumber, orderResp.GetOrderNumber())
	}
	if programRPC.lastGetProgramPreorderReq != nil || userRPC.lastGetUserAndTicketUserListReq != nil {
		t.Fatalf("expected CreateOrder to skip ProgramRpc/UserRpc, got programReq=%+v userReq=%+v", programRPC.lastGetProgramPreorderReq, userRPC.lastGetUserAndTicketUserListReq)
	}
}
