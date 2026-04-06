package svc

import (
	"damai-go/jobs/order-rush-reconcile/internal/config"
	"damai-go/services/order-rpc/orderrpc"

	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config   config.Config
	OrderRpc orderrpc.OrderRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:   c,
		OrderRpc: orderrpc.NewOrderRpc(zrpc.MustNewClient(c.OrderRpc)),
	}
}
