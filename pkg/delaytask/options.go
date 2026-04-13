package delaytask

import "time"

type Options struct {
	Queue          string
	MaxRetry       int
	UniqueTTL      time.Duration
	EnqueueTimeout time.Duration
}
