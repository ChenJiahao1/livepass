package mq

import (
	"context"
	"errors"
	"sync"
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
	mu      sync.RWMutex
	closed  bool
}

var errOrderCreateProducerClosed = errors.New("order create producer is closed")

func NewOrderCreateProducer(cfg config.KafkaConfig) OrderCreateProducer {
	producer := &kafkaOrderCreateProducer{
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

	return producer
}

func (p *kafkaOrderCreateProducer) Send(ctx context.Context, key string, value []byte) error {
	if p == nil {
		return errOrderCreateProducerClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: append([]byte(nil), value...),
		Time:  time.Now(),
	}

	p.mu.RLock()
	writer := p.writer
	closed := p.closed
	timeout := p.timeout
	p.mu.RUnlock()
	if closed || writer == nil {
		return errOrderCreateProducerClosed
	}
	sendCtx := ctx
	if sendCtx == nil {
		sendCtx = context.Background()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		if _, hasDeadline := sendCtx.Deadline(); !hasDeadline {
			sendCtx, cancel = context.WithTimeout(sendCtx, timeout)
			defer cancel()
		}
	}

	return writer.WriteMessages(sendCtx, msg)
}

func (p *kafkaOrderCreateProducer) Close() error {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	writer := p.writer
	p.writer = nil
	p.mu.Unlock()

	if writer == nil {
		return nil
	}

	return writer.Close()
}
