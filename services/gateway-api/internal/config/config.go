package config

import "github.com/zeromicro/go-zero/gateway"

type AuthConfig struct {
	AccessSecret string `json:",optional"`
}

type InternalAuthConfig struct {
	Secret string `json:",optional"`
}

type PerfModeConfig struct {
	Enabled      bool     `json:",optional"`
	HeaderName   string   `json:",optional"`
	HeaderSecret string   `json:",optional"`
	UserIDHeader string   `json:",optional"`
	AllowedPaths []string `json:",optional"`
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
	Auth         AuthConfig
	InternalAuth InternalAuthConfig
	PerfMode     PerfModeConfig `json:",optional"`
	Cors         CorsConfig `json:",optional"`
}
