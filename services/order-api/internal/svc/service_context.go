// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"damai-go/services/order-api/internal/config"
	"damai-go/services/order-api/internal/middleware"
	"damai-go/services/order-rpc/orderrpc"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config   config.Config
	Auth     rest.Middleware
	OrderRpc orderrpc.OrderRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:   c,
		Auth:     middleware.NewAuthMiddleware(c.Auth.ChannelCodeHeader, c.Auth.ChannelMap).Handle,
		OrderRpc: orderrpc.NewOrderRpc(zrpc.MustNewClient(c.OrderRpc)),
	}
}
