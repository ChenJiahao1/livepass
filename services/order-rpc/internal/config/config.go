package config

import (
	"time"

	"damai-go/pkg/xmysql"

	"github.com/zeromicro/go-zero/zrpc"
)

type OrderConfig struct {
	CloseAfter time.Duration `json:",default=15m"`
}

type Config struct {
	zrpc.RpcServerConf
	MySQL      xmysql.Config
	ProgramRpc zrpc.RpcClientConf
	PayRpc     zrpc.RpcClientConf
	UserRpc    zrpc.RpcClientConf
	Order      OrderConfig
}
