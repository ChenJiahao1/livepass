package logic

import (
	"context"
	"testing"
	"time"

	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateOrderReturnsOrderNumberFromPurchaseToken(t *testing.T) {
	codec := rush.MustNewPurchaseTokenCodec("test-secret", 2*time.Minute)
	token, err := codec.Issue(rush.PurchaseTokenClaims{
		OrderNumber:      987654321001,
		UserID:           3001,
		ProgramID:        10001,
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{701},
		TicketCount:      1,
	})
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	resp, err := NewCreateOrderLogic(context.Background(), &svc.ServiceContext{
		PurchaseTokenCodec: codec,
	}).CreateOrder(&pb.CreateOrderReq{
		UserId:        3001,
		PurchaseToken: token,
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if resp.GetOrderNumber() != 987654321001 {
		t.Fatalf("expected order number 987654321001, got %d", resp.GetOrderNumber())
	}
}

func TestCreateOrderRejectsPurchaseTokenForDifferentUser(t *testing.T) {
	codec := rush.MustNewPurchaseTokenCodec("test-secret", 2*time.Minute)
	token, err := codec.Issue(rush.PurchaseTokenClaims{
		OrderNumber:      9001,
		UserID:           3001,
		ProgramID:        10001,
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{701},
		TicketCount:      1,
	})
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	_, err = NewCreateOrderLogic(context.Background(), &svc.ServiceContext{
		PurchaseTokenCodec: codec,
	}).CreateOrder(&pb.CreateOrderReq{
		UserId:        3002,
		PurchaseToken: token,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestCreateOrderRejectsExpiredPurchaseToken(t *testing.T) {
	codec := rush.MustNewPurchaseTokenCodec("test-secret", 2*time.Minute)
	token, err := codec.Issue(rush.PurchaseTokenClaims{
		OrderNumber:      9001,
		UserID:           3001,
		ProgramID:        10001,
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{701},
		TicketCount:      1,
		ExpireAt:         time.Now().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	_, err = NewCreateOrderLogic(context.Background(), &svc.ServiceContext{
		PurchaseTokenCodec: codec,
	}).CreateOrder(&pb.CreateOrderReq{
		UserId:        3001,
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

func TestRushContractOrderNumberDoesNotReuseAfterSequenceExhausted(t *testing.T) {
	originalGenerator := defaultOrderNumberGenerator
	originalNow := rushContractNow
	defer func() {
		defaultOrderNumberGenerator = originalGenerator
		rushContractNow = originalNow
	}()

	base := time.Date(2026, time.March, 25, 10, 30, 15, 0, time.UTC)
	current := base

	gen := newOrderNumberGenerator(func() int64 { return 0 })
	gen.now = func() time.Time { return current }
	gen.sleep = func(d time.Duration) { current = current.Add(d) }
	gen.lastUnixSecond = base.Unix()
	gen.sequence = maxOrderNumberSequence - 1

	defaultOrderNumberGenerator = gen
	rushContractNow = func() time.Time { return base }

	first := allocateRushContractOrderNumber(3001)
	second := allocateRushContractOrderNumber(3001)
	if first == second {
		t.Fatalf("expected unique order numbers across sequence exhaustion, got duplicated %d", first)
	}
}
