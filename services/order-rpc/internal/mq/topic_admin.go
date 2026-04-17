package mq

import (
	"context"
	"errors"
	"time"

	"livepass/services/order-rpc/internal/config"

	"github.com/segmentio/kafka-go"
)

func EnsureOrderCreateTopic(cfg config.KafkaConfig) error {
	if len(cfg.Brokers) == 0 {
		return nil
	}

	current, err := readOrderCreateTopicPartitionCount(cfg)
	if err != nil {
		return err
	}
	if current > 0 {
		return nil
	}

	partitions := cfg.TopicPartitions
	if partitions <= 0 {
		partitions = 1
	}

	timeout := cfg.ProducerTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", cfg.Brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()

	err = conn.CreateTopics(kafka.TopicConfig{
		Topic:             OrderCreateTopic(cfg),
		NumPartitions:     partitions,
		ReplicationFactor: 1,
	})
	if err != nil && !errors.Is(err, kafka.TopicAlreadyExists) {
		return err
	}

	return nil
}

func OrderCreateTopicPartitionCount(cfg config.KafkaConfig) (int, error) {
	timeout := cfg.ProducerTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	deadline := time.Now().Add(timeout)
	for {
		count, err := readOrderCreateTopicPartitionCount(cfg)
		if err != nil {
			return 0, err
		}
		if count > 0 || time.Now().After(deadline) {
			return count, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func readOrderCreateTopicPartitionCount(cfg config.KafkaConfig) (int, error) {
	if len(cfg.Brokers) == 0 {
		return 0, nil
	}

	timeout := cfg.ProducerTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", cfg.Brokers[0])
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		return 0, err
	}

	count := 0
	topic := OrderCreateTopic(cfg)
	for _, partition := range partitions {
		if partition.Topic == topic {
			count++
		}
	}

	return count, nil
}
