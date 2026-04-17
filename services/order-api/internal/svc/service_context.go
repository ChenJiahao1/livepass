// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"livepass/services/order-api/internal/config"
	"livepass/services/order-api/internal/middleware"
	"livepass/services/order-rpc/orderrpc"
	"time"

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
		Auth:     middleware.NewAuthMiddleware(c.GatewayAuth.Secret, time.Duration(c.GatewayAuth.MaxClockSkewSeconds)*time.Second).Handle,
		OrderRpc: orderrpc.NewOrderRpc(zrpc.MustNewClient(c.OrderRpc)),
	}
}
