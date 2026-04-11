package svc

import (
	"time"

	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/programcache"
	"damai-go/services/program-rpc/internal/seatcache"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SeatFreezeLocker interface {
	Lock(key string) func()
}

type ServiceContext struct {
	Config                  config.Config
	SqlConn                 sqlx.SqlConn
	Redis                   *xredis.Client
	SeatStockStore          *seatcache.SeatStockStore
	DProgramModel           model.DProgramModel
	DProgramCategoryModel   model.DProgramCategoryModel
	DProgramGroupModel      model.DProgramGroupModel
	DProgramShowTimeModel   model.DProgramShowTimeModel
	DSeatModel              model.DSeatModel
	DTicketCategoryModel    model.DTicketCategoryModel
	CategorySnapshotCache   *programcache.CategorySnapshotCache
	ProgramDetailCache      *programcache.ProgramDetailCache
	ProgramCacheRegistry    *programcache.InvalidationRegistry
	ProgramCacheInvalidator *programcache.ProgramCacheInvalidator
	ProgramCacheSubscriber  *programcache.PubSubSubscriber
	SeatFreezeLocker        SeatFreezeLocker
}

const (
	programModelCacheTTL         = 5 * time.Minute
	programModelCacheNotFoundTTL = 30 * time.Second
)

func NewServiceContext(c config.Config) *ServiceContext {
	c.MySQL = c.MySQL.Normalize()
	c.MySQL.DataSource = xmysql.WithLocalTime(c.MySQL.DataSource)
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	rawDB, err := conn.RawDB()
	if err != nil {
		panic(err)
	}
	xmysql.ApplyPool(rawDB, c.MySQL)
	var rds *xredis.Client
	if c.StoreRedis.Host != "" {
		rds = xredis.MustNew(c.StoreRedis)
	}

	dProgramModel := model.NewDProgramModel(conn)
	dProgramGroupModel := model.NewDProgramGroupModel(conn)
	dProgramShowTimeModel := model.NewDProgramShowTimeModel(conn)
	if rds != nil && len(c.Cache) > 0 {
		cacheOpts := []cache.Option{
			cache.WithExpiry(programModelCacheTTL),
			cache.WithNotFoundExpiry(programModelCacheNotFoundTTL),
		}
		dProgramModel = model.NewCachedDProgramModel(conn, c.Cache, cacheOpts...)
		dProgramGroupModel = model.NewCachedDProgramGroupModel(conn, c.Cache, cacheOpts...)
		dProgramShowTimeModel = model.NewCachedDProgramShowTimeModel(conn, c.Cache, cacheOpts...)
	}
	programCategoryModel := model.NewDProgramCategoryModel(conn)
	ticketCategoryModel := model.NewDTicketCategoryModel(conn)

	localCacheConf := c.LocalCache.Normalize()
	c.LocalCache = localCacheConf
	categorySnapshotCache, err := programcache.NewCategorySnapshotCache(programCategoryModel, localCacheConf.CategorySnapshotTTL)
	if err != nil {
		panic(err)
	}
	detailLoader := programcache.NewDetailLoader(
		dProgramModel,
		dProgramShowTimeModel,
		dProgramGroupModel,
		categorySnapshotCache,
		ticketCategoryModel,
	)
	programDetailCache, err := programcache.NewProgramDetailCache(
		detailLoader,
		localCacheConf.DetailTTL,
		localCacheConf.DetailNotFoundTTL,
		localCacheConf.DetailCacheLimit,
	)
	if err != nil {
		panic(err)
	}
	cacheInvalidationConf := c.CacheInvalidation.Normalize()
	c.CacheInvalidation = cacheInvalidationConf
	programCacheRegistry := programcache.NewInvalidationRegistry(programDetailCache, categorySnapshotCache)
	programCacheInvalidator := programcache.NewProgramCacheInvalidator(rds, programDetailCache)
	var programCacheSubscriber *programcache.PubSubSubscriber
	if rds != nil && cacheInvalidationConf.Enabled {
		programCacheSubscriber = programcache.NewPubSubSubscriber(
			rds,
			cacheInvalidationConf.Channel,
			programCacheRegistry,
			cacheInvalidationConf.PublishTimeout,
			cacheInvalidationConf.ReconnectBackoff,
		)
	}

	return &ServiceContext{
		Config:                  c,
		SqlConn:                 conn,
		Redis:                   rds,
		SeatStockStore:          seatcache.NewSeatStockStore(rds, model.NewDSeatModel(conn), seatcache.Config{}),
		DProgramModel:           dProgramModel,
		DProgramCategoryModel:   programCategoryModel,
		DProgramGroupModel:      dProgramGroupModel,
		DProgramShowTimeModel:   dProgramShowTimeModel,
		DSeatModel:              model.NewDSeatModel(conn),
		DTicketCategoryModel:    ticketCategoryModel,
		CategorySnapshotCache:   categorySnapshotCache,
		ProgramDetailCache:      programDetailCache,
		ProgramCacheRegistry:    programCacheRegistry,
		ProgramCacheInvalidator: programCacheInvalidator,
		ProgramCacheSubscriber:  programCacheSubscriber,
	}
}
