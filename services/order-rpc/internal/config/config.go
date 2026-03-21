package config

import (
	"time"

	"damai-go/pkg/xmysql"

	"github.com/zeromicro/go-zero/zrpc"
)

type OrderConfig struct {
	CloseAfter time.Duration `json:",default=15m"`
}

type RepeatGuardConfig struct {
	Prefix             string        `json:",default=/damai-go/repeat-guard/order-create/"`
	SessionTTL         int           `json:",default=10"`
	LockAcquireTimeout time.Duration `json:",default=200ms"`
}

type Config struct {
	zrpc.RpcServerConf
	MySQL       xmysql.Config
	ProgramRpc  zrpc.RpcClientConf
	PayRpc      zrpc.RpcClientConf
	UserRpc     zrpc.RpcClientConf
	Order       OrderConfig
	RepeatGuard RepeatGuardConfig
}
