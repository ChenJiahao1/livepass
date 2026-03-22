package main

import (
	"context"
	"flag"
	"fmt"

	"damai-go/pkg/xid"
	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/internal/server"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/program-rpc.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	xid.MustInitEtcd(context.Background(), xid.Config{
		Hosts:   c.Etcd.Hosts,
		Service: "program-rpc",
	})
	defer func() {
		_ = xid.Close()
	}()
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterProgramRpcServer(grpcServer, server.NewProgramRpcServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
