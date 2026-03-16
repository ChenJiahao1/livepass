package config

import (
	"damai-go/pkg/xmysql"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	MySQL xmysql.Config
}
