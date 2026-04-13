package main

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	"damai-go/jobs/rush-inventory-preheat-worker/internal/config"
	"damai-go/jobs/rush-inventory-preheat-worker/internal/svc"
	"damai-go/jobs/rush-inventory-preheat-worker/internal/worker"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/rush-inventory-preheat-worker.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serviceContext := svc.NewServiceContext(c)
	mux := worker.NewServeMux(serviceContext)

	if err := serviceContext.Server.Start(mux); err != nil {
		logx.WithContext(ctx).Errorf("start rush-inventory-preheat-worker failed: %v", err)
		return
	}

	<-ctx.Done()
	serviceContext.Server.Shutdown()
}
