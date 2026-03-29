package main

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	"damai-go/jobs/order-close-worker/internal/config"
	"damai-go/jobs/order-close-worker/internal/logic"
	"damai-go/jobs/order-close-worker/internal/svc"
	"damai-go/services/order-rpc/closequeue"

	"github.com/hibiken/asynq"
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

	serviceContext := svc.NewServiceContext(c)
	handler := logic.NewCloseTimeoutTaskLogic(serviceContext)
	mux := asynq.NewServeMux()
	mux.HandleFunc(closequeue.TaskTypeCloseTimeout, handler.Handle)

	if err := serviceContext.Server.Start(mux); err != nil {
		logx.WithContext(ctx).Errorf("start order-close-worker failed: %v", err)
		return
	}

	<-ctx.Done()
	serviceContext.Server.Shutdown()
}
