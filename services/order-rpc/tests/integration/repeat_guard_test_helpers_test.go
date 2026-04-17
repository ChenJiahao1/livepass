package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"livepass/services/order-rpc/internal/config"
	"livepass/services/order-rpc/internal/repeatguard"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func mustTestEtcdEndpoints(t *testing.T) []string {
	t.Helper()

	raw := os.Getenv("ORDER_REPEAT_GUARD_ETCD_ENDPOINTS")
	if raw == "" {
		raw = "127.0.0.1:2379"
	}
	endpoints := strings.Split(raw, ",")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("connect etcd %v: %v; start it with 'docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d'", endpoints, err)
	}
	defer client.Close()

	if _, err := client.Get(ctx, "repeat-guard-healthcheck"); err != nil {
		t.Fatalf("probe etcd %v: %v; start it with 'docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d'", endpoints, err)
	}

	return endpoints
}

func newTestEtcdRepeatGuard(t *testing.T, endpoints []string) repeatguard.Guard {
	t.Helper()

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new etcd client %v: %v", endpoints, err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})

	return repeatguard.NewEtcdGuard(
		client,
		config.RepeatGuardConfig{
			Prefix:             "/livepass/tests/repeat-guard/",
			SessionTTL:         10,
			LockAcquireTimeout: 200 * time.Millisecond,
		},
	)
}

func newClosedTestEtcdRepeatGuard(t *testing.T) repeatguard.Guard {
	t.Helper()

	endpoints := mustTestEtcdEndpoints(t)
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new etcd client %v: %v", endpoints, err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close etcd client %v: %v", endpoints, err)
	}

	return repeatguard.NewEtcdGuard(
		client,
		config.RepeatGuardConfig{
			Prefix:             "/livepass/tests/repeat-guard/",
			SessionTTL:         10,
			LockAcquireTimeout: 200 * time.Millisecond,
		},
	)
}

func newTestEtcdRepeatGuardFromConfig(t *testing.T, cfg config.Config) repeatguard.Guard {
	t.Helper()

	return newTestEtcdRepeatGuard(t, cfg.Etcd.Hosts)
}
