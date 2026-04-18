package main

import (
	"flag"

	"livepass/services/gateway-api/internal/config"
	"livepass/services/gateway-api/internal/middleware"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/gateway"
)

var configFile = flag.String("f", "etc/gateway-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := gateway.MustNewServer(c.GatewayConf)
	server.Use(middleware.NewCorsMiddleware(c.Cors).Handle)
	server.Use(middleware.NewAuthMiddleware(c.Auth.AccessSecret, c.InternalAuth.Secret, middleware.PerfAuthConfig{
		Enabled:      c.PerfMode.Enabled,
		HeaderName:   c.PerfMode.HeaderName,
		HeaderSecret: c.PerfMode.HeaderSecret,
		UserIDHeader: c.PerfMode.UserIDHeader,
		AllowedPaths: toAllowedPathMap(c.PerfMode.AllowedPaths),
	}).Handle)
	middleware.RegisterPreflightRoutes(server, c.Upstreams)
	defer server.Stop()

	server.Start()
}

func toAllowedPathMap(paths []string) map[string]struct{} {
	if len(paths) == 0 {
		return nil
	}

	resp := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		resp[path] = struct{}{}
	}

	if len(resp) == 0 {
		return nil
	}

	return resp
}
