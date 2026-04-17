package mq

import (
	"context"
	"errors"

	"livepass/services/order-rpc/internal/config"

	"github.com/segmentio/kafka-go"
)

type OrderCreateConsumer interface {
	Start(ctx context.Context, handler func(context.Context, []byte) error) error
	Close() error
}

type OrderCreateConsumerFactory interface {
	New(cfg config.KafkaConfig) OrderCreateConsumer
}

type kafkaOrderCreateConsumerFactory struct{}

type kafkaOrderCreateConsumer struct {
	reader *kafka.Reader
}

func NewOrderCreateConsumerFactory() OrderCreateConsumerFactory {
	return kafkaOrderCreateConsumerFactory{}
}

func (kafkaOrderCreateConsumerFactory) New(cfg config.KafkaConfig) OrderCreateConsumer {
	return NewOrderCreateConsumer(cfg)
}

func NewOrderCreateConsumer(cfg config.KafkaConfig) OrderCreateConsumer {
	return &kafkaOrderCreateConsumer{
		reader: newOrderCreateReader(cfg),
	}
}

func newOrderCreateReader(cfg config.KafkaConfig) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          OrderCreateTopic(cfg),
		GroupID:        OrderCreateConsumerGroup(cfg),
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxAttempts:    3,
		ReadBackoffMin: cfg.RetryBackoff,
		ReadBackoffMax: cfg.RetryBackoff,
	})
}

func (c *kafkaOrderCreateConsumer) Start(ctx context.Context, handler func(context.Context, []byte) error) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}

			return err
		}

		if err := handler(ctx, msg.Value); err != nil {
			return err
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}

			return err
		}
	}
}

func (c *kafkaOrderCreateConsumer) Close() error {
	if c == nil || c.reader == nil {
		return nil
	}

	return c.reader.Close()
}
