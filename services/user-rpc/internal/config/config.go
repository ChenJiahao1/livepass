package config

import (
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"time"

	"github.com/zeromicro/go-zero/zrpc"
)

type UserAuthConfig struct {
	TokenExpire    time.Duration     `json:",default=2h"`
	LoginFailLimit int64             `json:",default=5"`
	ChannelMap     map[string]string `json:",optional"`
}

type Config struct {
	zrpc.RpcServerConf
	MySQL    xmysql.Config
	Redis    xredis.Config
	UserAuth UserAuthConfig
}
