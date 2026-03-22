package xid

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
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

type Config struct {
	Hosts           []string
	Prefix          string
	Service         string
	SessionTTL      int64
	DialTimeout     time.Duration
	AllocateTimeout time.Duration
}

type generator struct {
	node            *snowflake.Node
	client          *clientv3.Client
	closeClient     bool
	nodeID          int64
	leaseID         clientv3.LeaseID
	keepAliveCancel context.CancelFunc
	closeOnce       sync.Once
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

	return gen.node.Generate().Int64()
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
	}

	go drainKeepAlive(keepAliveCh)

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

func drainKeepAlive(ch <-chan *clientv3.LeaseKeepAliveResponse) {
	for range ch {
	}
}

func (g *generator) close() error {
	var closeErr error

	g.closeOnce.Do(func() {
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
