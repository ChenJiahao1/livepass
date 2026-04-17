// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"livepass/services/program-api/internal/config"
	"livepass/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config     config.Config
	ProgramRpc programrpc.ProgramRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:     c,
		ProgramRpc: programrpc.NewProgramRpc(zrpc.MustNewClient(c.ProgramRpc)),
	}
}
