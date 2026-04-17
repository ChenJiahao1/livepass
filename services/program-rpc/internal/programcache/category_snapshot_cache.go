package programcache

import (
	"context"
	"errors"
	"time"

	"livepass/services/program-rpc/internal/model"

	"github.com/zeromicro/go-zero/core/collection"
)

const categorySnapshotCacheKey = "program:category:snapshot"

type CategorySnapshotCache struct {
	cache     *collection.Cache
	model     model.DProgramCategoryModel
	ttl       time.Duration
	loadGroup *loadGroup
}

func NewCategorySnapshotCache(categoryModel model.DProgramCategoryModel, ttl time.Duration) (*CategorySnapshotCache, error) {
	if categoryModel == nil {
		return nil, errors.New("program category model is required")
	}

	localCache, err := collection.NewCache(ttl,
		collection.WithLimit(1),
		collection.WithName("program-category-snapshot-cache"),
	)
	if err != nil {
		return nil, err
	}

	return &CategorySnapshotCache{
		cache:     localCache,
		model:     categoryModel,
		ttl:       ttl,
		loadGroup: newLoadGroup(),
	}, nil
}

func (c *CategorySnapshotCache) GetAll(ctx context.Context) ([]*model.DProgramCategory, error) {
	if payload, ok := c.cache.Get(categorySnapshotCacheKey); ok {
		categories, ok := payload.([]*model.DProgramCategory)
		if ok {
			return cloneProgramCategories(categories), nil
		}
		c.cache.Del(categorySnapshotCacheKey)
	}

	loaded, err := c.loadGroup.DoWithContext(ctx, categorySnapshotCacheKey, func(sharedCtx context.Context) (any, error) {
		if payload, ok := c.cache.Get(categorySnapshotCacheKey); ok {
			categories, ok := payload.([]*model.DProgramCategory)
			if ok {
				return cloneProgramCategories(categories), nil
			}
			c.cache.Del(categorySnapshotCacheKey)
		}

		categories, err := c.model.FindAll(sharedCtx)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				categories = []*model.DProgramCategory{}
			} else {
				return nil, err
			}
		}

		cloned := cloneProgramCategories(categories)
		c.cache.SetWithExpire(categorySnapshotCacheKey, cloned, c.ttl)

		return cloneProgramCategories(cloned), nil
	})
	if err != nil {
		return nil, err
	}

	categories, ok := loaded.([]*model.DProgramCategory)
	if !ok {
		return nil, errors.New("program category cache returned invalid payload")
	}
	return cloneProgramCategories(categories), nil
}

func (c *CategorySnapshotCache) Invalidate() {
	c.cache.Del(categorySnapshotCacheKey)
}

func cloneProgramCategories(categories []*model.DProgramCategory) []*model.DProgramCategory {
	if len(categories) == 0 {
		return []*model.DProgramCategory{}
	}

	cloned := make([]*model.DProgramCategory, 0, len(categories))
	for _, category := range categories {
		if category == nil {
			continue
		}
		categoryCopy := *category
		cloned = append(cloned, &categoryCopy)
	}

	return cloned
}
