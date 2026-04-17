package programcache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"livepass/pkg/xredis"
	"livepass/services/program-rpc/internal/model"
)

type ProgramCacheInvalidator struct {
	redis         *xredis.Client
	detailCache   *ProgramDetailViewCache
	categoryCache *CategorySnapshotCache
	publisher     PubSubPublisher
	clock         func() time.Time
	instanceID    string
	service       string
}

func NewProgramCacheInvalidator(redis *xredis.Client, detailCache *ProgramDetailViewCache) *ProgramCacheInvalidator {
	invalidator := &ProgramCacheInvalidator{
		redis:       redis,
		detailCache: detailCache,
		clock:       time.Now,
		instanceID:  defaultInstanceID(),
		service:     "program-rpc",
	}
	if detailCache != nil {
		if loader, ok := detailCache.loader.(*DetailLoader); ok {
			invalidator.categoryCache = loader.categorySnapshotCache
		}
	}

	return invalidator
}

func (i *ProgramCacheInvalidator) InvalidateProgram(ctx context.Context, programID int64, programGroupIDs ...int64) error {
	if i == nil {
		return errors.New("program cache invalidator is nil")
	}
	if programID <= 0 {
		return errors.New("program id must be greater than zero")
	}

	if i.redis != nil {
		keys := []string{
			model.ProgramCacheKey(programID),
			model.ProgramFirstShowTimeCacheKey(programID),
		}
		for _, programGroupID := range programGroupIDs {
			if programGroupID <= 0 {
				continue
			}
			keys = append(keys, model.ProgramGroupCacheKey(programGroupID))
		}

		if _, err := i.redis.DelCtx(ctx, uniqueKeys(keys)...); err != nil {
			return err
		}
	}

	if i.detailCache != nil {
		// Clear L1 after L2 so subsequent local misses do not repopulate from stale Redis placeholders.
		i.detailCache.Invalidate(programID)
	}

	return i.publish(ctx, []InvalidationEntry{
		{Cache: cacheProgramDetailView, ProgramID: programID},
	})
}

func (i *ProgramCacheInvalidator) InvalidateCategorySnapshot(ctx context.Context) error {
	if i == nil {
		return errors.New("program cache invalidator is nil")
	}

	if i.categoryCache != nil {
		i.categoryCache.Invalidate()
	}

	return i.publish(ctx, []InvalidationEntry{
		{Cache: cacheCategorySnapshot},
	})
}

func (i *ProgramCacheInvalidator) SetPublisher(publisher PubSubPublisher) {
	if i == nil {
		return
	}
	i.publisher = publisher
}

func uniqueKeys(keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}

	return result
}

func (i *ProgramCacheInvalidator) publish(ctx context.Context, entries []InvalidationEntry) error {
	if i == nil || i.publisher == nil {
		return nil
	}

	msg := InvalidationMessage{
		Version:     "v1",
		Service:     i.service,
		InstanceID:  i.instanceID,
		PublishedAt: i.clock(),
		Entries:     entries,
	}

	return i.publisher.Publish(ctx, msg)
}

func defaultInstanceID() string {
	host, err := os.Hostname()
	if err == nil && host != "" {
		return fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	return fmt.Sprintf("pid-%d", os.Getpid())
}
