package svc

import (
	"livepass/pkg/xmysql"
	"livepass/pkg/xredis"
	"livepass/services/program-rpc/internal/config"
	"livepass/services/program-rpc/internal/model"
	"livepass/services/program-rpc/internal/programcache"
	"livepass/services/program-rpc/internal/seatcache"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SeatFreezeLocker interface {
	Lock(key string) func()
}

type ServiceContext struct {
	Config                     config.Config
	SqlConn                    sqlx.SqlConn
	Redis                      *xredis.Client
	SeatStockStore             *seatcache.SeatStockStore
	DProgramModel              model.DProgramModel
	DProgramCategoryModel      model.DProgramCategoryModel
	DProgramGroupModel         model.DProgramGroupModel
	DProgramShowTimeModel      model.DProgramShowTimeModel
	DSeatModel                 model.DSeatModel
	DTicketCategoryModel       model.DTicketCategoryModel
	CategorySnapshotCache      *programcache.CategorySnapshotCache
	ProgramDetailViewCache     *programcache.ProgramDetailViewCache
	ProgramCacheRegistry       *programcache.InvalidationRegistry
	ProgramCacheInvalidator    *programcache.ProgramCacheInvalidator
	ProgramCacheSubscriber     *programcache.PubSubSubscriber
	RushInventoryPreheatClient RushInventoryPreheatClient
	SeatFreezeLocker           SeatFreezeLocker
}

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

	models := newProgramModels(conn, rds, c)

	localCacheConf := c.LocalCache.Normalize()
	c.LocalCache = localCacheConf
	cacheInvalidationConf := c.CacheInvalidation.Normalize()
	c.CacheInvalidation = cacheInvalidationConf
	queryCaches := newProgramQueryCaches(models, rds, c)
	rushInventoryPreheatClient, err := newRushInventoryPreheatClient(conn, c.RushInventoryPreheat)
	if err != nil {
		panic(err)
	}

	return &ServiceContext{
		Config:                     c,
		SqlConn:                    conn,
		Redis:                      rds,
		SeatStockStore:             seatcache.NewSeatStockStore(rds, models.DSeatModel, seatcache.Config{}),
		DProgramModel:              models.DProgramModel,
		DProgramCategoryModel:      models.DProgramCategoryModel,
		DProgramGroupModel:         models.DProgramGroupModel,
		DProgramShowTimeModel:      models.DProgramShowTimeModel,
		DSeatModel:                 models.DSeatModel,
		DTicketCategoryModel:       models.DTicketCategoryModel,
		CategorySnapshotCache:      queryCaches.CategorySnapshotCache,
		ProgramDetailViewCache:     queryCaches.ProgramDetailViewCache,
		ProgramCacheRegistry:       queryCaches.ProgramCacheRegistry,
		ProgramCacheInvalidator:    queryCaches.ProgramCacheInvalidator,
		ProgramCacheSubscriber:     queryCaches.ProgramCacheSubscriber,
		RushInventoryPreheatClient: rushInventoryPreheatClient,
	}
}

func NewMCPServiceContext(c config.MCPConfig) *ServiceContext {
	return NewServiceContext(config.Config{
		MySQL:                c.MySQL,
		StoreRedis:           c.StoreRedis,
		Cache:                c.Cache,
		LocalCache:           c.LocalCache,
		CacheInvalidation:    c.CacheInvalidation,
		RushInventoryPreheat: c.RushInventoryPreheat,
		Xid:                  c.Xid,
	})
}
