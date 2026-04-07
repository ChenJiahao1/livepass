package logic

import (
	"fmt"
	"sync"

	"damai-go/services/program-rpc/internal/svc"
)

type seatFreezeKeyedLocker struct {
	mu      sync.Mutex
	entries map[string]*seatFreezeLockEntry
}

type seatFreezeLockEntry struct {
	mu   sync.Mutex
	refs int
}

var defaultSeatFreezeLocker = newSeatFreezeKeyedLocker()

func newSeatFreezeKeyedLocker() *seatFreezeKeyedLocker {
	return &seatFreezeKeyedLocker{
		entries: make(map[string]*seatFreezeLockEntry),
	}
}

func (l *seatFreezeKeyedLocker) Lock(key string) func() {
	entry := l.acquireEntry(key)
	entry.mu.Lock()

	return func() {
		entry.mu.Unlock()
		l.releaseEntry(key)
	}
}

func (l *seatFreezeKeyedLocker) acquireEntry(key string) *seatFreezeLockEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok {
		entry = &seatFreezeLockEntry{}
		l.entries[key] = entry
	}
	entry.refs++
	return entry
}

func (l *seatFreezeKeyedLocker) releaseEntry(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok {
		return
	}

	entry.refs--
	if entry.refs <= 0 {
		delete(l.entries, key)
	}
}

func ensureSeatFreezeLocker(svcCtx *svc.ServiceContext) svc.SeatFreezeLocker {
	if svcCtx.SeatFreezeLocker == nil {
		svcCtx.SeatFreezeLocker = defaultSeatFreezeLocker
	}

	return svcCtx.SeatFreezeLocker
}

func seatFreezeLockKey(showTimeID, ticketCategoryID int64) string {
	return fmt.Sprintf("seat_freeze:%d:%d", showTimeID, ticketCategoryID)
}

func seatFreezeTokenLockKey(freezeToken string) string {
	return fmt.Sprintf("seat_freeze_token:%s", freezeToken)
}
