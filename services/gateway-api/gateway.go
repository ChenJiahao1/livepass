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
	server.Use(middleware.NewAuthMiddleware(c.Auth.AccessSecret, c.InternalAuth.Secret).Handle)
	middleware.RegisterPreflightRoutes(server, c.Upstreams)
	defer server.Stop()

	server.Start()
}
