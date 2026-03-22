package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/mq"

	"github.com/segmentio/kafka-go"
)

const testKafkaBroker = "127.0.0.1:9094"

func TestEnsureOrderCreateTopicCreatesConfiguredPartitions(t *testing.T) {
	topic := fmt.Sprintf("order.create.command.test.%d", time.Now().UnixNano())
	cfg := buildTopicAdminKafkaConfig(topic, 4)

	if err := mq.EnsureOrderCreateTopic(cfg); err != nil {
		t.Fatalf("ensure order create topic: %v", err)
	}

	got, err := mq.OrderCreateTopicPartitionCount(cfg)
	if err != nil {
		t.Fatalf("get order create topic partition count: %v", err)
	}
	if got != 4 {
		t.Fatalf("expected 4 partitions, got %d", got)
	}
}

func TestEnsureOrderCreateTopicLeavesExistingTopicWhenPartitionsAreLower(t *testing.T) {
	topic := fmt.Sprintf("order.create.command.test.%d", time.Now().UnixNano())
	createKafkaTopic(t, testKafkaBroker, topic, 2)

	cfg := buildTopicAdminKafkaConfig(topic, 4)
	if err := mq.EnsureOrderCreateTopic(cfg); err != nil {
		t.Fatalf("ensure order create topic: %v", err)
	}

	got, err := mq.OrderCreateTopicPartitionCount(cfg)
	if err != nil {
		t.Fatalf("get order create topic partition count: %v", err)
	}
	if got != 2 {
		t.Fatalf("expected existing topic to stay at 2 partitions, got %d", got)
	}
}

func TestGetOrderCreateTopicPartitionCount(t *testing.T) {
	topic := fmt.Sprintf("order.create.command.test.%d", time.Now().UnixNano())
	createKafkaTopic(t, testKafkaBroker, topic, 3)

	got, err := mq.OrderCreateTopicPartitionCount(buildTopicAdminKafkaConfig(topic, 4))
	if err != nil {
		t.Fatalf("get order create topic partition count: %v", err)
	}
	if got != 3 {
		t.Fatalf("expected 3 partitions, got %d", got)
	}
}

func buildTopicAdminKafkaConfig(topic string, partitions int) config.KafkaConfig {
	return config.KafkaConfig{
		Brokers:          []string{testKafkaBroker},
		TopicOrderCreate: topic,
		ConsumerGroup:    "damai-go-order-create",
		TopicPartitions:  partitions,
		ConsumerWorkers:  partitions,
		ProducerTimeout:  3 * time.Second,
		RetryBackoff:     time.Second,
	}
}

func createKafkaTopic(t *testing.T, broker, topic string, partitions int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", broker)
	if err != nil {
		t.Fatalf("dial kafka broker: %v", err)
	}
	defer conn.Close()

	if err := conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     partitions,
		ReplicationFactor: 1,
	}); err != nil {
		t.Fatalf("create kafka topic %q: %v", topic, err)
	}
}
