package config

import (
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	MySQL      xmysql.Config
	StoreRedis xredis.Config `json:"StoreRedis,optional"`
}
