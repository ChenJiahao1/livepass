package config

import "github.com/zeromicro/go-zero/gateway"

type AuthConfig struct {
	ChannelCodeHeader string            `json:",default=X-Channel-Code"`
	ChannelMap        map[string]string `json:",optional"`
}

type Config struct {
	gateway.GatewayConf
	Auth AuthConfig
}
