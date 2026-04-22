package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"livepass/jobs/order-close/internal/config"
	"livepass/jobs/order-close/internal/dispatch"
	"livepass/jobs/order-close/internal/svc"
	"livepass/jobs/order-close/taskdef"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/order-close-dispatcher.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serviceContext := svc.NewDispatcherServiceContext(c)
	runner := dispatch.NewRunOnceLogic(ctx, serviceContext.Store, serviceContext.Publisher)
	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	if err := runner.Run(taskdef.TaskTypeCloseTimeout); err != nil {
		logx.WithContext(ctx).Errorf("initial order-close dispatcher run failed: %v", err)
	}

	fmt.Printf("Starting order-close dispatcher, interval=%s\n", c.Interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := runner.Run(taskdef.TaskTypeCloseTimeout); err != nil {
				logx.WithContext(ctx).Errorf("scheduled order-close dispatcher run failed: %v", err)
			}
		}
	}
}
