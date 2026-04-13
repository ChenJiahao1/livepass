package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/svc"
	programrpc "damai-go/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/conf"
)

func main() {
	var (
		configFile = flag.String("config", "services/order-rpc/etc/order.yaml", "order-rpc config file")
		showTimeID = flag.Int64("showTimeId", 0, "show time id")
	)
	flag.Parse()

	fmt.Fprintln(os.Stderr, "debug-only: prime_admission_quota_tmp is for manual troubleshooting, not production flow")

	if *showTimeID <= 0 {
		panic("showTimeId must be greater than 0")
	}

	var c config.Config
	conf.MustLoad(*configFile, &c)

	svcCtx := svc.NewServiceContext(c)
	if err := logic.PrimeAdmissionQuota(context.Background(), svcCtx, *showTimeID); err != nil {
		panic(err)
	}

	preorder, err := svcCtx.ProgramRpc.GetProgramPreorder(context.Background(), &programrpc.GetProgramPreorderReq{
		ShowTimeId: *showTimeID,
	})
	if err != nil {
		panic(err)
	}

	resolvedShowTimeID := preorder.GetShowTimeId()
	if resolvedShowTimeID <= 0 {
		resolvedShowTimeID = *showTimeID
	}
	for _, item := range preorder.GetTicketCategoryVoList() {
		if item == nil || item.GetId() <= 0 {
			continue
		}
		fmt.Printf(
			"primed quota showTimeId=%d ticketCategoryId=%d quota=%d\n",
			resolvedShowTimeID,
			item.GetId(),
			item.GetAdmissionQuota(),
		)
	}
}
