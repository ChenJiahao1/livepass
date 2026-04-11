package programcache

import (
	"context"

	"github.com/zeromicro/go-zero/core/syncx"
)

type loadGroup struct {
	sf syncx.SingleFlight
}

func newLoadGroup() *loadGroup {
	return &loadGroup{
		sf: syncx.NewSingleFlight(),
	}
}

func (g *loadGroup) Do(key string, fn func() (any, error)) (any, error) {
	return g.sf.Do(key, fn)
}

func (g *loadGroup) DoWithContext(ctx context.Context, key string, fn func(context.Context) (any, error)) (any, error) {
	resultCh := make(chan loadResult, 1)
	go func() {
		val, err := g.sf.Do(key, func() (any, error) {
			return fn(withoutCancel(ctx))
		})
		resultCh <- loadResult{val: val, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		return result.val, result.err
	}
}

type loadResult struct {
	val any
	err error
}

func withoutCancel(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}
