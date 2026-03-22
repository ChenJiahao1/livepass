package xid

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestNewPanicsBeforeInit(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
	})

	_ = Close()

	defer func() {
		if recover() == nil {
			t.Fatal("expected New to panic before initialization")
		}
	}()

	_ = New()
}

func TestInitEtcdGeneratesIncreasingIDs(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
	})

	if err := InitEtcd(context.Background(), Config{
		Hosts:   mustTestEtcdEndpoints(t),
		Prefix:  testPrefix(t),
		Service: "pkg-xid-test-global",
	}); err != nil {
		t.Fatalf("InitEtcd error: %v", err)
	}

	first := New()
	second := New()

	if first <= 0 {
		t.Fatalf("expected first id > 0, got %d", first)
	}
	if second <= first {
		t.Fatalf("expected increasing ids, first=%d second=%d", first, second)
	}
}

func TestNewGeneratorWithClientAllocatesDifferentNodes(t *testing.T) {
	client := newTestEtcdClient(t, mustTestEtcdEndpoints(t))
	prefix := testPrefix(t)

	gen1, err := newGeneratorWithClient(context.Background(), client, Config{
		Prefix:  prefix,
		Service: "pkg-xid-test-1",
	})
	if err != nil {
		t.Fatalf("newGeneratorWithClient gen1 error: %v", err)
	}
	defer func() {
		_ = gen1.close()
	}()

	gen2, err := newGeneratorWithClient(context.Background(), client, Config{
		Prefix:  prefix,
		Service: "pkg-xid-test-2",
	})
	if err != nil {
		t.Fatalf("newGeneratorWithClient gen2 error: %v", err)
	}
	defer func() {
		_ = gen2.close()
	}()

	if gen1.nodeID == gen2.nodeID {
		t.Fatalf("expected different node ids, both=%d", gen1.nodeID)
	}
}

func TestGeneratorCloseReleasesNode(t *testing.T) {
	client := newTestEtcdClient(t, mustTestEtcdEndpoints(t))
	prefix := testPrefix(t)

	gen1, err := newGeneratorWithClient(context.Background(), client, Config{
		Prefix:  prefix,
		Service: "pkg-xid-test-release-1",
	})
	if err != nil {
		t.Fatalf("newGeneratorWithClient gen1 error: %v", err)
	}

	nodeID := gen1.nodeID
	if err := gen1.close(); err != nil {
		t.Fatalf("gen1.close error: %v", err)
	}

	gen2, err := newGeneratorWithClient(context.Background(), client, Config{
		Prefix:  prefix,
		Service: "pkg-xid-test-release-2",
	})
	if err != nil {
		t.Fatalf("newGeneratorWithClient gen2 error: %v", err)
	}
	defer func() {
		_ = gen2.close()
	}()

	if gen2.nodeID != nodeID {
		t.Fatalf("expected released node id %d to be reused, got %d", nodeID, gen2.nodeID)
	}
}

func TestInitEtcdReturnsErrorWhenClusterUnavailable(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
	})

	err := InitEtcd(context.Background(), Config{
		Hosts:       []string{"127.0.0.1:32379"},
		Prefix:      testPrefix(t),
		Service:     "pkg-xid-test-unavailable",
		DialTimeout: 200 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected InitEtcd to fail when etcd is unavailable")
	}
}

func mustTestEtcdEndpoints(t *testing.T) []string {
	t.Helper()

	raw := os.Getenv("XID_TEST_ETCD_ENDPOINTS")
	if raw == "" {
		raw = "127.0.0.1:2379"
	}
	endpoints := strings.Split(raw, ",")

	client := newTestEtcdClient(t, endpoints)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if _, err := client.Get(ctx, "xid-healthcheck"); err != nil {
		t.Fatalf("probe etcd %v: %v; start it with 'docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d'", endpoints, err)
	}

	return endpoints
}

func newTestEtcdClient(t *testing.T, endpoints []string) *clientv3.Client {
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

	return client
}

func testPrefix(t *testing.T) string {
	t.Helper()
	return "/damai-go/tests/xid/" + strings.ReplaceAll(t.Name(), "/", "-") + "/"
}
