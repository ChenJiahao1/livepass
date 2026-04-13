package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
)

func main() {
	var (
		configFile = flag.String("config", "services/program-rpc/etc/program-rpc.yaml", "program-rpc config file")
		showTimeID = flag.Int64("showTimeId", 0, "show time id")
	)
	flag.Parse()

	fmt.Fprintln(os.Stderr, "debug-only: prime_program_seat_ledger_tmp is for manual troubleshooting, not production flow")

	if *showTimeID <= 0 {
		panic("showTimeId must be greater than 0")
	}

	var c config.Config
	conf.MustLoad(*configFile, &c)

	svcCtx := svc.NewServiceContext(c)
	list, err := svcCtx.DTicketCategoryModel.FindByShowTimeId(context.Background(), *showTimeID)
	if err != nil {
		panic(err)
	}
	if svcCtx.SeatStockStore == nil {
		panic("seat stock store is nil")
	}

	for _, item := range list {
		if item == nil || item.Id <= 0 {
			continue
		}
		if err := svcCtx.SeatStockStore.PrimeFromDB(context.Background(), *showTimeID, item.Id); err != nil {
			panic(err)
		}
		snapshot, err := svcCtx.SeatStockStore.Snapshot(context.Background(), *showTimeID, item.Id)
		if err != nil {
			panic(err)
		}
		fmt.Printf(
			"primed seat ledger showTimeId=%d ticketCategoryId=%d ready=%v available=%d\n",
			*showTimeID,
			item.Id,
			snapshot.Ready,
			snapshot.AvailableCount,
		)
	}
}
