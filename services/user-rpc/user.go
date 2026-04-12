package main

import (
	"flag"
	"fmt"

	"damai-go/pkg/xid"
	"damai-go/services/user-rpc/internal/config"
	"damai-go/services/user-rpc/internal/server"
	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/user-rpc.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
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

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterUserRpcServer(grpcServer, server.NewUserRpcServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
