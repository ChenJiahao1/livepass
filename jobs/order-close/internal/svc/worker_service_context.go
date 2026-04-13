package svc

import (
	"context"

	"damai-go/jobs/order-close/internal/config"
	orderrpc "damai-go/services/order-rpc/orderrpc"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

type WorkerOrderCloseRPC interface {
	CloseExpiredOrder(ctx context.Context, in *orderrpc.CloseExpiredOrderReq, opts ...grpc.CallOption) (*orderrpc.BoolResp, error)
}

type WorkerServiceContext struct {
	Config   config.Config
	Server   *asynq.Server
	OrderRpc WorkerOrderCloseRPC
}

func NewWorkerServiceContext(c config.Config) *WorkerServiceContext {
	return &WorkerServiceContext{
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
		OrderRpc: orderrpc.NewOrderRpc(zrpc.MustNewClient(c.OrderRpc)),
	}
}
