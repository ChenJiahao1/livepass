package programcache

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"damai-go/services/program-rpc/pb"
)

func TestProgramDetailCacheCachesLoadedDetail(t *testing.T) {
	loader := &stubDetailLoader{
		responses: map[int64]*pb.ProgramDetailInfo{
			10001: {Id: 10001, Title: "Phase1 示例演出"},
		},
	}

	cache, err := NewProgramDetailCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailCache returned error: %v", err)
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

	cache, err := NewProgramDetailCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailCache returned error: %v", err)
	}

	if _, err := cache.Get(context.Background(), 10002); !errors.Is(err, ErrProgramNotFound) {
		t.Fatalf("expected first Get to return ErrProgramNotFound, got %v", err)
	}

	delete(loader.errors, 10002)
	loader.responses = map[int64]*pb.ProgramDetailInfo{
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
		responses: map[int64]*pb.ProgramDetailInfo{
			10003: {Id: 10003, Title: "v1"},
		},
	}

	cache, err := NewProgramDetailCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailCache returned error: %v", err)
	}

	first, err := cache.Get(context.Background(), 10003)
	if err != nil {
		t.Fatalf("first Get returned error: %v", err)
	}
	if first.Title != "v1" {
		t.Fatalf("expected initial detail title v1, got %+v", first)
	}

	loader.responses[10003] = &pb.ProgramDetailInfo{Id: 10003, Title: "v2"}
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

	cache, err := NewProgramDetailCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailCache returned error: %v", err)
	}

	if _, err := cache.Get(context.Background(), 10004); !errors.Is(err, ErrProgramNotFound) {
		t.Fatalf("expected first Get to return ErrProgramNotFound, got %v", err)
	}

	delete(loader.errors, 10004)
	loader.responses = map[int64]*pb.ProgramDetailInfo{
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
		responses: map[int64]*pb.ProgramDetailInfo{
			10005: {Id: 10005, Title: "reloaded from source"},
		},
	}

	cache, err := NewProgramDetailCache(loader, 20*time.Second, 5*time.Second, 16)
	if err != nil {
		t.Fatalf("NewProgramDetailCache returned error: %v", err)
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

type stubDetailLoader struct {
	responses map[int64]*pb.ProgramDetailInfo
	errors    map[int64]error
	calls     map[int64]int
}

func (l *stubDetailLoader) Load(_ context.Context, programID int64) (*pb.ProgramDetailInfo, error) {
	if l.calls == nil {
		l.calls = make(map[int64]int)
	}
	l.calls[programID]++

	if err := l.errors[programID]; err != nil {
		return nil, err
	}

	if resp := l.responses[programID]; resp != nil {
		return &pb.ProgramDetailInfo{
			Id:    resp.Id,
			Title: resp.Title,
		}, nil
	}

	return nil, fmt.Errorf("unexpected program id %d", programID)
}
