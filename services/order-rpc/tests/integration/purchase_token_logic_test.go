package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/pb"
	userrpc "damai-go/services/user-rpc/userrpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPurchaseTokenPrecheckAndCreateOrderTokenOnly(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	userID, programID, ticketCategoryID, viewerIDs, _ := nextRushTestIDs()

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(programID, ticketCategoryID, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		userID,
		&userrpc.TicketUserInfo{Id: viewerIDs[0], UserId: userID + 1, RelName: "非本人", IdType: 1, IdNumber: "110101199001011111"},
	)

	createPurchaseTokenLogic := logicpkg.NewCreatePurchaseTokenLogic(context.Background(), svcCtx)
	_, err := createPurchaseTokenLogic.CreatePurchaseToken(&pb.CreatePurchaseTokenReq{
		UserId:           userID,
		ShowTimeId:       programID,
		TicketCategoryId: ticketCategoryID,
		TicketUserIds:    []int64{viewerIDs[0]},
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument for precheck failure, got err=%v", err)
	}

	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		userID,
		&userrpc.TicketUserInfo{Id: viewerIDs[0], UserId: userID, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)
	tokenResp, err := createPurchaseTokenLogic.CreatePurchaseToken(&pb.CreatePurchaseTokenReq{
		UserId:           userID,
		ShowTimeId:       programID,
		TicketCategoryId: ticketCategoryID,
		TicketUserIds:    []int64{viewerIDs[0]},
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
	if claims.UserID != userID || claims.ProgramID != programID || claims.ShowTimeID != programID || claims.TicketCategoryID != ticketCategoryID {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.DistributionMode != "express" || claims.TakeTicketMode != "paper" {
		t.Fatalf("unexpected transport claims: %+v", claims)
	}
	if claims.Generation != rush.BuildRushGeneration(programID) || claims.SaleWindowEndAt == 0 || claims.ShowEndAt == 0 {
		t.Fatalf("expected show time generation window claims, got %+v", claims)
	}
	available, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(context.Background(), programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 100 {
		t.Fatalf("expected purchase token precheck to prime quota=100, got ok=%t available=%d", ok, available)
	}

	programRPC.getProgramPreorderErr = status.Error(codes.Unavailable, "program rpc unavailable")
	userRPC.getUserAndTicketUserListErr = status.Error(codes.Unavailable, "user rpc unavailable")
	programRPC.lastGetProgramPreorderReq = nil
	userRPC.lastGetUserAndTicketUserListReq = nil

	createOrderLogic := logicpkg.NewCreateOrderLogic(context.Background(), svcCtx)
	orderResp, err := createOrderLogic.CreateOrder(&pb.CreateOrderReq{
		UserId:        userID,
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

func TestCreatePurchaseTokenRejectsDuplicateTicketUserIDs(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	userID, programID, ticketCategoryID, viewerIDs, _ := nextRushTestIDs()

	programRPC.getProgramPreorderResp = buildTestProgramPreorder(programID, ticketCategoryID, 2, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		userID,
		&userrpc.TicketUserInfo{Id: viewerIDs[0], UserId: userID, RelName: "张三", IdType: 1, IdNumber: "110101199001011234"},
	)

	createPurchaseTokenLogic := logicpkg.NewCreatePurchaseTokenLogic(context.Background(), svcCtx)
	_, err := createPurchaseTokenLogic.CreatePurchaseToken(&pb.CreatePurchaseTokenReq{
		UserId:           userID,
		ShowTimeId:       programID,
		TicketCategoryId: ticketCategoryID,
		TicketUserIds:    []int64{viewerIDs[0], viewerIDs[0]},
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument for duplicate ticket users, got err=%v", err)
	}
	if status.Convert(err).Message() != "order ticket user invalid" {
		t.Fatalf("expected duplicate ticket users to be rejected as order ticket user invalid, got %q", status.Convert(err).Message())
	}
}
