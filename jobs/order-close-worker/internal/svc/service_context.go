package svc

import (
	"context"

	"damai-go/jobs/order-close-worker/internal/config"
	orderrpc "damai-go/services/order-rpc/orderrpc"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/zrpc"
)

type OrderCloseRPC interface {
	CloseExpiredOrder(ctx context.Context, in *orderrpc.CloseExpiredOrderReq) (*orderrpc.BoolResp, error)
}

type ServiceContext struct {
	Config   config.Config
	Server   *asynq.Server
	OrderRpc OrderCloseRPC
}

type orderRPCCloseAdapter struct {
	client orderrpc.OrderRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
		Server: asynq.NewServer(asynq.RedisClientOpt{
			Addr:     c.Asynq.Redis.Host,
			Username: c.Asynq.Redis.User,
			Password: c.Asynq.Redis.Pass,
		}, asynq.Config{
			Concurrency:     c.Asynq.Concurrency,
			Queues:          map[string]int{c.Asynq.Queue: 1},
			ShutdownTimeout: c.Asynq.ShutdownTimeout,
		}),
		OrderRpc: &orderRPCCloseAdapter{
			client: orderrpc.NewOrderRpc(zrpc.MustNewClient(c.OrderRpc)),
		},
	}
}

func (a *orderRPCCloseAdapter) CloseExpiredOrder(ctx context.Context, in *orderrpc.CloseExpiredOrderReq) (*orderrpc.BoolResp, error) {
	return a.client.CloseExpiredOrder(ctx, in)
}
