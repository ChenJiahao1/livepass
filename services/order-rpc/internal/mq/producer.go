package mq

import (
	"context"
	"time"

	"damai-go/services/order-rpc/internal/config"

	"github.com/segmentio/kafka-go"
)

type OrderCreateProducer interface {
	Send(ctx context.Context, key string, value []byte) error
	Close() error
}

type kafkaOrderCreateProducer struct {
	writer  *kafka.Writer
	timeout time.Duration
}

func NewOrderCreateProducer(cfg config.KafkaConfig) OrderCreateProducer {
	return &kafkaOrderCreateProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(cfg.Brokers...),
			Topic:        OrderCreateTopic(cfg),
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			MaxAttempts:  3,
			WriteTimeout: cfg.ProducerTimeout,
		},
		timeout: cfg.ProducerTimeout,
	}
}

func (p *kafkaOrderCreateProducer) Send(ctx context.Context, key string, value []byte) error {
	sendCtx := ctx
	if p.timeout > 0 {
		var cancel context.CancelFunc
		sendCtx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
	}

	return p.writer.WriteMessages(sendCtx, kafka.Message{
		Key:   []byte(key),
		Value: value,
		Time:  time.Now(),
	})
}

func (p *kafkaOrderCreateProducer) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}

	return p.writer.Close()
}
