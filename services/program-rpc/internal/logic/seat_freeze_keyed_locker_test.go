package logic

import (
	"testing"
	"time"
)

func TestSeatFreezeKeyedLockerSerializesSameKey(t *testing.T) {
	locker := newSeatFreezeKeyedLocker()

	unlockFirst := locker.Lock("program:1:category:1")
	acquiredSecond := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		unlockSecond := locker.Lock("program:1:category:1")
		close(acquiredSecond)
		unlockSecond()
	}()

	select {
	case <-acquiredSecond:
		t.Fatal("expected same key to block until first unlock")
	case <-time.After(50 * time.Millisecond):
	}

	unlockFirst()

	select {
	case <-acquiredSecond:
	case <-time.After(time.Second):
		t.Fatal("expected second waiter to acquire after first unlock")
	}

	<-done
}

func TestSeatFreezeKeyedLockerAllowsDifferentKeysInParallel(t *testing.T) {
	locker := newSeatFreezeKeyedLocker()

	unlockFirst := locker.Lock("program:1:category:1")
	defer unlockFirst()

	acquired := make(chan struct{})
	go func() {
		unlockSecond := locker.Lock("program:1:category:2")
		close(acquired)
		unlockSecond()
	}()

	select {
	case <-acquired:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("expected different key to acquire without waiting")
	}
}

func TestSeatFreezeKeyedLockerReleasesAndCleansUpEntries(t *testing.T) {
	locker := newSeatFreezeKeyedLocker()

	unlock := locker.Lock("program:1:category:1")
	if len(locker.entries) != 1 {
		t.Fatalf("expected 1 locker entry after acquire, got %d", len(locker.entries))
	}

	unlock()

	if len(locker.entries) != 0 {
		t.Fatalf("expected locker entry cleanup after release, got %d", len(locker.entries))
	}
}
