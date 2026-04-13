package main

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	"damai-go/jobs/order-close/internal/config"
	"damai-go/jobs/order-close/internal/svc"
	"damai-go/jobs/order-close/internal/worker"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/order-close-worker.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serviceContext := svc.NewWorkerServiceContext(c)
	mux := worker.NewServeMux(serviceContext)

	if err := serviceContext.Server.Start(mux); err != nil {
		logx.WithContext(ctx).Errorf("start order-close worker failed: %v", err)
		return
	}

	<-ctx.Done()
	serviceContext.Server.Shutdown()
}
