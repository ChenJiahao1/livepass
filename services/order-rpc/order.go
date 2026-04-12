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

	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/order-rpc.yaml", "the config file")

func main() {
	flag.Parse()

	c, err := config.Load(*configFile)
	if err != nil {
		panic(err)
	}
	xid.MustInit(xid.Config{
		Provider:          xid.Provider(c.Xid.Provider),
		NodeID:            c.Xid.NodeId,
		ServiceBaseNodeID: c.Xid.ServiceBaseNodeId,
		MaxReplicas:       c.Xid.MaxReplicas,
		PodName:           c.Xid.PodName,
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
