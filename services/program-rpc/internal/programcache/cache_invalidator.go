package programcache

import (
	"context"
	"errors"

	"damai-go/pkg/xredis"
	"damai-go/services/program-rpc/internal/model"
)

type ProgramCacheInvalidator struct {
	redis       *xredis.Client
	detailCache *ProgramDetailCache
}

func NewProgramCacheInvalidator(redis *xredis.Client, detailCache *ProgramDetailCache) *ProgramCacheInvalidator {
	return &ProgramCacheInvalidator{
		redis:       redis,
		detailCache: detailCache,
	}
}

func (i *ProgramCacheInvalidator) InvalidateProgram(ctx context.Context, programID int64, programGroupIDs ...int64) error {
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

	return nil
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
