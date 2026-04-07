package mq

import (
	"context"
	"errors"
	"sync"
	"time"

	"damai-go/services/order-rpc/internal/config"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
)

type OrderCreateProducer interface {
	Send(ctx context.Context, key string, value []byte) error
	Close() error
}

type kafkaOrderCreateProducer struct {
	writer   *kafka.Writer
	timeout  time.Duration
	handoff  chan kafka.Message
	closedCh chan struct{}
	doneCh   chan struct{}
	mu       sync.RWMutex
	closed   bool
}

const orderCreateProducerBufferSize = 256

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
		timeout:  cfg.ProducerTimeout,
		handoff:  make(chan kafka.Message, orderCreateProducerBufferSize),
		closedCh: make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	go producer.run()

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
	if p.closed {
		p.mu.RUnlock()
		return errOrderCreateProducerClosed
	}
	handoff := p.handoff
	closedCh := p.closedCh
	p.mu.RUnlock()

	select {
	case handoff <- msg:
		return nil
	case <-closedCh:
		return errOrderCreateProducerClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *kafkaOrderCreateProducer) run() {
	defer close(p.doneCh)

	for {
		select {
		case msg := <-p.handoff:
			p.writeMessage(msg)
		case <-p.closedCh:
			p.drain()
			return
		}
	}
}

func (p *kafkaOrderCreateProducer) drain() {
	for {
		select {
		case msg := <-p.handoff:
			p.writeMessage(msg)
		default:
			if p.writer != nil {
				if err := p.writer.Close(); err != nil {
					logx.Errorf("close order create producer failed: %v", err)
				}
			}
			return
		}
	}
}

func (p *kafkaOrderCreateProducer) writeMessage(msg kafka.Message) {
	if p == nil || p.writer == nil {
		return
	}

	sendCtx := context.Background()
	if p.timeout > 0 {
		var cancel context.CancelFunc
		sendCtx, cancel = context.WithTimeout(sendCtx, p.timeout)
		defer cancel()
	}

	if err := p.writer.WriteMessages(sendCtx, msg); err != nil {
		logx.Errorf("write order create event failed, topic=%s err=%v", p.writer.Topic, err)
	}
}

func (p *kafkaOrderCreateProducer) Close() error {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		<-p.doneCh
		return nil
	}
	p.closed = true
	close(p.closedCh)
	p.mu.Unlock()

	<-p.doneCh
	return nil
}
