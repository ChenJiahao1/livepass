package config

import (
	"time"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	Interval          time.Duration `json:",default=5s"`
	BatchSize         int64         `json:",default=200"`
	ScanSlotStart     int64         `json:",default=0"`
	ScanSlotEnd       int64         `json:",default=1023"`
	ScanSlotBatchSize int64         `json:",default=64"`
	CheckpointSlot    int64         `json:",default=0"`
	OrderRpc          zrpc.RpcClientConf
}
