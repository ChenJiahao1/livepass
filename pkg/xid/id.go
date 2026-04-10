package xid

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/snowflake"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	defaultPrefix                = "/damai-go/snowflake/nodes/"
	defaultSessionTTL      int64 = 10
	defaultDialTimeout           = 500 * time.Millisecond
	defaultAllocateTimeout       = 2 * time.Second
	maxNodeID              int64 = 1023
)

var global struct {
	mu  sync.RWMutex
	gen *generator
}

const (
	generatorStateReady uint32 = iota
	generatorStateClosed
	generatorStateLeaseLost
)

var (
	ErrLeaseLost       = errors.New("xid lease lost")
	errGeneratorClosed = errors.New("xid generator closed")
)

type Config struct {
	Hosts           []string
	Prefix          string
	Service         string
	SessionTTL      int64
	DialTimeout     time.Duration
	AllocateTimeout time.Duration
	OnLeaseLost     func(error)
}

type generator struct {
	node            *snowflake.Node
	client          *clientv3.Client
	closeClient     bool
	nodeID          int64
	leaseID         clientv3.LeaseID
	keepAliveCancel context.CancelFunc
	closeOnce       sync.Once
	sessionTTL      time.Duration
	state           atomic.Uint32
	onLeaseLost     func(error)
}

func InitEtcd(ctx context.Context, cfg Config) error {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.gen != nil {
		return errors.New("xid already initialized")
	}

	gen, err := newGenerator(ctx, cfg)
	if err != nil {
		return err
	}

	global.gen = gen
	return nil
}

func MustInitEtcd(ctx context.Context, cfg Config) {
	if err := InitEtcd(ctx, cfg); err != nil {
		panic(err)
	}
}

func New() int64 {
	global.mu.RLock()
	gen := global.gen
	global.mu.RUnlock()

	if gen == nil {
		panic("xid not initialized")
	}

	id, err := gen.newID()
	if err != nil {
		panic(err)
	}

	return id
}

func Close() error {
	global.mu.Lock()
	gen := global.gen
	global.gen = nil
	global.mu.Unlock()

	if gen == nil {
		return nil
	}

	return gen.close()
}

func newGenerator(ctx context.Context, cfg Config) (*generator, error) {
	cfg = normalizeConfig(cfg)
	if len(cfg.Hosts) == 0 {
		return nil, errors.New("xid etcd hosts required")
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Hosts,
		DialTimeout: cfg.DialTimeout,
	})
	if err != nil {
		return nil, err
	}

	gen, err := newGeneratorWithClient(ctx, client, cfg)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	gen.closeClient = true
	return gen, nil
}

func newGeneratorWithClient(ctx context.Context, client *clientv3.Client, cfg Config) (*generator, error) {
	cfg = normalizeConfig(cfg)

	allocateCtx, cancel := context.WithTimeout(ctx, cfg.AllocateTimeout)
	defer cancel()

	leaseResp, err := client.Grant(allocateCtx, cfg.SessionTTL)
	if err != nil {
		return nil, err
	}

	nodeID, err := allocateNodeID(allocateCtx, client, leaseResp.ID, cfg)
	if err != nil {
		_, _ = client.Revoke(context.Background(), leaseResp.ID)
		return nil, err
	}

	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		_, _ = client.Revoke(context.Background(), leaseResp.ID)
		return nil, err
	}

	keepAliveCtx, keepAliveCancel := context.WithCancel(context.Background())
	keepAliveCh, err := client.KeepAlive(keepAliveCtx, leaseResp.ID)
	if err != nil {
		keepAliveCancel()
		_, _ = client.Revoke(context.Background(), leaseResp.ID)
		return nil, err
	}

	gen := &generator{
		node:            node,
		client:          client,
		nodeID:          nodeID,
		leaseID:         leaseResp.ID,
		keepAliveCancel: keepAliveCancel,
		sessionTTL:      time.Duration(cfg.SessionTTL) * time.Second,
		onLeaseLost:     cfg.OnLeaseLost,
	}

	go gen.watchKeepAlive(keepAliveCh)

	return gen, nil
}

func allocateNodeID(ctx context.Context, client *clientv3.Client, leaseID clientv3.LeaseID, cfg Config) (int64, error) {
	payload := leaseValue(cfg)

	for nodeID := int64(0); nodeID <= maxNodeID; nodeID++ {
		key := cfg.Prefix + strconv.FormatInt(nodeID, 10)
		resp, err := client.Txn(ctx).
			If(clientv3.Compare(clientv3.Version(key), "=", 0)).
			Then(clientv3.OpPut(key, payload, clientv3.WithLease(leaseID))).
			Commit()
		if err != nil {
			return 0, err
		}
		if resp.Succeeded {
			return nodeID, nil
		}
	}

	return 0, errors.New("no available snowflake node id")
}

func leaseValue(cfg Config) string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}

	service := cfg.Service
	if service == "" {
		service = "unknown-service"
	}

	return fmt.Sprintf(
		"service=%s host=%s pid=%d started_at=%s",
		service,
		hostname,
		os.Getpid(),
		time.Now().UTC().Format(time.RFC3339),
	)
}

func normalizeConfig(cfg Config) Config {
	if cfg.Prefix == "" {
		cfg.Prefix = defaultPrefix
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = defaultSessionTTL
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = defaultDialTimeout
	}
	if cfg.AllocateTimeout <= 0 {
		cfg.AllocateTimeout = defaultAllocateTimeout
	}

	return cfg
}

func (g *generator) newID() (int64, error) {
	switch g.state.Load() {
	case generatorStateLeaseLost:
		return 0, ErrLeaseLost
	case generatorStateClosed:
		return 0, errGeneratorClosed
	default:
		return g.node.Generate().Int64(), nil
	}
}

func (g *generator) watchKeepAlive(ch <-chan *clientv3.LeaseKeepAliveResponse) {
	timer := time.NewTimer(g.keepAliveTimeout(0))
	defer timer.Stop()

	for {
		select {
		case resp, ok := <-ch:
			if !ok {
				g.markLeaseLost()
				return
			}
			if resp == nil || resp.TTL <= 0 {
				g.markLeaseLost()
				return
			}

			resetTimer(timer, g.keepAliveTimeout(resp.TTL))
		case <-timer.C:
			g.markLeaseLost()
			return
		}
	}
}

func (g *generator) keepAliveTimeout(ttl int64) time.Duration {
	if ttl > 0 {
		return time.Duration(ttl) * time.Second
	}
	if g.sessionTTL > 0 {
		return g.sessionTTL
	}

	return time.Second
}

func resetTimer(timer *time.Timer, d time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	timer.Reset(d)
}

func (g *generator) markLeaseLost() {
	if !g.state.CompareAndSwap(generatorStateReady, generatorStateLeaseLost) {
		return
	}

	if g.onLeaseLost != nil {
		g.onLeaseLost(ErrLeaseLost)
		return
	}

	os.Exit(1)
}

func (g *generator) close() error {
	var closeErr error

	g.closeOnce.Do(func() {
		g.state.CompareAndSwap(generatorStateReady, generatorStateClosed)

		if g.keepAliveCancel != nil {
			g.keepAliveCancel()
		}

		if g.client != nil && g.leaseID != 0 {
			ctx, cancel := context.WithTimeout(context.Background(), defaultAllocateTimeout)
			defer cancel()

			_, err := g.client.Revoke(ctx, g.leaseID)
			if err != nil {
				closeErr = err
			}
		}

		if g.closeClient && g.client != nil {
			if err := g.client.Close(); closeErr == nil {
				closeErr = err
			}
		}
	})

	return closeErr
}
