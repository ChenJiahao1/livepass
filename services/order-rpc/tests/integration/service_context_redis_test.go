package integration_test

import (
	"testing"
	"time"
	_ "unsafe"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//go:linkname mapOrderError damai-go/services/order-rpc/internal/logic.mapOrderError
func mapOrderError(err error) error

func TestOrderServiceContextRedis(t *testing.T) {
	baseConfig := config.Config{
		RpcServerConf: zrpc.RpcServerConf{
			Etcd: discov.EtcdConf{
				Hosts: []string{"127.0.0.1:2379"},
			},
		},
		MySQL: xmysql.Config{
			DataSource: testOrderMySQLDataSource,
		},
		Order: config.OrderConfig{
			CloseAfter: 15 * time.Minute,
		},
		RepeatGuard: config.RepeatGuardConfig{
			Prefix:             "/damai-go/tests/repeat-guard/order-create/",
			SessionTTL:         10,
			LockAcquireTimeout: 200 * time.Millisecond,
		},
	}

	t.Run("redis disabled when host empty", func(t *testing.T) {
		svcCtx := svc.NewServiceContext(baseConfig)
		if svcCtx.Redis != nil {
			t.Fatalf("expected redis client to be nil when host is empty")
		}
	})

	t.Run("redis enabled when host configured", func(t *testing.T) {
		cfg := baseConfig
		cfg.StoreRedis = xredis.Config{
			Host: "127.0.0.1:6379",
			Type: "node",
		}
		svcCtx := svc.NewServiceContext(cfg)
		if svcCtx.Redis == nil {
			t.Fatalf("expected redis client to be wired when host is configured")
		}
	})
}

func TestOrderLedgerNotReadyMappedToFailedPrecondition(t *testing.T) {
	err := mapOrderError(xerr.ErrOrderLimitLedgerNotReady)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
}
