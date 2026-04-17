package svc

import (
	"livepass/pkg/xmysql"
	"livepass/pkg/xredis"
	"livepass/services/order-rpc/internal/config"
	"livepass/services/order-rpc/internal/mq"
	"livepass/services/order-rpc/internal/repeatguard"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/repository"
	"livepass/services/order-rpc/sharding"
	payrpc "livepass/services/pay-rpc/payrpc"
	programrpc "livepass/services/program-rpc/programrpc"
	userrpc "livepass/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type ServiceContext struct {
	Config                     config.Config
	SqlConn                    sqlx.SqlConn
	ShardSqlConns              map[string]sqlx.SqlConn
	Redis                      *xredis.Client
	AttemptStore               *rush.AttemptStore
	OrderRouteMap              *sharding.RouteMap
	OrderRouter                sharding.Router
	OrderRepository            repository.OrderRepository
	PurchaseTokenCodec         *rush.PurchaseTokenCodec
	RepeatGuard                repeatguard.Guard
	ProgramRpc                 programrpc.ProgramRpc
	PayRpc                     payrpc.PayRpc
	UserRpc                    userrpc.UserRpc
	OrderCreateProducer        mq.OrderCreateProducer
	OrderCreateConsumerFactory mq.OrderCreateConsumerFactory
}

func NewServiceContext(c config.Config) *ServiceContext {
	c.MySQL = c.MySQL.Normalize()
	c.Sharding = c.Sharding.Normalize()

	sqlConn := mustNewMysqlConn(c.MySQL)
	shardConns := make(map[string]sqlx.SqlConn, len(c.Sharding.Shards))
	for key, shardCfg := range c.Sharding.Shards {
		shardConns[key] = mustNewMysqlConn(shardCfg)
	}

	routeMap := mustNewOrderRouteMap(c.Sharding.RouteMap)
	var orderRouter sharding.Router
	if routeMap != nil {
		orderRouter = sharding.NewStaticRouter(routeMap)
	}
	orderRepository := repository.NewOrderRepository(repository.Dependencies{
		ShardConns: shardConns,
		RouteMap:   routeMap,
		Router:     orderRouter,
	})
	var rds *xredis.Client
	if c.StoreRedis.Host != "" {
		rds = xredis.MustNew(c.StoreRedis)
	}
	var attemptStore *rush.AttemptStore
	if c.RushOrder.Enabled && rds != nil {
		attemptStore = rush.NewAttemptStore(rds, rush.AttemptStoreConfig{
			InFlightTTL:   c.RushOrder.InFlightTTL,
			FinalStateTTL: c.RushOrder.FinalStateTTL,
		})
	}
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   c.Etcd.Hosts,
		DialTimeout: c.RepeatGuard.LockAcquireTimeout,
	})
	if err != nil {
		panic(err)
	}

	var orderCreateProducer mq.OrderCreateProducer
	var orderCreateConsumerFactory mq.OrderCreateConsumerFactory
	if len(c.Kafka.Brokers) > 0 {
		if err := mq.EnsureOrderCreateTopic(c.Kafka); err != nil {
			panic(err)
		}
		if currentPartitions, err := mq.OrderCreateTopicPartitionCount(c.Kafka); err != nil {
			logx.Errorf("inspect order create topic partitions failed: %v", err)
		} else {
			desiredPartitions := c.Kafka.TopicPartitions
			if desiredPartitions <= 0 {
				desiredPartitions = 1
			}
			if currentPartitions < desiredPartitions {
				logx.Infof("[WARN] order create topic has fewer partitions than configured, topic=%s current=%d desired=%d",
					mq.OrderCreateTopic(c.Kafka), currentPartitions, desiredPartitions)
			}
		}
		orderCreateProducer = mq.NewOrderCreateProducer(c.Kafka)
		orderCreateConsumerFactory = mq.NewOrderCreateConsumerFactory()
	}
	var purchaseTokenCodec *rush.PurchaseTokenCodec
	if c.RushOrder.Enabled && c.RushOrder.TokenSecret != "" {
		purchaseTokenCodec, err = rush.NewPurchaseTokenCodec(c.RushOrder.TokenSecret, c.RushOrder.TokenTTL)
		if err != nil {
			panic(err)
		}
	}

	return &ServiceContext{
		Config:                     c,
		SqlConn:                    sqlConn,
		ShardSqlConns:              shardConns,
		Redis:                      rds,
		AttemptStore:               attemptStore,
		OrderRouteMap:              routeMap,
		OrderRouter:                orderRouter,
		OrderRepository:            orderRepository,
		PurchaseTokenCodec:         purchaseTokenCodec,
		RepeatGuard:                repeatguard.NewEtcdGuard(etcdClient, c.RepeatGuard),
		ProgramRpc:                 newProgramRPC(c.ProgramRpc),
		PayRpc:                     newPayRPC(c.PayRpc),
		UserRpc:                    newUserRPC(c.UserRpc),
		OrderCreateProducer:        orderCreateProducer,
		OrderCreateConsumerFactory: orderCreateConsumerFactory,
	}
}

func mustNewMysqlConn(cfg xmysql.Config) sqlx.SqlConn {
	cfg = cfg.Normalize()
	cfg.DataSource = xmysql.WithLocalTime(cfg.DataSource)

	conn := sqlx.NewMysql(cfg.DataSource)
	rawDB, err := conn.RawDB()
	if err != nil {
		panic(err)
	}
	xmysql.ApplyPool(rawDB, cfg)

	return conn
}

func mustNewOrderRouteMap(cfg config.RouteMapConfig) *sharding.RouteMap {
	if cfg.Version == "" || len(cfg.Entries) == 0 {
		return nil
	}

	entries := make([]sharding.RouteEntry, 0, len(cfg.Entries))
	for _, entry := range cfg.Entries {
		entries = append(entries, sharding.RouteEntry{
			Version:     entry.Version,
			LogicSlot:   entry.LogicSlot,
			DBKey:       entry.DBKey,
			TableSuffix: entry.TableSuffix,
			Status:      entry.Status,
			WriteMode:   entry.WriteMode,
		})
	}

	routeMap, err := sharding.NewRouteMap(cfg.Version, entries)
	if err != nil {
		panic(err)
	}

	return routeMap
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
