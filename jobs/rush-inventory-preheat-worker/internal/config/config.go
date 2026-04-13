package config

import (
	"time"

	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"

	"github.com/zeromicro/go-zero/zrpc"
)

type AsynqConfig struct {
	Queue           string        `json:",default=rush_inventory_preheat"`
	Concurrency     int           `json:",default=16"`
	ShutdownTimeout time.Duration `json:",default=10s"`
	Redis           xredis.Config `json:"Redis,optional"`
}

type Config struct {
	Asynq      AsynqConfig
	MySQL      xmysql.Config
	OrderRpc   zrpc.RpcClientConf
	ProgramRpc zrpc.RpcClientConf
}
