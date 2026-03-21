package svc

import (
	"damai-go/pkg/xmysql"
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/repeatguard"
	payrpc "damai-go/services/pay-rpc/payrpc"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type ServiceContext struct {
	Config                config.Config
	SqlConn               sqlx.SqlConn
	DOrderModel           model.DOrderModel
	DOrderTicketUserModel model.DOrderTicketUserModel
	RepeatGuard           repeatguard.Guard
	ProgramRpc            programrpc.ProgramRpc
	PayRpc                payrpc.PayRpc
	UserRpc               userrpc.UserRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	c.MySQL.DataSource = xmysql.WithLocalTime(c.MySQL.DataSource)
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   c.Etcd.Hosts,
		DialTimeout: c.RepeatGuard.LockAcquireTimeout,
	})
	if err != nil {
		panic(err)
	}

	return &ServiceContext{
		Config:                c,
		SqlConn:               conn,
		DOrderModel:           model.NewDOrderModel(conn),
		DOrderTicketUserModel: model.NewDOrderTicketUserModel(conn),
		RepeatGuard:           repeatguard.NewEtcdGuard(etcdClient, c.RepeatGuard),
		ProgramRpc:            programrpc.NewProgramRpc(zrpc.MustNewClient(c.ProgramRpc)),
		PayRpc:                payrpc.NewPayRpc(zrpc.MustNewClient(c.PayRpc)),
		UserRpc:               userrpc.NewUserRpc(zrpc.MustNewClient(c.UserRpc)),
	}
}
