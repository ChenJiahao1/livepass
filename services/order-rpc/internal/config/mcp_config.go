package config

import (
	"path/filepath"

	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"

	"github.com/zeromicro/go-zero/core/conf"
	gomcp "github.com/zeromicro/go-zero/mcp"
	"github.com/zeromicro/go-zero/zrpc"
)

type MCPConfig struct {
	gomcp.McpConf
	MySQL      xmysql.Config
	StoreRedis xredis.Config `json:"StoreRedis,optional"`
	ProgramRpc zrpc.RpcClientConf
	PayRpc     zrpc.RpcClientConf
	UserRpc    zrpc.RpcClientConf
	Sharding   ShardingConfig `json:"Sharding,optional"`
}

func LoadMCP(configFile string) (MCPConfig, error) {
	var c MCPConfig
	if err := conf.Load(configFile, &c); err != nil {
		return MCPConfig{}, err
	}
	if err := loadRouteMapFile(filepath.Dir(configFile), &c.Sharding.RouteMap); err != nil {
		return MCPConfig{}, err
	}
	return c, nil
}
