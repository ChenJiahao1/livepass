package programcache

import (
	"context"
	"errors"
	"testing"
	"time"

	"damai-go/pkg/xredis"
	"damai-go/services/program-rpc/internal/model"
)

func TestProgramCacheInvalidatorClearsRedisBeforePublishing(t *testing.T) {
	ctx := context.Background()
	redis := xredis.MustNew(xredis.Config{
		Host: "127.0.0.1:6379",
		Type: "node",
	})

	programID := int64(10101)
	groupID := int64(20101)
	keys := []string{
		model.ProgramCacheKey(programID),
		model.ProgramFirstShowTimeCacheKey(programID),
		model.ProgramGroupCacheKey(groupID),
	}
	for _, key := range keys {
		if err := redis.SetCtx(ctx, key, "1"); err != nil {
			t.Fatalf("seed redis key %s error: %v", key, err)
		}
	}
	defer func() {
		_, _ = redis.DelCtx(ctx, keys...)
	}()

	publisher := &assertRedisClearedPublisher{
		t:    t,
		rds:  redis,
		keys: keys,
	}

	invalidator := NewProgramCacheInvalidator(redis, nil)
	invalidator.publisher = publisher

	if err := invalidator.InvalidateProgram(ctx, programID, groupID); err != nil {
		t.Fatalf("InvalidateProgram returned error: %v", err)
	}
	if !publisher.published {
		t.Fatalf("expected publisher to be called")
	}
}

func TestProgramCacheInvalidatorInvalidatesCategorySnapshotLocally(t *testing.T) {
	fakeConn := &stubCategorySqlConn{
		rows: []*model.DProgramCategory{
			{Id: 11, Name: "演出", Type: 1, Status: 1},
		},
	}
	categoryModel := model.NewDProgramCategoryModel(fakeConn)
	cache, err := NewCategorySnapshotCache(categoryModel, 20*time.Second)
	if err != nil {
		t.Fatalf("NewCategorySnapshotCache returned error: %v", err)
	}

	categories, err := cache.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll returned error: %v", err)
	}
	if len(categories) != 1 || categories[0].Id != 11 {
		t.Fatalf("unexpected initial categories: %+v", categories)
	}
	if _, ok := cache.cache.Get(categorySnapshotCacheKey); !ok {
		t.Fatalf("expected category snapshot to be cached")
	}

	fakeConn.rows = []*model.DProgramCategory{
		{Id: 21, Name: "音乐会", Type: 1, Status: 1},
	}

	invalidator := NewProgramCacheInvalidator(nil, nil)
	invalidator.categoryCache = cache
	invalidator.publisher = noopPublisher{}

	if err := invalidator.InvalidateCategorySnapshot(context.Background()); err != nil {
		t.Fatalf("InvalidateCategorySnapshot returned error: %v", err)
	}
	if _, ok := cache.cache.Get(categorySnapshotCacheKey); ok {
		t.Fatalf("expected category snapshot cache to be cleared locally")
	}

	reloaded, err := cache.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll after invalidate returned error: %v", err)
	}
	if len(reloaded) != 1 || reloaded[0].Id != 21 {
		t.Fatalf("expected categories to be reloaded after invalidation, got %+v", reloaded)
	}
}

type assertRedisClearedPublisher struct {
	t         *testing.T
	rds       *xredis.Client
	keys      []string
	published bool
}

func (p *assertRedisClearedPublisher) Publish(ctx context.Context, _ InvalidationMessage) error {
	p.t.Helper()
	for _, key := range p.keys {
		exists, err := p.rds.ExistsCtx(ctx, key)
		if err != nil {
			p.t.Fatalf("ExistsCtx for key %s returned error: %v", key, err)
		}
		if exists {
			p.t.Fatalf("expected key %s to be cleared before publish", key)
		}
	}
	p.published = true
	return nil
}

type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, InvalidationMessage) error {
	return nil
}

type failingPublisher struct{}

func (failingPublisher) Publish(context.Context, InvalidationMessage) error {
	return errors.New("publish failed")
}

func TestProgramCacheInvalidatorNilInvalidateProgramReturnsError(t *testing.T) {
	var invalidator *ProgramCacheInvalidator

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("expected no panic, got %v", recovered)
		}
	}()

	if err := invalidator.InvalidateProgram(context.Background(), 10001); err == nil {
		t.Fatalf("expected error for nil invalidator")
	}
}
