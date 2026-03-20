package svc

import (
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/model"
	payrpc "damai-go/services/pay-rpc/payrpc"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config                config.Config
	SqlConn               sqlx.SqlConn
	DOrderModel           model.DOrderModel
	DOrderTicketUserModel model.DOrderTicketUserModel
	ProgramRpc            programrpc.ProgramRpc
	PayRpc                payrpc.PayRpc
	UserRpc               userrpc.UserRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.MySQL.DataSource)

	return &ServiceContext{
		Config:                c,
		SqlConn:               conn,
		DOrderModel:           model.NewDOrderModel(conn),
		DOrderTicketUserModel: model.NewDOrderTicketUserModel(conn),
		ProgramRpc:            programrpc.NewProgramRpc(zrpc.MustNewClient(c.ProgramRpc)),
		PayRpc:                payrpc.NewPayRpc(zrpc.MustNewClient(c.PayRpc)),
		UserRpc:               userrpc.NewUserRpc(zrpc.MustNewClient(c.UserRpc)),
	}
}
