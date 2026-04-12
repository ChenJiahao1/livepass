package config

import (
	"time"

	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"

	"github.com/zeromicro/go-zero/zrpc"
)

type AsynqConfig struct {
	Queue          string        `json:",default=order_close"`
	EnqueueTimeout time.Duration `json:",default=500ms"`
	UniqueTTL      time.Duration `json:",default=30m"`
	MaxRetry       int           `json:",default=8"`
	Redis          xredis.Config `json:"Redis,optional"`
}

type Config struct {
	Interval  time.Duration            `json:",default=5s"`
	BatchSize int64                    `json:",default=200"`
	Shards    map[string]xmysql.Config `json:",optional"`
	Asynq     AsynqConfig
	OrderRpc  zrpc.RpcClientConf
}
