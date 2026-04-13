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

type CacheInvalidationConfig struct {
	Enabled          bool          `json:",default=true"`
	Channel          string        `json:",default=damai-go:program:cache:invalidate"`
	PublishTimeout   time.Duration `json:",default=200ms"`
	ReconnectBackoff time.Duration `json:",default=1s"`
}

func (c CacheInvalidationConfig) Normalize() CacheInvalidationConfig {
	if c.Channel == "" {
		c.Channel = "damai-go:program:cache:invalidate"
	}
	if c.PublishTimeout <= 0 {
		c.PublishTimeout = 200 * time.Millisecond
	}
	if c.ReconnectBackoff <= 0 {
		c.ReconnectBackoff = time.Second
	}

	return c
}

type RushInventoryPreheatConfig struct {
	Enable    bool          `json:",default=true"`
	LeadTime  time.Duration `json:",default=5m"`
	Queue     string        `json:",default=rush_inventory_preheat"`
	MaxRetry  int           `json:",default=8"`
	UniqueTTL time.Duration `json:",default=30m"`
	Redis     xredis.Config `json:"Redis,optional"`
}

type XidConf struct {
	Provider          string `json:",default=static"`
	NodeId            int64  `json:",optional"`
	ServiceBaseNodeId int64  `json:",optional"`
	MaxReplicas       int64  `json:",optional"`
	PodName           string `json:",optional"`
}

type Config struct {
	zrpc.RpcServerConf
	MySQL                xmysql.Config
	StoreRedis           xredis.Config              `json:"StoreRedis,optional"`
	Cache                cache.CacheConf            `json:",optional"`
	LocalCache           LocalCacheConfig           `json:",optional"`
	CacheInvalidation    CacheInvalidationConfig    `json:",optional"`
	RushInventoryPreheat RushInventoryPreheatConfig `json:"RushInventoryPreheat,optional"`
	Xid                  XidConf                    `json:"Xid,optional"`
}
