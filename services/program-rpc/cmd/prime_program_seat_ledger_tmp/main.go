package main

import (
	"context"
	"fmt"

	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
)

func main() {
	var c config.Config
	conf.MustLoad("services/program-rpc/etc/program-rpc.yaml", &c)

	svcCtx := svc.NewServiceContext(c)
	list, err := svcCtx.DTicketCategoryModel.FindByShowTimeId(context.Background(), 30001)
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
		if err := svcCtx.SeatStockStore.PrimeFromDB(context.Background(), 30001, item.Id); err != nil {
			panic(err)
		}
		snapshot, err := svcCtx.SeatStockStore.Snapshot(context.Background(), 30001, item.Id)
		if err != nil {
			panic(err)
		}
		fmt.Printf("primed ticketCategoryId=%d ready=%v available=%d\n", item.Id, snapshot.Ready, snapshot.AvailableCount)
	}
}
