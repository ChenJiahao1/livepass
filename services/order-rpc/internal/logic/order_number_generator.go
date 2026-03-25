package logic

import (
	"sync"
	"time"

	"damai-go/pkg/xid"
	"damai-go/services/order-rpc/sharding"
)

var defaultOrderNumberGenerator = newOrderNumberGenerator(func() int64 {
	return (xid.New() >> 12) & 0x3FF
})

const maxOrderNumberSequence = (1 << 12) - 1

type orderNumberGenerator struct {
	mu             sync.Mutex
	workerIDSource func() int64
	now            func() time.Time
	sleep          func(time.Duration)
	lastUnixSecond int64
	sequence       int64
}

func newOrderNumberGenerator(workerIDSource func() int64) *orderNumberGenerator {
	if workerIDSource == nil {
		panic("workerIDSource is required")
	}

	return &orderNumberGenerator{
		workerIDSource: workerIDSource,
		now:            time.Now,
		sleep:          time.Sleep,
		sequence:       -1,
	}
}

func (g *orderNumberGenerator) Next(userID int64, now time.Time) int64 {
	workerID := g.workerIDSource()
	unixSecond := now.UTC().Unix()

	g.mu.Lock()
	if unixSecond < g.lastUnixSecond {
		unixSecond = g.lastUnixSecond
	}

	switch {
	case g.sequence == -1 || unixSecond > g.lastUnixSecond:
		g.lastUnixSecond = unixSecond
		g.sequence = 0
	case g.sequence < maxOrderNumberSequence:
		g.sequence++
	default:
		g.lastUnixSecond = g.waitNextSecondLocked(g.lastUnixSecond)
		g.sequence = 0
	}

	generatedAt := time.Unix(g.lastUnixSecond, 0).UTC()
	sequence := g.sequence
	g.mu.Unlock()

	return sharding.BuildOrderNumber(userID, generatedAt, workerID, sequence)
}

func (g *orderNumberGenerator) waitNextSecondLocked(lastUnixSecond int64) int64 {
	for {
		current := g.now().UTC()
		currentUnixSecond := current.Unix()
		if currentUnixSecond > lastUnixSecond {
			return currentUnixSecond
		}

		wait := time.Unix(lastUnixSecond+1, 0).Sub(current)
		if wait <= 0 {
			wait = time.Second - time.Duration(current.Nanosecond())
			if wait <= 0 {
				wait = time.Millisecond
			}
		}

		g.mu.Unlock()
		g.sleep(wait)
		g.mu.Lock()
	}
}
