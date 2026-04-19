package logic

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"livepass/services/order-rpc/internal/rush"
)

type processingLease struct {
	lost atomic.Bool
	stop func()
}

func startProcessingLease(ctx context.Context, store *rush.AttemptStore, showTimeID, orderNumber int64, interval time.Duration) *processingLease {
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}

	leaseCtx, cancel := context.WithCancel(ctx)
	lease := &processingLease{
		stop: cancel,
	}
	if store == nil || showTimeID <= 0 || orderNumber <= 0 {
		lease.lost.Store(true)
		return lease
	}

	var once sync.Once
	lease.stop = func() {
		once.Do(cancel)
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				refreshed, err := store.RefreshProcessingLease(leaseCtx, showTimeID, orderNumber, time.Now())
				if err != nil || !refreshed {
					lease.lost.Store(true)
					return
				}
			}
		}
	}()

	return lease
}
