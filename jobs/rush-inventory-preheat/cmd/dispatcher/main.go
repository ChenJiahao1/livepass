package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"livepass/jobs/rush-inventory-preheat/internal/config"
	"livepass/jobs/rush-inventory-preheat/internal/dispatch"
	"livepass/jobs/rush-inventory-preheat/internal/svc"
	"livepass/jobs/rush-inventory-preheat/taskdef"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/rush-inventory-preheat-dispatcher.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.DispatcherConfig
	conf.MustLoad(*configFile, &c)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serviceContext := svc.NewDispatcherServiceContext(c)
	runner := dispatch.NewRunOnceLogic(ctx, serviceContext.Store, serviceContext.Publisher, c.BatchSize)
	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	if err := runner.Run(taskdef.TaskTypeRushInventoryPreheat); err != nil {
		logx.WithContext(ctx).Errorf("initial rush-inventory-preheat dispatcher run failed: %v", err)
	}

	fmt.Printf("Starting rush-inventory-preheat dispatcher, interval=%s batchSize=%d\n", c.Interval, c.BatchSize)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := runner.Run(taskdef.TaskTypeRushInventoryPreheat); err != nil {
				logx.WithContext(ctx).Errorf("scheduled rush-inventory-preheat dispatcher run failed: %v", err)
			}
		}
	}
}
