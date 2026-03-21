package main

import (
	"flag"

	"damai-go/services/gateway-api/internal/config"
	"damai-go/services/gateway-api/internal/middleware"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/gateway"
)

var configFile = flag.String("f", "etc/gateway-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := gateway.MustNewServer(
		c.GatewayConf,
		gateway.WithMiddleware(
			middleware.NewAuthMiddleware(c.Auth.ChannelCodeHeader, c.Auth.ChannelMap).Handle,
		),
	)
	defer server.Stop()

	server.Start()
}
