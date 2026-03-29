package config

import (
	"time"

	"damai-go/pkg/xredis"

	"github.com/zeromicro/go-zero/zrpc"
)

type AsynqConfig struct {
	Queue           string        `json:",default=order_close"`
	Concurrency     int           `json:",default=4"`
	ShutdownTimeout time.Duration `json:",default=10s"`
	Redis           xredis.Config `json:"Redis,optional"`
}

type Config struct {
	Asynq    AsynqConfig
	OrderRpc zrpc.RpcClientConf
}
