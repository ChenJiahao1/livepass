package svc

import (
	"damai-go/pkg/xredis"
	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/internal/programcache"
)

type ProgramQueryCaches struct {
	CategorySnapshotCache   *programcache.CategorySnapshotCache
	ProgramDetailCache      *programcache.ProgramDetailCache
	ProgramCacheRegistry    *programcache.InvalidationRegistry
	ProgramCacheInvalidator *programcache.ProgramCacheInvalidator
	ProgramCacheSubscriber  *programcache.PubSubSubscriber
}

func newProgramQueryCaches(models ProgramModels, rds *xredis.Client, c config.Config) ProgramQueryCaches {
	categorySnapshotCache, err := programcache.NewCategorySnapshotCache(
		models.DProgramCategoryModel,
		c.LocalCache.CategorySnapshotTTL,
	)
	if err != nil {
		panic(err)
	}

	detailLoader := programcache.NewDetailLoader(programcache.DetailLoaderDeps{
		ProgramModel:          models.DProgramModel,
		ProgramShowTimeModel:  models.DProgramShowTimeModel,
		ProgramGroupModel:     models.DProgramGroupModel,
		CategorySnapshotCache: categorySnapshotCache,
		// TicketCategory 的展示缓存归属 ProgramDetailCache。
		TicketCategoryModel: models.DTicketCategoryModel,
	})
	programDetailCache, err := programcache.NewProgramDetailCache(
		detailLoader,
		c.LocalCache.DetailTTL,
		c.LocalCache.DetailNotFoundTTL,
		c.LocalCache.DetailCacheLimit,
	)
	if err != nil {
		panic(err)
	}

	queryCaches := ProgramQueryCaches{
		CategorySnapshotCache:   categorySnapshotCache,
		ProgramDetailCache:      programDetailCache,
		ProgramCacheRegistry:    programcache.NewInvalidationRegistry(programDetailCache, categorySnapshotCache),
		ProgramCacheInvalidator: programcache.NewProgramCacheInvalidator(rds, programDetailCache),
	}

	if rds != nil && c.CacheInvalidation.Enabled {
		queryCaches.ProgramCacheSubscriber = programcache.NewPubSubSubscriber(
			rds,
			c.CacheInvalidation.Channel,
			queryCaches.ProgramCacheRegistry,
			c.CacheInvalidation.PublishTimeout,
			c.CacheInvalidation.ReconnectBackoff,
		)
	}

	return queryCaches
}
