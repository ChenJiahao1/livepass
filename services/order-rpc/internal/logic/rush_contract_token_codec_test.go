package logic

import (
	"context"
	"testing"

	"damai-go/services/order-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRushContractPurchaseTokenCodecRoundTrip(t *testing.T) {
	token, err := encodeRushContractPurchaseToken(3001, 987654321001)
	if err != nil {
		t.Fatalf("encodeRushContractPurchaseToken returned error: %v", err)
	}
	if token == "" {
		t.Fatalf("expected non-empty token")
	}

	decodedUserID, decodedOrderNumber, err := decodeRushContractPurchaseToken(token)
	if err != nil {
		t.Fatalf("decodeRushContractPurchaseToken returned error: %v", err)
	}
	if decodedUserID != 3001 || decodedOrderNumber != 987654321001 {
		t.Fatalf("unexpected decoded values: userID=%d orderNumber=%d", decodedUserID, decodedOrderNumber)
	}
}

func TestCreatePurchaseTokenAndCreateOrderAreConsistentAndIdempotent(t *testing.T) {
	createPurchaseTokenLogic := NewCreatePurchaseTokenLogic(context.Background(), nil)
	createOrderLogic := NewCreateOrderLogic(context.Background(), nil)

	tokenResp, err := createPurchaseTokenLogic.CreatePurchaseToken(&pb.CreatePurchaseTokenReq{
		UserId:           3001,
		ProgramId:        10001,
		TicketCategoryId: 40001,
		TicketUserIds:    []int64{701},
	})
	if err != nil {
		t.Fatalf("CreatePurchaseToken returned error: %v", err)
	}
	if tokenResp.GetPurchaseToken() == "" {
		t.Fatalf("expected non-empty purchase token")
	}

	firstResp, err := createOrderLogic.CreateOrder(&pb.CreateOrderReq{
		UserId:        3001,
		PurchaseToken: tokenResp.GetPurchaseToken(),
	})
	if err != nil {
		t.Fatalf("CreateOrder first call returned error: %v", err)
	}
	secondResp, err := createOrderLogic.CreateOrder(&pb.CreateOrderReq{
		UserId:        3001,
		PurchaseToken: tokenResp.GetPurchaseToken(),
	})
	if err != nil {
		t.Fatalf("CreateOrder second call returned error: %v", err)
	}
	if firstResp.GetOrderNumber() <= 0 {
		t.Fatalf("expected positive order number, got %d", firstResp.GetOrderNumber())
	}
	if firstResp.GetOrderNumber() != secondResp.GetOrderNumber() {
		t.Fatalf("expected idempotent order number, got %d and %d", firstResp.GetOrderNumber(), secondResp.GetOrderNumber())
	}
}

func TestCreateOrderRejectsPurchaseTokenForDifferentUser(t *testing.T) {
	token, err := encodeRushContractPurchaseToken(3001, 9001)
	if err != nil {
		t.Fatalf("encodeRushContractPurchaseToken returned error: %v", err)
	}

	_, err = NewCreateOrderLogic(context.Background(), nil).CreateOrder(&pb.CreateOrderReq{
		UserId:        3002,
		PurchaseToken: token,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestPollOrderProgressReturnsUnimplemented(t *testing.T) {
	_, err := NewPollOrderProgressLogic(context.Background(), nil).PollOrderProgress(&pb.PollOrderProgressReq{
		UserId:      3001,
		OrderNumber: 9001,
	})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected unimplemented, got %v", err)
	}
}

func TestVerifyAttemptDueReturnsUnimplemented(t *testing.T) {
	_, err := NewVerifyAttemptDueLogic(context.Background(), nil).VerifyAttemptDue(&pb.VerifyAttemptDueReq{
		OrderNumber: 9001,
		DueAtUnix:   1,
	})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected unimplemented, got %v", err)
	}
}

func TestReconcileRushAttemptsReturnsUnimplemented(t *testing.T) {
	_, err := NewReconcileRushAttemptsLogic(context.Background(), nil).ReconcileRushAttempts(&pb.ReconcileRushAttemptsReq{
		Limit: 10,
	})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected unimplemented, got %v", err)
	}
}
