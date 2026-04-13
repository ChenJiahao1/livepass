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
	"damai-go/services/order-rpc/sharding"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/zrpc"
)

func TestServiceContextKafkaBuildsKafkaProducer(t *testing.T) {
	cfg := buildKafkaServiceContextConfig("ticketing.attempt.command.v1")
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

func TestServiceContextKafkaEnsuresKafkaTopicExists(t *testing.T) {
	topic := fmt.Sprintf("ticketing.attempt.command.test.%d", time.Now().UnixNano())
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

func TestServiceContextKafkaBuildsShardingResources(t *testing.T) {
	cfg := buildKafkaServiceContextConfig("ticketing.attempt.command.v1")
	svcCtx := svc.NewServiceContext(cfg)
	if svcCtx.SqlConn == nil {
		t.Fatalf("expected sql conn to be wired")
	}
	if len(svcCtx.ShardSqlConns) != 2 {
		t.Fatalf("expected 2 shard sql conns, got %d", len(svcCtx.ShardSqlConns))
	}
	if svcCtx.OrderRouteMap == nil {
		t.Fatalf("expected route map to be wired")
	}
	if svcCtx.OrderRouter == nil {
		t.Fatalf("expected order router to be wired")
	}

	route, err := svcCtx.OrderRouter.RouteByUserID(20260324001)
	if err != nil {
		t.Fatalf("RouteByUserID() error = %v", err)
	}
	expectedSlot := sharding.LogicSlotByUserID(20260324001)
	if route.LogicSlot != expectedSlot {
		t.Fatalf("route logic slot = %d, want %d", route.LogicSlot, expectedSlot)
	}
	if route.DBKey != "order-db-0" {
		t.Fatalf("route db key = %s, want order-db-0", route.DBKey)
	}
}

func TestServiceContextKafkaLoadOrderKafkaConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configFile := filepath.Join(dir, "order.yaml")
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
  TopicOrderCreate: ticketing.attempt.command.test
  ConsumerGroup: damai-go-ticketing-attempt
  TopicPartitions: 5
  ConsumerWorkers: 1
Sharding:
  Mode: shard_only
  Shards:
    order-db-0:
      DataSource: root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true
  RouteMap:
    Version: v1
    Entries:
      - Version: v1
        LogicSlot: 734
        DBKey: order-db-0
        TableSuffix: "00"
        Status: stable
        WriteMode: shard_primary
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
	if c.Sharding.Mode != "shard_only" {
		t.Fatalf("expected sharding mode shard_only, got %s", c.Sharding.Mode)
	}
	if c.Sharding.RouteMap.Version != "v1" {
		t.Fatalf("expected route map version v1, got %s", c.Sharding.RouteMap.Version)
	}
	if len(c.Sharding.RouteMap.Entries) != 1 {
		t.Fatalf("expected 1 route map entry, got %d", len(c.Sharding.RouteMap.Entries))
	}
}

func TestServiceContextKafkaLoadOrderKafkaConfigDefaults(t *testing.T) {
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
Sharding:
  Shards:
    order-db-0:
      DataSource: root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true
  RouteMap:
    Version: v1
    Entries:
      - Version: v1
        LogicSlot: 734
        DBKey: order-db-0
        TableSuffix: "00"
        Status: stable
        WriteMode: shard_primary
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
	if c.Sharding.Mode != "shard_only" {
		t.Fatalf("expected default sharding mode shard_only, got %s", c.Sharding.Mode)
	}
}

func buildKafkaServiceContextConfig(topic string) config.Config {
	logicSlot := sharding.LogicSlotByUserID(20260324001)
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
			ConsumerGroup:    "damai-go-ticketing-attempt",
			TopicPartitions:  5,
			ConsumerWorkers:  1,
			ProducerTimeout:  3 * time.Second,
			RetryBackoff:     time.Second,
		},
		Sharding: config.ShardingConfig{
			Mode: "shard_only",
			Shards: map[string]xmysql.Config{
				"order-db-0": {
					DataSource: testOrderMySQLDataSource,
				},
				"order-db-1": {
					DataSource: testOrderMySQLDataSource,
				},
			},
			RouteMap: config.RouteMapConfig{
				Version: "v1",
				Entries: []config.RouteEntryConfig{
					{
						Version:     "v1",
						LogicSlot:   logicSlot,
						DBKey:       "order-db-0",
						TableSuffix: "00",
						Status:      sharding.RouteStatusStable,
						WriteMode:   sharding.WriteModeShardPrimary,
					},
				},
			},
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
