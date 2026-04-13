package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type routeResolver struct {
	router   sharding.Router
	routeMap *sharding.RouteMap
}

func (r routeResolver) RouteByUserID(userID int64) (sharding.Route, error) {
	if r.router == nil {
		return sharding.Route{}, sharding.ErrRouteNotFound
	}
	return r.router.RouteByUserID(userID)
}

func (r routeResolver) RouteByOrderNumber(ctx context.Context, orderNumber int64) (sharding.Route, error) {
	_ = ctx
	if r.router != nil {
		return r.router.RouteByOrderNumber(orderNumber)
	}
	return sharding.Route{}, sharding.ErrRouteNotFound
}

type shardedOrderRepository struct {
	deps     Dependencies
	resolver routeResolver
}

func newShardedOrderRepository(deps Dependencies) *shardedOrderRepository {
	return &shardedOrderRepository{
		deps: deps,
		resolver: routeResolver{
			router:   deps.Router,
			routeMap: deps.RouteMap,
		},
	}
}

func (r *shardedOrderRepository) TransactByOrderNumber(ctx context.Context, orderNumber int64, fn func(context.Context, OrderTx) error) error {
	route, err := r.RouteByOrderNumber(ctx, orderNumber)
	if err != nil {
		return err
	}
	return r.transactByRoute(ctx, route, fn)
}

func (r *shardedOrderRepository) TransactByUserID(ctx context.Context, userID int64, fn func(context.Context, OrderTx) error) error {
	route, err := r.RouteByUserID(ctx, userID)
	if err != nil {
		return err
	}
	return r.transactByRoute(ctx, route, fn)
}

func (r *shardedOrderRepository) FindOrderByNumber(ctx context.Context, orderNumber int64) (*model.DOrder, error) {
	route, err := r.RouteByOrderNumber(ctx, orderNumber)
	if err != nil {
		return nil, err
	}
	target, err := r.storeForRoute(route)
	if err != nil {
		return nil, err
	}
	return target.orderModel.FindOneByOrderNumber(ctx, orderNumber)
}

func (r *shardedOrderRepository) FindOrderTicketsByNumber(ctx context.Context, orderNumber int64) ([]*model.DOrderTicketUser, error) {
	route, err := r.RouteByOrderNumber(ctx, orderNumber)
	if err != nil {
		return nil, err
	}
	target, err := r.storeForRoute(route)
	if err != nil {
		return nil, err
	}
	return target.ticketModel.FindByOrderNumber(ctx, orderNumber)
}

func (r *shardedOrderRepository) FindOrderPageByUser(ctx context.Context, userID, orderStatus, pageNumber, pageSize int64) ([]*model.DOrder, int64, error) {
	route, err := r.RouteByUserID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	target, err := r.storeForRoute(route)
	if err != nil {
		return nil, 0, err
	}

	return target.orderModel.FindPageByUserAndStatus(ctx, userID, orderStatus, pageNumber, pageSize)
}

func (r *shardedOrderRepository) FindExpiredUnpaidBySlot(ctx context.Context, logicSlot int, before time.Time, limit int64) ([]*model.DOrder, error) {
	if r.deps.RouteMap == nil {
		return nil, sharding.ErrRouteNotFound
	}

	route, err := r.deps.RouteMap.RouteByLogicSlot(logicSlot)
	if err != nil {
		return nil, err
	}
	target, err := r.storeForRoute(route)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		return []*model.DOrder{}, nil
	}

	queryLimit := limit
	if queryLimit < 32 {
		queryLimit = 32
	}

	orders := make([]*model.DOrder, 0, limit)
	var (
		cursorExpireTime time.Time
		cursorID         int64
		hasCursor        bool
	)

	for int64(len(orders)) < limit {
		query := fmt.Sprintf(
			"select * from %s where `status` = 1 and `order_status` = 1 and `order_expire_time` <= ?",
			shardOrderTable(route.TableSuffix),
		)
		args := []interface{}{before}
		if hasCursor {
			query += " and (`order_expire_time` > ? or (`order_expire_time` = ? and `id` > ?))"
			args = append(args, cursorExpireTime, cursorExpireTime, cursorID)
		}
		query += " order by `order_expire_time` asc, `id` asc limit ?"
		args = append(args, queryLimit)

		var batch []*model.DOrder
		err := target.conn.QueryRowsCtx(ctx, &batch, query, args...)
		switch {
		case err == nil:
		case errors.Is(err, sqlx.ErrNotFound):
			return orders, nil
		default:
			return nil, err
		}
		if len(batch) == 0 {
			return orders, nil
		}

		for _, order := range batch {
			if order == nil {
				continue
			}
			if orderMatchesLogicSlot(order, logicSlot) {
				orders = append(orders, order)
				if int64(len(orders)) >= limit {
					return orders, nil
				}
			}
		}

		last := batch[len(batch)-1]
		cursorExpireTime = last.OrderExpireTime
		cursorID = last.Id
		hasCursor = true

		if int64(len(batch)) < queryLimit {
			return orders, nil
		}
	}

	return orders, nil
}

func (r *shardedOrderRepository) CountActiveTicketsByUserShowTime(ctx context.Context, userID, showTimeID int64) (int64, error) {
	route, err := r.RouteByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	target, err := r.storeForRoute(route)
	if err != nil {
		return 0, err
	}

	return target.orderModel.CountActiveTicketsByUserShowTime(ctx, userID, showTimeID)
}

func (r *shardedOrderRepository) ListUnpaidReservationsByUserShowTime(ctx context.Context, userID, showTimeID int64) (map[int64]int64, error) {
	route, err := r.RouteByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	target, err := r.storeForRoute(route)
	if err != nil {
		return nil, err
	}

	return target.orderModel.ListUnpaidReservationsByUserShowTime(ctx, userID, showTimeID)
}

func (r *shardedOrderRepository) RouteByUserID(_ context.Context, userID int64) (sharding.Route, error) {
	return r.resolver.RouteByUserID(userID)
}

func (r *shardedOrderRepository) RouteByOrderNumber(ctx context.Context, orderNumber int64) (sharding.Route, error) {
	return r.resolver.RouteByOrderNumber(ctx, orderNumber)
}

func (r *shardedOrderRepository) transactByRoute(ctx context.Context, route sharding.Route, fn func(context.Context, OrderTx) error) error {
	target, err := r.storeForRoute(route)
	if err != nil {
		return err
	}
	return target.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		tx := newSingleOrderTx(
			route,
			session,
			target.orderModel,
			target.ticketModel,
			target.userGuardModel,
			target.viewerGuardModel,
			target.seatGuardModel,
			target.outboxModel,
			target.delayTaskModel,
		)
		return fn(ctx, tx)
	})
}

type routeStore struct {
	conn             sqlx.SqlConn
	orderModel       model.DOrderModel
	ticketModel      model.DOrderTicketUserModel
	userGuardModel   model.DOrderUserGuardModel
	viewerGuardModel model.DOrderViewerGuardModel
	seatGuardModel   model.DOrderSeatGuardModel
	outboxModel      model.DOrderOutboxModel
	delayTaskModel   model.DDelayTaskOutboxModel
}

func (r *shardedOrderRepository) storeForRoute(route sharding.Route) (*routeStore, error) {
	conn, ok := r.deps.ShardConns[route.DBKey]
	if !ok {
		return nil, fmt.Errorf("shard db key not configured: %s", route.DBKey)
	}

	return &routeStore{
		conn:             conn,
		orderModel:       model.NewDOrderModelWithTable(conn, shardOrderTable(route.TableSuffix)),
		ticketModel:      model.NewDOrderTicketUserModelWithTable(conn, shardOrderTicketTable(route.TableSuffix)),
		userGuardModel:   model.NewDOrderUserGuardModelWithTable(conn, shardOrderUserGuardTable(route.TableSuffix)),
		viewerGuardModel: model.NewDOrderViewerGuardModelWithTable(conn, shardOrderViewerGuardTable(route.TableSuffix)),
		seatGuardModel:   model.NewDOrderSeatGuardModelWithTable(conn, shardOrderSeatGuardTable(route.TableSuffix)),
		outboxModel:      model.NewDOrderOutboxModelWithTable(conn, shardOrderOutboxTable(route.TableSuffix)),
		delayTaskModel:   model.NewDDelayTaskOutboxModelWithTable(conn, delayTaskOutboxTable()),
	}, nil
}

func shardOrderTable(suffix string) string {
	return "d_order_" + suffix
}

func shardOrderTicketTable(suffix string) string {
	return "d_order_ticket_user_" + suffix
}

func shardOrderUserGuardTable(suffix string) string {
	_ = suffix
	return "d_order_user_guard"
}

func shardOrderViewerGuardTable(suffix string) string {
	_ = suffix
	return "d_order_viewer_guard"
}

func shardOrderSeatGuardTable(suffix string) string {
	_ = suffix
	return "d_order_seat_guard"
}

func shardOrderOutboxTable(suffix string) string {
	_ = suffix
	return "d_order_outbox"
}

func delayTaskOutboxTable() string {
	return "d_delay_task_outbox"
}

func orderMatchesLogicSlot(order *model.DOrder, logicSlot int) bool {
	if order == nil {
		return false
	}

	slot, err := sharding.LogicSlotByOrderNumber(order.OrderNumber)
	if err != nil {
		return false
	}

	return slot == logicSlot
}
