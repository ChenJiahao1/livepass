package integration_test

import (
	"testing"
	"time"

	"damai-go/pkg/xmysql"
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/zrpc"
)

func TestNewOrderServiceContextBuildsKafkaProducer(t *testing.T) {
	cfg := config.Config{
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
		Kafka: config.KafkaConfig{
			Brokers:          []string{"127.0.0.1:9092"},
			TopicOrderCreate: "order.create.command.v1",
			ConsumerGroup:    "damai-go-order-create",
			MaxMessageDelay:  5 * time.Second,
			ProducerTimeout:  3 * time.Second,
			RetryBackoff:     time.Second,
		},
	}

	svcCtx := svc.NewServiceContext(cfg)
	if svcCtx.OrderCreateProducer == nil {
		t.Fatalf("expected kafka producer to be wired")
	}
	if svcCtx.OrderCreateConsumer == nil {
		t.Fatalf("expected kafka consumer to be wired")
	}
	t.Cleanup(func() {
		_ = svcCtx.OrderCreateConsumer.Close()
		_ = svcCtx.OrderCreateProducer.Close()
	})
}
