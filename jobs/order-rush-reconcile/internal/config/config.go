package config

import (
	"time"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	Interval  time.Duration `json:",default=1s"`
	BatchSize int64         `json:",default=100"`
	OrderRpc  zrpc.RpcClientConf
}
