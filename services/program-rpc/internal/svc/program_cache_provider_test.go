package svc

import (
	"reflect"
	"testing"
	"time"

	"damai-go/pkg/xredis"
	"damai-go/services/program-rpc/internal/config"
)

func TestNewProgramQueryCachesBuildsDetailCacheAndRegistry(t *testing.T) {
	t.Run("builds local caches and registry", func(t *testing.T) {
		caches := newProgramQueryCachesForTest(t, true, true)

		if caches.CategorySnapshotCache == nil {
			t.Fatal("expected category snapshot cache")
		}
		if caches.ProgramDetailCache == nil {
			t.Fatal("expected detail cache")
		}
		if caches.ProgramCacheRegistry == nil {
			t.Fatal("expected invalidation registry")
		}
		if caches.ProgramCacheInvalidator == nil {
			t.Fatal("expected invalidator")
		}
		requireDurationField(t, caches.ProgramDetailCache, "detailTTL", 42*time.Second)
		requireDurationField(t, caches.ProgramDetailCache, "notFoundTTL", 9*time.Second)
	})

	t.Run("builds subscriber only when redis and invalidation enabled", func(t *testing.T) {
		withSubscriber := newProgramQueryCachesForTest(t, true, true)
		if withSubscriber.ProgramCacheSubscriber == nil {
			t.Fatal("expected subscriber when redis and invalidation are enabled")
		}

		disabled := newProgramQueryCachesForTest(t, true, false)
		if disabled.ProgramCacheSubscriber != nil {
			t.Fatal("expected no subscriber when invalidation is disabled")
		}

		withoutRedis := newProgramQueryCachesForTest(t, false, true)
		if withoutRedis.ProgramCacheSubscriber != nil {
			t.Fatal("expected no subscriber when redis is nil")
		}
	})
}

func newProgramQueryCachesForTest(t *testing.T, withRedis bool, invalidationEnabled bool) ProgramQueryCaches {
	t.Helper()

	models := newProgramModelsForTest(t, withRedis)

	var rds *xredis.Client
	if withRedis {
		rds = xredis.MustNew(xredis.Config{
			Host: "127.0.0.1:6379",
			Type: "node",
		})
	}

	cfg := config.Config{
		LocalCache: config.LocalCacheConfig{
			DetailTTL:           42 * time.Second,
			DetailNotFoundTTL:   9 * time.Second,
			DetailCacheLimit:    1024,
			CategorySnapshotTTL: 3 * time.Minute,
		},
		CacheInvalidation: config.CacheInvalidationConfig{
			Enabled:          invalidationEnabled,
			Channel:          "test:program:invalidate",
			PublishTimeout:   300 * time.Millisecond,
			ReconnectBackoff: 2 * time.Second,
		},
	}

	return newProgramQueryCaches(models, rds, cfg)
}

func requireDurationField(t *testing.T, target any, name string, expected time.Duration) {
	t.Helper()

	field := reflect.ValueOf(target).Elem().FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("expected field %s", name)
	}
	if got := time.Duration(field.Int()); got != expected {
		t.Fatalf("expected %s %s, got %s", name, expected, got)
	}
}
