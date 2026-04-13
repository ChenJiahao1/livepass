// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"damai-go/services/user-api/internal/config"
	"damai-go/services/user-api/internal/middleware"
	"damai-go/services/user-rpc/userrpc"
	"time"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config  config.Config
	Auth    rest.Middleware
	UserRpc userrpc.UserRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:  c,
		Auth:    middleware.NewAuthMiddleware(c.GatewayAuth.Secret, time.Duration(c.GatewayAuth.MaxClockSkewSeconds)*time.Second).Handle,
		UserRpc: userrpc.NewUserRpc(zrpc.MustNewClient(c.UserRpc)),
	}
}
