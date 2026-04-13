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
	InsertUserGuard(ctx context.Context, guard *model.DOrderUserGuard) error
	InsertViewerGuards(ctx context.Context, guards []*model.DOrderViewerGuard) error
	InsertSeatGuards(ctx context.Context, guards []*model.DOrderSeatGuard) error
	InsertOutbox(ctx context.Context, rows []*model.DOrderOutbox) error
	InsertDelayTasks(ctx context.Context, rows []*model.DDelayTaskOutbox) error
	DeleteGuardsByOrderNumber(ctx context.Context, orderNumber int64) error
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
	CountActiveTicketsByUserShowTime(ctx context.Context, userID, showTimeID int64) (int64, error)
	ListUnpaidReservationsByUserShowTime(ctx context.Context, userID, showTimeID int64) (map[int64]int64, error)
	WalkActiveUserGuardsByShowTime(ctx context.Context, showTimeID, batchSize int64, fn func([]*model.DOrderUserGuard) error) error
	WalkActiveViewerGuardsByShowTime(ctx context.Context, showTimeID, batchSize int64, fn func([]*model.DOrderViewerGuard) error) error
	RouteByUserID(ctx context.Context, userID int64) (sharding.Route, error)
	RouteByOrderNumber(ctx context.Context, orderNumber int64) (sharding.Route, error)
}

type Dependencies struct {
	ShardConns map[string]sqlx.SqlConn
	RouteMap   *sharding.RouteMap
	Router     sharding.Router
}

func NewOrderRepository(deps Dependencies) OrderRepository {
	return newShardedOrderRepository(deps)
}
