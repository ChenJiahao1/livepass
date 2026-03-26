package repository

import (
	"context"
	"time"

	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type OrderTx interface {
	Route() sharding.Route
	InsertOrder(ctx context.Context, order *model.DOrder) error
	InsertOrderTickets(ctx context.Context, tickets []*model.DOrderTicketUser) error
	InsertLegacyRoute(ctx context.Context, legacyRoute *model.DOrderRouteLegacy) error
	FindOrderByNumberForUpdate(ctx context.Context, orderNumber int64) (*model.DOrder, error)
	UpdateCancelStatus(ctx context.Context, orderNumber int64, cancelTime time.Time) error
	UpdatePayStatus(ctx context.Context, orderNumber int64, payTime time.Time) error
	UpdateRefundStatus(ctx context.Context, orderNumber int64, refundTime time.Time) error
}

type OrderRepository interface {
	TransactByOrderNumber(ctx context.Context, orderNumber int64, fn func(context.Context, OrderTx) error) error
	TransactByUserID(ctx context.Context, userID int64, fn func(context.Context, OrderTx) error) error
	FindOrderByNumber(ctx context.Context, orderNumber int64) (*model.DOrder, error)
	FindOrderTicketsByNumber(ctx context.Context, orderNumber int64) ([]*model.DOrderTicketUser, error)
	FindOrderPageByUser(ctx context.Context, userID, orderStatus, pageNumber, pageSize int64) ([]*model.DOrder, int64, error)
	FindExpiredUnpaidBySlot(ctx context.Context, logicSlot int, before time.Time, limit int64) ([]*model.DOrder, error)
	CountActiveTicketsByUserProgram(ctx context.Context, userID, programID int64) (int64, error)
	ListUnpaidReservationsByUserProgram(ctx context.Context, userID, programID int64) (map[int64]int64, error)
	RouteByUserID(ctx context.Context, userID int64) (sharding.Route, error)
	RouteByOrderNumber(ctx context.Context, orderNumber int64) (sharding.Route, error)
}

type Dependencies struct {
	Mode                       string
	LegacyConn                 sqlx.SqlConn
	LegacyOrderModel           model.DOrderModel
	LegacyOrderTicketUserModel model.DOrderTicketUserModel
	LegacyRouteDirectoryModel  model.DOrderRouteLegacyModel
	ShardConns                 map[string]sqlx.SqlConn
	RouteMap                   *sharding.RouteMap
	Router                     sharding.Router
}

func NewOrderRepository(deps Dependencies) OrderRepository {
	legacyRepo := newLegacyOrderRepository(deps)
	if deps.Router == nil || deps.RouteMap == nil || len(deps.ShardConns) == 0 {
		return legacyRepo
	}

	shardedRepo := newShardedOrderRepository(deps)
	switch deps.Mode {
	case sharding.MigrationModeShardOnly:
		return shardedRepo
	case sharding.MigrationModeDualWriteShadow, sharding.MigrationModeDualWriteReadOld, sharding.MigrationModeDualWriteReadNew:
		return newDualWriteOrderRepository(deps.Mode, legacyRepo, shardedRepo)
	default:
		return legacyRepo
	}
}
