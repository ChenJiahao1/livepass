package svc

import (
	"context"
	"reflect"
	"testing"
	"time"

	"livepass/pkg/xredis"
	"livepass/services/program-rpc/internal/config"

	"github.com/redis/go-redis/v9"
)

func TestNewProgramQueryCachesBuildsDetailCacheAndRegistry(t *testing.T) {
	t.Run("builds local caches and registry", func(t *testing.T) {
		caches := newProgramQueryCachesForTest(t, true, true)

		if caches.CategorySnapshotCache == nil {
			t.Fatal("expected category snapshot cache")
		}
		if caches.ProgramDetailViewCache == nil {
			t.Fatal("expected detail cache")
		}
		if caches.ProgramCacheRegistry == nil {
			t.Fatal("expected invalidation registry")
		}
		if caches.ProgramCacheInvalidator == nil {
			t.Fatal("expected invalidator")
		}
		requireDurationField(t, caches.ProgramDetailViewCache, "detailTTL", 42*time.Second)
		requireDurationField(t, caches.ProgramDetailViewCache, "notFoundTTL", 9*time.Second)
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

	t.Run("injects publisher with configured channel", func(t *testing.T) {
		caches := newProgramQueryCachesForTest(t, true, true)
		if caches.ProgramCacheInvalidator == nil {
			t.Fatal("expected invalidator")
		}

		client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
		defer func() {
			_ = client.Close()
		}()

		ctx := context.Background()
		pubsub := client.Subscribe(ctx, "test:program:invalidate")
		defer func() {
			_ = pubsub.Close()
		}()
		if _, err := pubsub.ReceiveTimeout(ctx, time.Second); err != nil {
			t.Fatalf("subscribe custom invalidation channel error: %v", err)
		}

		if err := caches.ProgramCacheInvalidator.InvalidateCategorySnapshot(ctx); err != nil {
			t.Fatalf("InvalidateCategorySnapshot returned error: %v", err)
		}

		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			t.Fatalf("expected publish on configured channel, got %v", err)
		}
		if msg.Channel != "test:program:invalidate" {
			t.Fatalf("expected publish on configured channel, got %q", msg.Channel)
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
