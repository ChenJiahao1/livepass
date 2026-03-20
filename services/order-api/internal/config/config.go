// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type AuthConfig struct {
	ChannelCodeHeader string            `json:",default=X-Channel-Code"`
	ChannelMap        map[string]string `json:",optional"`
}

type Config struct {
	rest.RestConf
	OrderRpc zrpc.RpcClientConf
	Auth     AuthConfig
}
