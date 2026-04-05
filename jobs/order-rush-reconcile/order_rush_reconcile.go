package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"damai-go/jobs/order-rush-reconcile/internal/config"
	"damai-go/jobs/order-rush-reconcile/internal/logic"
	"damai-go/jobs/order-rush-reconcile/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/order-rush-reconcile.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serviceContext := svc.NewServiceContext(c)
	runner := logic.NewReconcileRushAttemptsLogic(ctx, serviceContext)
	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	if err := runner.RunOnce(); err != nil {
		logx.WithContext(ctx).Errorf("initial order-rush-reconcile run failed: %v", err)
	}

	fmt.Printf("Starting order-rush-reconcile job, interval=%s batchSize=%d\n", c.Interval, c.BatchSize)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := runner.RunOnce(); err != nil {
				logx.WithContext(ctx).Errorf("scheduled order-rush-reconcile run failed: %v", err)
			}
		}
	}
}
