package xid

import (
	"sync/atomic"
	"time"
)

var seq atomic.Uint64

func New() int64 {
	now := time.Now().UnixMilli()
	next := seq.Add(1) & 0xffff
	return (now << 16) | int64(next)
}
