package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"livepass/services/order-rpc/internal/config"
	"livepass/services/order-rpc/internal/logic"
	"livepass/services/order-rpc/internal/svc"
	programrpc "livepass/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/conf"
)

func main() {
	var (
		configFile = flag.String("config", "services/order-rpc/etc/order.yaml", "order-rpc config file")
		programID  = flag.Int64("programId", 0, "program id")
	)
	flag.Parse()

	fmt.Fprintln(os.Stderr, "debug-only: prime_rush_runtime_tmp triggers PrimeRushRuntime for manual troubleshooting, not production flow")

	if *programID <= 0 {
		panic("programId must be greater than 0")
	}

	var c config.Config
	conf.MustLoad(*configFile, &c)

	svcCtx := svc.NewServiceContext(c)
	if err := logic.PrimeRushRuntime(context.Background(), svcCtx, *programID); err != nil {
		panic(err)
	}

	showTimes, err := svcCtx.ProgramRpc.ListProgramShowTimesForRush(context.Background(), &programrpc.ListProgramShowTimesForRushReq{
		ProgramId: *programID,
	})
	if err != nil {
		panic(err)
	}

	for _, showTime := range showTimes.GetList() {
		if showTime == nil || showTime.GetShowTimeId() <= 0 {
			continue
		}

		preorder, err := svcCtx.ProgramRpc.GetProgramPreorder(context.Background(), &programrpc.GetProgramPreorderReq{
			ShowTimeId: showTime.GetShowTimeId(),
		})
		if err != nil {
			panic(err)
		}

		resolvedShowTimeID := preorder.GetShowTimeId()
		if resolvedShowTimeID <= 0 {
			resolvedShowTimeID = showTime.GetShowTimeId()
		}
		for _, item := range preorder.GetTicketCategoryVoList() {
			if item == nil || item.GetId() <= 0 {
				continue
			}
			fmt.Printf(
				"primed rush runtime programId=%d showTimeId=%d ticketCategoryId=%d quota=%d\n",
				*programID,
				resolvedShowTimeID,
				item.GetId(),
				item.GetAdmissionQuota(),
			)
		}
	}
}
