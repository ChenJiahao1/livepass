// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type AuthConfig struct {
	Secret              string `json:",optional"`
	MaxClockSkewSeconds int64  `json:",default=300"`
}

type Config struct {
	rest.RestConf
	UserRpc     zrpc.RpcClientConf
	GatewayAuth AuthConfig
}
