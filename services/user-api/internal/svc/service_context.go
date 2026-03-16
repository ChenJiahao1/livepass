// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"damai-go/services/user-api/internal/config"
	"damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config  config.Config
	UserRpc userrpc.UserRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:  c,
		UserRpc: userrpc.NewUserRpc(zrpc.MustNewClient(c.UserRpc)),
	}
}
