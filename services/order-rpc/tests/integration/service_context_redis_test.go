package integration_test

import (
	"context"
	"database/sql"
	"testing"
	"time"
	_ "unsafe"

	"livepass/pkg/xerr"
	"livepass/pkg/xmysql"
	"livepass/pkg/xredis"
	"livepass/services/order-rpc/internal/config"
	"livepass/services/order-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//go:linkname mapOrderError livepass/services/order-rpc/internal/logic.mapOrderError
func mapOrderError(err error) error

func TestOrderServiceContextRedis(t *testing.T) {
	requireOrderMySQL(t)

	baseConfig := config.Config{
		RpcServerConf: zrpc.RpcServerConf{
			Etcd: discov.EtcdConf{
				Hosts: []string{"127.0.0.1:2379"},
			},
		},
		MySQL: xmysql.Config{
			DataSource:      testOrderMySQLDataSource,
			MaxOpenConns:    23,
			MaxIdleConns:    7,
			ConnMaxLifetime: 2 * time.Minute,
			ConnMaxIdleTime: 30 * time.Second,
		},
		Order: config.OrderConfig{
			CloseAfter: 15 * time.Minute,
		},
		RepeatGuard: config.RepeatGuardConfig{
			Prefix:             "/livepass/tests/repeat-guard/order-create/",
			SessionTTL:         10,
			LockAcquireTimeout: 200 * time.Millisecond,
		},
	}

	t.Run("redis disabled when host empty", func(t *testing.T) {
		svcCtx := svc.NewServiceContext(baseConfig)
		if svcCtx.Redis != nil {
			t.Fatalf("expected redis client to be nil when host is empty")
		}
		rawDB, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			t.Fatalf("raw db: %v", err)
		}
		if rawDB.Stats().MaxOpenConnections != 23 {
			t.Fatalf("expected max open connections 23, got %d", rawDB.Stats().MaxOpenConnections)
		}
	})

	t.Run("redis enabled when host configured", func(t *testing.T) {
		cfg := baseConfig
		cfg.StoreRedis = xredis.Config{
			Host: "127.0.0.1:6379",
			Type: "node",
		}
		cfg.RushOrder = config.RushOrderConfig{
			Enabled:       true,
			TokenSecret:   "order-rpc-test-secret",
			TokenTTL:      2 * time.Minute,
			InFlightTTL:   30 * time.Second,
			FinalStateTTL: 30 * time.Minute,
		}
		svcCtx := svc.NewServiceContext(cfg)
		if svcCtx.Redis == nil {
			t.Fatalf("expected redis client to be wired when host is configured")
		}
		if svcCtx.AttemptStore == nil {
			t.Fatalf("expected attempt store to be wired when rush order and redis are configured")
		}
	})
}

func requireOrderMySQL(t *testing.T) {
	t.Helper()

	db, err := sql.Open("mysql", testOrderMySQLDataSource)
	if err != nil {
		t.Skipf("skip integration test, sql.Open mysql failed: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skip integration test, mysql unavailable: %v", err)
	}
}

func TestOrderLedgerNotReadyMappedToFailedPrecondition(t *testing.T) {
	err := mapOrderError(xerr.ErrOrderLimitLedgerNotReady)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
}
