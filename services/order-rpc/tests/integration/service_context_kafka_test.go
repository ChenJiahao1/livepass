package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"damai-go/pkg/xmysql"
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/svc"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/zrpc"
)

func TestNewOrderServiceContextBuildsKafkaProducer(t *testing.T) {
	cfg := buildKafkaServiceContextConfig("order.create.command.v1")
	svcCtx := svc.NewServiceContext(cfg)
	if svcCtx.OrderCreateProducer == nil {
		t.Fatalf("expected kafka producer to be wired")
	}
	if svcCtx.OrderCreateConsumerFactory == nil {
		t.Fatalf("expected kafka consumer factory to be wired")
	}
	t.Cleanup(func() {
		_ = svcCtx.OrderCreateProducer.Close()
	})

	consumer := svcCtx.OrderCreateConsumerFactory.New(cfg.Kafka)
	if consumer == nil {
		t.Fatalf("expected kafka consumer to be creatable from factory")
	}
	t.Cleanup(func() {
		_ = consumer.Close()
	})
}

func TestNewOrderServiceContextEnsuresKafkaTopicExists(t *testing.T) {
	topic := fmt.Sprintf("order.create.command.test.%d", time.Now().UnixNano())
	cfg := buildKafkaServiceContextConfig(topic)

	svcCtx := svc.NewServiceContext(cfg)
	t.Cleanup(func() {
		_ = svcCtx.OrderCreateProducer.Close()
	})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if kafkaTopicExists(t, cfg.Kafka.Brokers[0], topic) {
			partitions := kafkaTopicPartitionCount(t, cfg.Kafka.Brokers[0], topic)
			if partitions != cfg.Kafka.TopicPartitions {
				t.Fatalf("expected kafka topic %q to have %d partitions, got %d", topic, cfg.Kafka.TopicPartitions, partitions)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("expected kafka topic %q to exist after service context init", topic)
}

func TestLoadOrderKafkaConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configFile := filepath.Join(dir, "order-rpc.yaml")
	content := []byte(`
Name: order.rpc
ListenOn: 0.0.0.0:8082
Etcd:
  Hosts:
    - 127.0.0.1:2379
  Key: order.rpc
MySQL:
  DataSource: root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true
ProgramRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:2379
    Key: program.rpc
PayRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:2379
    Key: pay.rpc
UserRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:2379
    Key: user.rpc
Kafka:
  Brokers:
    - 127.0.0.1:9094
  TopicOrderCreate: order.create.command.test
  ConsumerGroup: damai-go-order-create
  TopicPartitions: 5
  ConsumerWorkers: 1
  MaxMessageDelay: 60s
`)
	if err := os.WriteFile(configFile, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", configFile, err)
	}

	var c config.Config
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Kafka.TopicPartitions != 5 {
		t.Fatalf("expected topic partitions 5, got %d", c.Kafka.TopicPartitions)
	}
	if c.Kafka.ConsumerWorkers != 1 {
		t.Fatalf("expected consumer workers 1, got %d", c.Kafka.ConsumerWorkers)
	}
	if c.Kafka.MaxMessageDelay != 60*time.Second {
		t.Fatalf("expected max message delay 60s, got %s", c.Kafka.MaxMessageDelay)
	}
}

func TestLoadOrderKafkaConfigDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configFile := filepath.Join(dir, "order-rpc-defaults.yaml")
	content := []byte(`
Name: order.rpc
ListenOn: 0.0.0.0:8082
Etcd:
  Hosts:
    - 127.0.0.1:2379
  Key: order.rpc
MySQL:
  DataSource: root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true
ProgramRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:2379
    Key: program.rpc
PayRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:2379
    Key: pay.rpc
UserRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:2379
    Key: user.rpc
Kafka:
  Brokers:
    - 127.0.0.1:9094
`)
	if err := os.WriteFile(configFile, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", configFile, err)
	}

	var c config.Config
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Kafka.TopicPartitions != 1 {
		t.Fatalf("expected default topic partitions 1, got %d", c.Kafka.TopicPartitions)
	}
	if c.Kafka.ConsumerWorkers != 1 {
		t.Fatalf("expected default consumer workers 1, got %d", c.Kafka.ConsumerWorkers)
	}
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
			TopicPartitions:  5,
			ConsumerWorkers:  1,
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

func kafkaTopicPartitionCount(t *testing.T, broker, topic string) int {
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

	count := 0
	for _, partition := range partitions {
		if partition.Topic == topic {
			count++
		}
	}

	return count
}
