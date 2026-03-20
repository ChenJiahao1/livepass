// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"damai-go/services/program-api/internal/config"
	"damai-go/services/program-rpc/programrpc"

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
