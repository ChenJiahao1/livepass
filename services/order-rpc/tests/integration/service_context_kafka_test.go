package integration_test

import (
	"fmt"
	"testing"
	"time"

	"damai-go/pkg/xmysql"
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/svc"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/zrpc"
)

func TestNewOrderServiceContextBuildsKafkaProducer(t *testing.T) {
	cfg := buildKafkaServiceContextConfig("order.create.command.v1")
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

func TestNewOrderServiceContextEnsuresKafkaTopicExists(t *testing.T) {
	topic := fmt.Sprintf("order.create.command.test.%d", time.Now().UnixNano())
	cfg := buildKafkaServiceContextConfig(topic)

	svcCtx := svc.NewServiceContext(cfg)
	t.Cleanup(func() {
		_ = svcCtx.OrderCreateConsumer.Close()
		_ = svcCtx.OrderCreateProducer.Close()
	})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if kafkaTopicExists(t, cfg.Kafka.Brokers[0], topic) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("expected kafka topic %q to exist after service context init", topic)
}

func buildKafkaServiceContextConfig(topic string) config.Config {
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
			Brokers:          []string{"127.0.0.1:9094"},
			TopicOrderCreate: topic,
			ConsumerGroup:    "damai-go-order-create",
			MaxMessageDelay:  5 * time.Second,
			ProducerTimeout:  3 * time.Second,
			RetryBackoff:     time.Second,
		},
	}

	return cfg
}

func kafkaTopicExists(t *testing.T, broker, topic string) bool {
	t.Helper()

	conn, err := kafka.Dial("tcp", broker)
	if err != nil {
		t.Fatalf("dial kafka broker: %v", err)
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		t.Fatalf("read kafka partitions: %v", err)
	}
	for _, partition := range partitions {
		if partition.Topic == topic {
			return true
		}
	}

	return false
}
