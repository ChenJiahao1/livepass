package svc

import (
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/limitcache"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/mq"
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
	Redis                 *xredis.Client
	PurchaseLimitStore    *limitcache.PurchaseLimitStore
	DOrderModel           model.DOrderModel
	DOrderTicketUserModel model.DOrderTicketUserModel
	RepeatGuard           repeatguard.Guard
	ProgramRpc            programrpc.ProgramRpc
	PayRpc                payrpc.PayRpc
	UserRpc               userrpc.UserRpc
	OrderCreateProducer   mq.OrderCreateProducer
	OrderCreateConsumer   mq.OrderCreateConsumer
}

func NewServiceContext(c config.Config) *ServiceContext {
	c.MySQL.DataSource = xmysql.WithLocalTime(c.MySQL.DataSource)
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	var rds *xredis.Client
	if c.StoreRedis.Host != "" {
		rds = xredis.MustNew(c.StoreRedis)
	}
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   c.Etcd.Hosts,
		DialTimeout: c.RepeatGuard.LockAcquireTimeout,
	})
	if err != nil {
		panic(err)
	}

	var orderCreateProducer mq.OrderCreateProducer
	var orderCreateConsumer mq.OrderCreateConsumer
	if len(c.Kafka.Brokers) > 0 {
		if err := mq.EnsureOrderCreateTopic(c.Kafka); err != nil {
			panic(err)
		}
		orderCreateProducer = mq.NewOrderCreateProducer(c.Kafka)
		orderCreateConsumer = mq.NewOrderCreateConsumer(c.Kafka)
	}

	return &ServiceContext{
		Config:                c,
		SqlConn:               conn,
		Redis:                 rds,
		PurchaseLimitStore:    limitcache.NewPurchaseLimitStore(rds, model.NewDOrderModel(conn), limitcache.Config{}),
		DOrderModel:           model.NewDOrderModel(conn),
		DOrderTicketUserModel: model.NewDOrderTicketUserModel(conn),
		RepeatGuard:           repeatguard.NewEtcdGuard(etcdClient, c.RepeatGuard),
		ProgramRpc:            newProgramRPC(c.ProgramRpc),
		PayRpc:                newPayRPC(c.PayRpc),
		UserRpc:               newUserRPC(c.UserRpc),
		OrderCreateProducer:   orderCreateProducer,
		OrderCreateConsumer:   orderCreateConsumer,
	}
}

func hasRPCClientConf(conf zrpc.RpcClientConf) bool {
	return len(conf.Endpoints) > 0 || conf.Target != "" || len(conf.Etcd.Hosts) > 0
}

func newProgramRPC(conf zrpc.RpcClientConf) programrpc.ProgramRpc {
	if !hasRPCClientConf(conf) {
		return nil
	}

	return programrpc.NewProgramRpc(zrpc.MustNewClient(conf))
}

func newPayRPC(conf zrpc.RpcClientConf) payrpc.PayRpc {
	if !hasRPCClientConf(conf) {
		return nil
	}

	return payrpc.NewPayRpc(zrpc.MustNewClient(conf))
}

func newUserRPC(conf zrpc.RpcClientConf) userrpc.UserRpc {
	if !hasRPCClientConf(conf) {
		return nil
	}

	return userrpc.NewUserRpc(zrpc.MustNewClient(conf))
}
