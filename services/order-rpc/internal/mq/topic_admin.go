package mq

import (
	"context"
	"errors"
	"time"

	"damai-go/services/order-rpc/internal/config"

	"github.com/segmentio/kafka-go"
)

func EnsureOrderCreateTopic(cfg config.KafkaConfig) error {
	if len(cfg.Brokers) == 0 {
		return nil
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
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	if err != nil && !errors.Is(err, kafka.TopicAlreadyExists) {
		return err
	}

	return nil
}
