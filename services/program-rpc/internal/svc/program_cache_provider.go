package svc

import (
	"livepass/pkg/xredis"
	"livepass/services/program-rpc/internal/config"
	"livepass/services/program-rpc/internal/programcache"
)

type ProgramQueryCaches struct {
	CategorySnapshotCache   *programcache.CategorySnapshotCache
	ProgramDetailViewCache  *programcache.ProgramDetailViewCache
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
		// TicketCategory 的展示缓存归属 ProgramDetailViewCache。
		TicketCategoryModel: models.DTicketCategoryModel,
	})
	programDetailViewCache, err := programcache.NewProgramDetailViewCache(
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
		ProgramDetailViewCache:  programDetailViewCache,
		ProgramCacheRegistry:    programcache.NewInvalidationRegistry(programDetailViewCache, categorySnapshotCache),
		ProgramCacheInvalidator: programcache.NewProgramCacheInvalidator(rds, programDetailViewCache),
	}

	if rds != nil && c.CacheInvalidation.Enabled {
		publisher, err := programcache.NewRedisPubSubPublisher(
			rds,
			c.CacheInvalidation.Channel,
			c.CacheInvalidation.PublishTimeout,
		)
		if err != nil {
			panic(err)
		}
		queryCaches.ProgramCacheInvalidator.SetPublisher(publisher)

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
