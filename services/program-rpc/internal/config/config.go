package config

import (
	"time"

	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type LocalCacheConfig struct {
	DetailTTL           time.Duration `json:",default=20s"`
	DetailNotFoundTTL   time.Duration `json:",default=5s"`
	DetailCacheLimit    int           `json:",default=5000"`
	CategorySnapshotTTL time.Duration `json:",default=5m"`
}

func (c LocalCacheConfig) Normalize() LocalCacheConfig {
	if c.DetailTTL <= 0 {
		c.DetailTTL = 20 * time.Second
	}
	if c.DetailNotFoundTTL <= 0 {
		c.DetailNotFoundTTL = 5 * time.Second
	}
	if c.DetailCacheLimit <= 0 {
		c.DetailCacheLimit = 5000
	}
	if c.CategorySnapshotTTL <= 0 {
		c.CategorySnapshotTTL = 5 * time.Minute
	}

	return c
}

type Config struct {
	zrpc.RpcServerConf
	MySQL      xmysql.Config
	StoreRedis xredis.Config    `json:"StoreRedis,optional"`
	Cache      cache.CacheConf  `json:",optional"`
	LocalCache LocalCacheConfig `json:",optional"`
}
