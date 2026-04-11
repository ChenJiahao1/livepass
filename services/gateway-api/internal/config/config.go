package config

import "github.com/zeromicro/go-zero/gateway"

type AuthConfig struct {
	ChannelCodeHeader string            `json:",default=X-Channel-Code"`
	ChannelMap        map[string]string `json:",optional"`
}

type CorsConfig struct {
	AllowOrigins     []string `json:",optional"`
	AllowHeaders     []string `json:",optional"`
	ExposeHeaders    []string `json:",optional"`
	AllowMethods     []string `json:",optional"`
	AllowCredentials bool     `json:",optional"`
	MaxAge           int      `json:",optional"`
}

type Config struct {
	gateway.GatewayConf
	Auth AuthConfig
	Cors CorsConfig `json:",optional"`
}
