package svc

import (
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/repository"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func NewMCPServiceContext(c config.MCPConfig) *ServiceContext {
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

	return &ServiceContext{
		Config:          config.Config{},
		SqlConn:         sqlConn,
		ShardSqlConns:   shardConns,
		OrderRouteMap:   routeMap,
		OrderRouter:     orderRouter,
		OrderRepository: orderRepository,
		ProgramRpc:      newProgramRPC(c.ProgramRpc),
		PayRpc:          newPayRPC(c.PayRpc),
		UserRpc:         newUserRPC(c.UserRpc),
	}
}
