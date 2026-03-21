package integration_test

import (
	"context"
	"errors"
	"testing"

	"damai-go/services/order-rpc/internal/repeatguard"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestEtcdGuardReturnsLockedForDuplicateKey(t *testing.T) {
	guard := newTestEtcdRepeatGuard(t, mustTestEtcdEndpoints(t))

	unlock, err := guard.Lock(context.Background(), "create_order:3001:10001")
	if err != nil {
		t.Fatalf("first lock returned error: %v", err)
	}
	defer unlock()

	_, err = guard.Lock(context.Background(), "create_order:3001:10001")
	if !errors.Is(err, repeatguard.ErrLocked) {
		t.Fatalf("expected repeatguard.ErrLocked, got %v", err)
	}
}

func TestEtcdGuardReturnsUnavailableWhenClusterIsDown(t *testing.T) {
	guard := newTestEtcdRepeatGuard(t, []string{"127.0.0.1:32379"})

	_, err := guard.Lock(context.Background(), "create_order:3001:10001")
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}
}
