package config

import (
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/stores/cache"
	gomcp "github.com/zeromicro/go-zero/mcp"
)

type MCPConfig struct {
	gomcp.McpConf
	MySQL                xmysql.Config
	StoreRedis           xredis.Config              `json:"StoreRedis,optional"`
	Cache                cache.CacheConf            `json:",optional"`
	LocalCache           LocalCacheConfig           `json:",optional"`
	CacheInvalidation    CacheInvalidationConfig    `json:",optional"`
	RushInventoryPreheat RushInventoryPreheatConfig `json:"RushInventoryPreheat,optional"`
	Xid                  XidConf                    `json:"Xid,optional"`
}

func LoadMCP(file string) (MCPConfig, error) {
	var c MCPConfig
	conf.MustLoad(file, &c)
	return c, nil
}
