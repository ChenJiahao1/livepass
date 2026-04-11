package programcache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestPubSubSubscriberRejectsMalformedPayload(t *testing.T) {
	subscriber := NewPubSubSubscriber(nil, "program-cache", nil, time.Second, time.Second)

	payload := `{"version":"v1","service":"program-rpc","instance_id":"node-1","published_at":"2026-04-11T00:00:00Z","entries":[]}`
	if err := subscriber.handlePayload(payload); err == nil {
		t.Fatalf("expected malformed payload to return error")
	}
}

func TestPubSubSubscriberCloseStopsBlockingReceive(t *testing.T) {
	pubsub := &stubPubSub{
		started: make(chan struct{}),
		closed:  make(chan struct{}),
	}
	client := &stubSubscriberClient{
		pubsub: pubsub,
	}
	subscriber := newPubSubSubscriber(nil, "program-cache", &InvalidationRegistry{}, 0, 5*time.Millisecond, func() (redisSubscriberClient, error) {
		return client, nil
	})

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		subscriber.Start(ctx)
		close(done)
	}()

	select {
	case <-pubsub.started:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected ReceiveMessage to start")
	}

	subscriber.Close()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected Start to exit after Close")
	}
	if !client.closed {
		t.Fatalf("expected client to be closed")
	}
	if !pubsub.closedOnce {
		t.Fatalf("expected pubsub to be closed")
	}
}

func TestRedisPubSubPublisherSkipsTimeoutWhenDisabled(t *testing.T) {
	client := &stubPublisherClient{}
	publisher := &RedisPubSubPublisher{
		client:         client,
		channel:        "program-cache",
		publishTimeout: 0,
	}

	if err := publisher.Publish(context.Background(), InvalidationMessage{
		Version:    "v1",
		Service:    "program-rpc",
		InstanceID: "node-1",
		Entries: []InvalidationEntry{
			{Cache: cacheCategorySnapshot},
		},
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	if client.deadlineSet {
		t.Fatalf("expected publish context without deadline when timeout disabled")
	}
}

type stubSubscriberClient struct {
	pubsub *stubPubSub
	closed bool
}

func (s *stubSubscriberClient) Subscribe(_ context.Context, _ ...string) pubSubConn {
	return s.pubsub
}

func (s *stubSubscriberClient) Close() error {
	s.closed = true
	return nil
}

type stubPubSub struct {
	started    chan struct{}
	closed     chan struct{}
	closedOnce bool
}

func (s *stubPubSub) ReceiveMessage(ctx context.Context) (*redis.Message, error) {
	select {
	case <-s.started:
	default:
		close(s.started)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.closed:
		return nil, errors.New("pubsub closed")
	}
}

func (s *stubPubSub) Close() error {
	if !s.closedOnce {
		close(s.closed)
		s.closedOnce = true
	}
	return nil
}

type stubPublisherClient struct {
	deadlineSet bool
}

func (s *stubPublisherClient) Publish(ctx context.Context, _ string, _ any) *redis.IntCmd {
	_, ok := ctx.Deadline()
	s.deadlineSet = ok
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(1)
	return cmd
}

func (s *stubPublisherClient) Close() error {
	return nil
}
