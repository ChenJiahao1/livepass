package programcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"damai-go/services/program-rpc/pb"
)

func TestProgramDetailCacheCachesLoadedDetail(t *testing.T) {
	loader := &stubDetailLoader{
		responses: map[int64]*pb.ProgramDetailViewInfo{
			10001: {Id: 10001, Title: "Phase1 示例演出"},
		},
	}

	cache, err := NewProgramDetailViewCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailViewCache returned error: %v", err)
	}

	first, err := cache.Get(context.Background(), 10001)
	if err != nil {
		t.Fatalf("first Get returned error: %v", err)
	}
	first.Title = "mutated in caller"

	second, err := cache.Get(context.Background(), 10001)
	if err != nil {
		t.Fatalf("second Get returned error: %v", err)
	}
	if second.Title != "Phase1 示例演出" {
		t.Fatalf("expected cached payload to be reloaded from bytes, got %+v", second)
	}
	if loader.calls[10001] != 1 {
		t.Fatalf("expected loader to be called once, got %d", loader.calls[10001])
	}
}

func TestProgramDetailCacheCachesNotFoundResult(t *testing.T) {
	loader := &stubDetailLoader{
		errors: map[int64]error{
			10002: ErrProgramNotFound,
		},
	}

	cache, err := NewProgramDetailViewCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailViewCache returned error: %v", err)
	}

	if _, err := cache.Get(context.Background(), 10002); !errors.Is(err, ErrProgramNotFound) {
		t.Fatalf("expected first Get to return ErrProgramNotFound, got %v", err)
	}

	delete(loader.errors, 10002)
	loader.responses = map[int64]*pb.ProgramDetailViewInfo{
		10002: {Id: 10002, Title: "newly backfilled"},
	}

	if _, err := cache.Get(context.Background(), 10002); !errors.Is(err, ErrProgramNotFound) {
		t.Fatalf("expected second Get to hit not-found cache, got %v", err)
	}
	if loader.calls[10002] != 1 {
		t.Fatalf("expected loader to be called once for not-found cache, got %d", loader.calls[10002])
	}
}

func TestProgramDetailCacheInvalidateClearsCachedDetail(t *testing.T) {
	loader := &stubDetailLoader{
		responses: map[int64]*pb.ProgramDetailViewInfo{
			10003: {Id: 10003, Title: "v1"},
		},
	}

	cache, err := NewProgramDetailViewCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailViewCache returned error: %v", err)
	}

	first, err := cache.Get(context.Background(), 10003)
	if err != nil {
		t.Fatalf("first Get returned error: %v", err)
	}
	if first.Title != "v1" {
		t.Fatalf("expected initial detail title v1, got %+v", first)
	}

	loader.responses[10003] = &pb.ProgramDetailViewInfo{Id: 10003, Title: "v2"}
	cache.Invalidate(10003)

	second, err := cache.Get(context.Background(), 10003)
	if err != nil {
		t.Fatalf("second Get returned error: %v", err)
	}
	if second.Title != "v2" {
		t.Fatalf("expected invalidated cache to reload v2, got %+v", second)
	}
	if loader.calls[10003] != 2 {
		t.Fatalf("expected loader to be called twice after invalidation, got %d", loader.calls[10003])
	}
}

func TestProgramDetailCacheInvalidateClearsNotFoundEntry(t *testing.T) {
	loader := &stubDetailLoader{
		errors: map[int64]error{
			10004: ErrProgramNotFound,
		},
	}

	cache, err := NewProgramDetailViewCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailViewCache returned error: %v", err)
	}

	if _, err := cache.Get(context.Background(), 10004); !errors.Is(err, ErrProgramNotFound) {
		t.Fatalf("expected first Get to return ErrProgramNotFound, got %v", err)
	}

	delete(loader.errors, 10004)
	loader.responses = map[int64]*pb.ProgramDetailViewInfo{
		10004: {Id: 10004, Title: "backfilled after invalidate"},
	}
	cache.Invalidate(10004)

	resp, err := cache.Get(context.Background(), 10004)
	if err != nil {
		t.Fatalf("expected Get after invalidate to reload detail, got %v", err)
	}
	if resp.Title != "backfilled after invalidate" {
		t.Fatalf("expected invalidated not-found cache to reload fresh detail, got %+v", resp)
	}
}

func TestProgramDetailCacheDropsCorruptedPayloadAndReloads(t *testing.T) {
	loader := &stubDetailLoader{
		responses: map[int64]*pb.ProgramDetailViewInfo{
			10005: {Id: 10005, Title: "reloaded from source"},
		},
	}

	cache, err := NewProgramDetailViewCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailViewCache returned error: %v", err)
	}

	cache.cache.SetWithExpire(detailCacheKey(10005), []byte("broken"), time.Minute)

	resp, err := cache.Get(context.Background(), 10005)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if resp.Title != "reloaded from source" {
		t.Fatalf("expected corrupted payload to be reloaded, got %+v", resp)
	}
	if loader.calls[10005] != 1 {
		t.Fatalf("expected loader to be called once after dropping corrupted payload, got %d", loader.calls[10005])
	}
}

func TestProgramDetailCacheDeduplicatesConcurrentLoads(t *testing.T) {
	loader := &blockingDetailLoader{
		resp: &pb.ProgramDetailViewInfo{Id: 20001, Title: "concurrent load"},
	}

	cache, err := NewProgramDetailViewCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailViewCache returned error: %v", err)
	}

	start := make(chan struct{})
	resultCh := make(chan error, 20)
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			resp, err := cache.Get(context.Background(), 20001)
			if err != nil {
				resultCh <- err
				return
			}
			if resp == nil || resp.Id != 20001 {
				resultCh <- fmt.Errorf("unexpected response: %+v", resp)
				return
			}
			resultCh <- nil
		}()
	}

	close(start)
	wg.Wait()
	close(resultCh)

	for err := range resultCh {
		if err != nil {
			t.Fatalf("concurrent Get returned error: %v", err)
		}
	}

	if calls := atomic.LoadInt32(&loader.calls); calls != 1 {
		t.Fatalf("expected loader to be called once, got %d", calls)
	}
}

func TestProgramDetailCacheSharedLoadIgnoresCallerCancel(t *testing.T) {
	loader := &cancelAwareDetailLoader{
		resp:    &pb.ProgramDetailViewInfo{Id: 20002, Title: "shared load"},
		started: make(chan struct{}),
	}

	cache, err := NewProgramDetailViewCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailViewCache returned error: %v", err)
	}

	cancelCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 2)
	respCh := make(chan *pb.ProgramDetailViewInfo, 1)

	go func() {
		_, err := cache.Get(cancelCtx, 20002)
		errCh <- err
	}()

	<-loader.started

	go func() {
		resp, err := cache.Get(context.Background(), 20002)
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
		errCh <- nil
	}()

	var firstErr error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if !errors.Is(firstErr, context.DeadlineExceeded) {
		t.Fatalf("expected caller cancel to return context deadline exceeded, got %v", firstErr)
	}

	select {
	case resp := <-respCh:
		if resp.Id != 20002 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	default:
		t.Fatalf("expected successful response for non-canceled caller")
	}

	if calls := atomic.LoadInt32(&loader.calls); calls != 1 {
		t.Fatalf("expected loader to be called once, got %d", calls)
	}
}

type stubDetailLoader struct {
	responses map[int64]*pb.ProgramDetailViewInfo
	errors    map[int64]error
	calls     map[int64]int
}

type blockingDetailLoader struct {
	resp  *pb.ProgramDetailViewInfo
	calls int32
}

func (l *blockingDetailLoader) Load(_ context.Context, _ int64) (*pb.ProgramDetailViewInfo, error) {
	atomic.AddInt32(&l.calls, 1)
	time.Sleep(50 * time.Millisecond)
	return &pb.ProgramDetailViewInfo{Id: l.resp.Id, Title: l.resp.Title}, nil
}

type cancelAwareDetailLoader struct {
	resp    *pb.ProgramDetailViewInfo
	calls   int32
	started chan struct{}
}

func (l *cancelAwareDetailLoader) Load(ctx context.Context, _ int64) (*pb.ProgramDetailViewInfo, error) {
	if atomic.AddInt32(&l.calls, 1) == 1 && l.started != nil {
		close(l.started)
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(50 * time.Millisecond):
		return &pb.ProgramDetailViewInfo{Id: l.resp.Id, Title: l.resp.Title}, nil
	}
}

func (l *stubDetailLoader) Load(_ context.Context, programID int64) (*pb.ProgramDetailViewInfo, error) {
	if l.calls == nil {
		l.calls = make(map[int64]int)
	}
	l.calls[programID]++

	if err := l.errors[programID]; err != nil {
		return nil, err
	}

	if resp := l.responses[programID]; resp != nil {
		return &pb.ProgramDetailViewInfo{
			Id:    resp.Id,
			Title: resp.Title,
		}, nil
	}

	return nil, fmt.Errorf("unexpected program id %d", programID)
}
