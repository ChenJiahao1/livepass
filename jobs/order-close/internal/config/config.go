package config

import (
	"time"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	Interval          time.Duration `json:",default=1m"`
	BatchSize         int64         `json:",default=100"`
	ScanSlotStart     int64         `json:",default=0"`
	ScanSlotEnd       int64         `json:",default=1023"`
	ScanSlotBatchSize int64         `json:",default=1"`
	CheckpointSlot    int64         `json:",default=0"`
	OrderRpc          zrpc.RpcClientConf
}
