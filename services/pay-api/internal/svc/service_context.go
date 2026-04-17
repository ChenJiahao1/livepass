// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"livepass/services/pay-api/internal/config"
	payrpc "livepass/services/pay-rpc/payrpc"

	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config config.Config
	PayRpc payrpc.PayRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
		PayRpc: payrpc.NewPayRpc(zrpc.MustNewClient(c.PayRpc)),
	}
}
