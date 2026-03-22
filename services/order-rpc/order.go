package main

import (
	"context"
	"flag"
	"fmt"

	"damai-go/pkg/xid"
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/server"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/order-rpc.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	xid.MustInitEtcd(context.Background(), xid.Config{
		Hosts:   c.Etcd.Hosts,
		Service: "order-rpc",
	})
	defer func() {
		_ = xid.Close()
	}()
	ctx := svc.NewServiceContext(c)
	stopOrderCreateConsumer := logic.StartOrderCreateConsumer(context.Background(), ctx)
	defer stopOrderCreateConsumer()
	defer func() {
		if ctx.OrderCreateProducer != nil {
			_ = ctx.OrderCreateProducer.Close()
		}
	}()

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterOrderRpcServer(grpcServer, server.NewOrderRpcServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
