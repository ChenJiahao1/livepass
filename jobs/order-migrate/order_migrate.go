package main

import (
	"context"
	"flag"
	"fmt"

	"damai-go/jobs/order-migrate/internal/config"
	"damai-go/jobs/order-migrate/internal/logic"
	"damai-go/jobs/order-migrate/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
)

var (
	configFile = flag.String("f", "etc/order-migrate.yaml", "the config file")
	action     = flag.String("action", "backfill", "the migrate action: backfill|verify|switch|rollback")
)

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	serviceContext, err := svc.NewServiceContext(c)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	switch *action {
	case "backfill":
		resp, err := logic.NewBackfillOrdersLogic(ctx, serviceContext).BackfillOrders()
		if err != nil {
			panic(err)
		}
		fmt.Printf("backfill finished, processed=%d lastOrderID=%d\n", resp.ProcessedCount, resp.LastOrderID)
	case "verify":
		resp, err := logic.NewVerifyOrdersLogic(ctx, serviceContext).VerifyOrders()
		if err != nil {
			panic(err)
		}
		fmt.Printf("verify finished, verifiedSlots=%d comparedOrders=%d lastOrderID=%d completed=%t\n", resp.VerifiedSlots, resp.ComparedOrders, resp.LastOrderID, resp.Completed)
	case "switch":
		resp, err := logic.NewSwitchSlotsLogic(ctx, serviceContext).SwitchSlots()
		if err != nil {
			panic(err)
		}
		fmt.Printf("switch finished, updatedSlots=%d\n", resp.UpdatedSlots)
	case "rollback":
		resp, err := logic.NewRollbackSlotsLogic(ctx, serviceContext).RollbackSlots()
		if err != nil {
			panic(err)
		}
		fmt.Printf("rollback finished, updatedSlots=%d\n", resp.UpdatedSlots)
	default:
		panic(fmt.Sprintf("unsupported action: %s", *action))
	}
}
