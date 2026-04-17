package config

import (
	"livepass/pkg/xmysql"

	"github.com/zeromicro/go-zero/zrpc"
)

type XidConf struct {
	Provider          string `json:",default=static"`
	NodeId            int64  `json:",optional"`
	ServiceBaseNodeId int64  `json:",optional"`
	MaxReplicas       int64  `json:",optional"`
	PodName           string `json:",optional"`
}

type Config struct {
	zrpc.RpcServerConf
	MySQL xmysql.Config
	Xid   XidConf `json:"Xid,optional"`
}
