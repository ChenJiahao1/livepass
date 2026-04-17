package programcache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"livepass/pkg/xredis"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

type PubSubPublisher interface {
	Publish(ctx context.Context, msg InvalidationMessage) error
}

type redisPublisherClient interface {
	Publish(ctx context.Context, channel string, message any) *redis.IntCmd
	Close() error
}

type RedisPubSubPublisher struct {
	client         redisPublisherClient
	channel        string
	publishTimeout time.Duration
}

func NewRedisPubSubPublisher(store *xredis.Client, channel string, publishTimeout time.Duration) (*RedisPubSubPublisher, error) {
	client, err := buildGoRedisClient(store)
	if err != nil {
		return nil, err
	}

	return &RedisPubSubPublisher{
		client:         client,
		channel:        channel,
		publishTimeout: publishTimeout,
	}, nil
}

func (p *RedisPubSubPublisher) Publish(ctx context.Context, msg InvalidationMessage) error {
	if p == nil || p.client == nil {
		return errors.New("pubsub publisher is nil")
	}

	payload, err := MarshalInvalidationMessage(msg)
	if err != nil {
		return err
	}

	publishCtx := ctx
	cancel := func() {}
	if p.publishTimeout > 0 {
		publishCtx, cancel = context.WithTimeout(ctx, p.publishTimeout)
	}
	defer cancel()

	return p.client.Publish(publishCtx, p.channel, payload).Err()
}

type pubSubConn interface {
	ReceiveMessage(ctx context.Context) (*redis.Message, error)
	Close() error
}

type redisSubscriberClient interface {
	Subscribe(ctx context.Context, channels ...string) pubSubConn
	Close() error
}

type redisSubscriberAdapter struct {
	client redis.UniversalClient
}

func (a *redisSubscriberAdapter) Subscribe(ctx context.Context, channels ...string) pubSubConn {
	return a.client.Subscribe(ctx, channels...)
}

func (a *redisSubscriberAdapter) Close() error {
	return a.client.Close()
}

type redisClientFactory func() (redisSubscriberClient, error)

type PubSubSubscriber struct {
	redis            *xredis.Client
	channel          string
	registry         *InvalidationRegistry
	reconnectBackoff time.Duration
	newClient        redisClientFactory
	closed           chan struct{}
	once             sync.Once
	mu               sync.Mutex
	cancel           context.CancelFunc
	activeSub        pubSubConn
}

func NewPubSubSubscriber(store *xredis.Client, channel string, registry *InvalidationRegistry, publishTimeout, reconnectBackoff time.Duration) *PubSubSubscriber {
	return newPubSubSubscriber(store, channel, registry, publishTimeout, reconnectBackoff, nil)
}

func newPubSubSubscriber(store *xredis.Client, channel string, registry *InvalidationRegistry, publishTimeout, reconnectBackoff time.Duration, newClient redisClientFactory) *PubSubSubscriber {
	if newClient == nil {
		newClient = func() (redisSubscriberClient, error) {
			client, err := buildGoRedisClient(store)
			if err != nil {
				return nil, err
			}
			return &redisSubscriberAdapter{client: client}, nil
		}
	}

	_ = publishTimeout

	return &PubSubSubscriber{
		redis:            store,
		channel:          channel,
		registry:         registry,
		reconnectBackoff: reconnectBackoff,
		newClient:        newClient,
		closed:           make(chan struct{}),
	}
}

func (s *PubSubSubscriber) Start(ctx context.Context) {
	if s == nil {
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	s.setCancel(cancel)
	defer s.clearCancel(cancel)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		default:
		}

		client, err := s.newClient()
		if err != nil {
			logx.Error(err)
			if !s.waitBackoff(ctx) {
				return
			}
			continue
		}
		if client == nil {
			logx.Error(errors.New("pubsub redis client is nil"))
			if !s.waitBackoff(ctx) {
				return
			}
			continue
		}

		s.consume(ctx, client)
		_ = client.Close()

		if !s.waitBackoff(ctx) {
			return
		}
	}
}

func (s *PubSubSubscriber) Close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		s.closeActive()
		close(s.closed)
	})
}

func (s *PubSubSubscriber) consume(ctx context.Context, client redisSubscriberClient) {
	if client == nil {
		return
	}

	sub := client.Subscribe(ctx, s.channel)
	s.setActiveSub(sub)
	defer func() {
		s.setActiveSub(nil)
		_ = sub.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		default:
		}

		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			logx.Error(err)
			return
		}
		if err := s.handlePayload(msg.Payload); err != nil {
			logx.Error(err)
		}
	}
}

func (s *PubSubSubscriber) handlePayload(payload string) error {
	msg, err := ParseInvalidationMessage([]byte(payload))
	if err != nil {
		return err
	}
	if s.registry == nil {
		return errors.New("invalidation registry is nil")
	}

	return s.registry.Dispatch(msg)
}

func (s *PubSubSubscriber) waitBackoff(ctx context.Context) bool {
	if s.reconnectBackoff <= 0 {
		return true
	}

	timer := time.NewTimer(s.reconnectBackoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-s.closed:
		return false
	case <-timer.C:
		return true
	}
}

func buildGoRedisClient(store *xredis.Client) (redis.UniversalClient, error) {
	if store == nil {
		return nil, errors.New("redis client is required")
	}

	addrs := splitRedisAddrs(store.Addr)
	if len(addrs) == 0 {
		return nil, errors.New("redis addr is empty")
	}

	opts := &redis.UniversalOptions{
		Addrs:    addrs,
		Username: store.User,
		Password: store.Pass,
	}
	if store.Type == "cluster" {
		opts.IsClusterMode = true
	}

	return redis.NewUniversalClient(opts), nil
}

func (s *PubSubSubscriber) setCancel(cancel context.CancelFunc) {
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
}

func (s *PubSubSubscriber) clearCancel(cancel context.CancelFunc) {
	s.mu.Lock()
	s.cancel = nil
	s.mu.Unlock()
}

func (s *PubSubSubscriber) setActiveSub(sub pubSubConn) {
	s.mu.Lock()
	s.activeSub = sub
	s.mu.Unlock()
}

func (s *PubSubSubscriber) closeActive() {
	s.mu.Lock()
	cancel := s.cancel
	sub := s.activeSub
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if sub != nil {
		_ = sub.Close()
	}
}

func splitRedisAddrs(addr string) []string {
	if addr == "" {
		return nil
	}

	raw := strings.Split(addr, ",")
	addrs := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		addrs = append(addrs, item)
	}

	return addrs
}
