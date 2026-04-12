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

type XidConf struct {
	Provider          string `json:",default=static"`
	NodeId            int64  `json:",optional"`
	ServiceBaseNodeId int64  `json:",optional"`
	MaxReplicas       int64  `json:",optional"`
	PodName           string `json:",optional"`
}

type Config struct {
	zrpc.RpcServerConf
	MySQL      xmysql.Config
	StoreRedis xredis.Config `json:"StoreRedis,optional"`
	UserAuth   UserAuthConfig
	Xid        XidConf `json:"Xid,optional"`
}
