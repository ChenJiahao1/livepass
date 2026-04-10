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

func TestCreateOrderReturnsInternalWithoutRushDependencies(t *testing.T) {
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
	if resp != nil {
		t.Fatalf("expected nil response when rush dependencies are missing, got %+v", resp)
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected internal, got %v", err)
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

func TestPollOrderProgressReturnsInternalWithoutAttemptStore(t *testing.T) {
	_, err := NewPollOrderProgressLogic(context.Background(), nil).PollOrderProgress(&pb.PollOrderProgressReq{
		UserId:      3001,
		OrderNumber: 9001,
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected internal, got %v", err)
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
