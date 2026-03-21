package repeatguard

import (
	"context"
	"errors"
	"time"

	"damai-go/services/order-rpc/internal/config"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EtcdGuard struct {
	client         *clientv3.Client
	prefix         string
	sessionTTL     int
	acquireTimeout time.Duration
}

func NewEtcdGuard(client *clientv3.Client, cfg config.RepeatGuardConfig) *EtcdGuard {
	acquireTimeout := cfg.LockAcquireTimeout
	if acquireTimeout <= 0 {
		acquireTimeout = 200 * time.Millisecond
	}

	sessionTTL := cfg.SessionTTL
	if sessionTTL <= 0 {
		sessionTTL = 10
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "/damai-go/repeat-guard/order-create/"
	}

	return &EtcdGuard{
		client:         client,
		prefix:         prefix,
		sessionTTL:     sessionTTL,
		acquireTimeout: acquireTimeout,
	}
}

func (g *EtcdGuard) Lock(ctx context.Context, key string) (UnlockFunc, error) {
	sessionCtx, cancelSession := context.WithTimeout(ctx, g.acquireTimeout)
	defer cancelSession()

	session, err := concurrency.NewSession(
		g.client,
		concurrency.WithTTL(g.sessionTTL),
		concurrency.WithContext(sessionCtx),
	)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	lockCtx, cancelLock := context.WithTimeout(ctx, g.acquireTimeout)
	defer cancelLock()

	mutex := concurrency.NewMutex(session, g.prefix+key)
	if err := mutex.TryLock(lockCtx); err != nil {
		_ = session.Close()
		if errors.Is(err, concurrency.ErrLocked) {
			return nil, ErrLocked
		}

		return nil, status.Error(codes.Unavailable, err.Error())
	}

	return func() {
		unlockCtx, cancelUnlock := context.WithTimeout(context.Background(), g.acquireTimeout)
		defer cancelUnlock()
		_ = mutex.Unlock(unlockCtx)
		_ = session.Close()
	}, nil
}
