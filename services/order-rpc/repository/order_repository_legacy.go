package repository

import (
	"context"
	"errors"
	"time"

	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type legacyOrderRepository struct {
	deps     Dependencies
	resolver routeResolver
}

func newLegacyOrderRepository(deps Dependencies) *legacyOrderRepository {
	return &legacyOrderRepository{
		deps: deps,
		resolver: routeResolver{
			router:           deps.Router,
			routeMap:         deps.RouteMap,
			legacyRouteModel: deps.LegacyRouteDirectoryModel,
		},
	}
}

func (r *legacyOrderRepository) TransactByOrderNumber(ctx context.Context, orderNumber int64, fn func(context.Context, OrderTx) error) error {
	route, err := r.RouteByOrderNumber(ctx, orderNumber)
	if err != nil {
		return err
	}
	return r.deps.LegacyConn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		tx := newSingleOrderTx(route, session, r.deps.LegacyOrderModel, r.deps.LegacyOrderTicketUserModel, r.deps.LegacyUserOrderIndexModel, r.deps.LegacyRouteDirectoryModel)
		return fn(ctx, tx)
	})
}

func (r *legacyOrderRepository) TransactByUserID(ctx context.Context, userID int64, fn func(context.Context, OrderTx) error) error {
	route, err := r.RouteByUserID(ctx, userID)
	if err != nil {
		return err
	}
	return r.deps.LegacyConn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		tx := newSingleOrderTx(route, session, r.deps.LegacyOrderModel, r.deps.LegacyOrderTicketUserModel, r.deps.LegacyUserOrderIndexModel, r.deps.LegacyRouteDirectoryModel)
		return fn(ctx, tx)
	})
}

func (r *legacyOrderRepository) FindOrderByNumber(ctx context.Context, orderNumber int64) (*model.DOrder, error) {
	return r.deps.LegacyOrderModel.FindOneByOrderNumber(ctx, orderNumber)
}

func (r *legacyOrderRepository) FindOrderTicketsByNumber(ctx context.Context, orderNumber int64) ([]*model.DOrderTicketUser, error) {
	return r.deps.LegacyOrderTicketUserModel.FindByOrderNumber(ctx, orderNumber)
}

func (r *legacyOrderRepository) FindOrderPageByUser(ctx context.Context, userID, orderStatus, pageNumber, pageSize int64) ([]*model.DOrder, int64, error) {
	return r.deps.LegacyOrderModel.FindPageByUserAndStatus(ctx, userID, orderStatus, pageNumber, pageSize)
}

func (r *legacyOrderRepository) FindExpiredUnpaidBySlot(ctx context.Context, logicSlot int, before time.Time, limit int64) ([]*model.DOrder, error) {
	if logicSlot != 0 {
		return []*model.DOrder{}, nil
	}
	return r.deps.LegacyOrderModel.FindExpiredUnpaid(ctx, before, limit)
}

func (r *legacyOrderRepository) CountActiveTicketsByUserProgram(ctx context.Context, userID, programID int64) (int64, error) {
	return r.deps.LegacyOrderModel.CountActiveTicketsByUserProgram(ctx, userID, programID)
}

func (r *legacyOrderRepository) ListUnpaidReservationsByUserProgram(ctx context.Context, userID, programID int64) (map[int64]int64, error) {
	return r.deps.LegacyOrderModel.ListUnpaidReservationsByUserProgram(ctx, userID, programID)
}

func (r *legacyOrderRepository) RouteByUserID(_ context.Context, userID int64) (sharding.Route, error) {
	route, err := r.resolver.RouteByUserID(userID)
	if err == nil {
		return route, nil
	}

	return sharding.Route{
		LogicSlot:   0,
		DBKey:       "legacy",
		TableSuffix: "",
		Version:     "legacy",
		WriteMode:   sharding.WriteModeLegacyPrimary,
		Status:      sharding.RouteStatusStable,
	}, nil
}

func (r *legacyOrderRepository) RouteByOrderNumber(ctx context.Context, orderNumber int64) (sharding.Route, error) {
	route, err := r.resolver.RouteByOrderNumber(ctx, orderNumber)
	if err == nil {
		return route, nil
	}
	if !errors.Is(err, sharding.ErrLegacyOrderRequiresDirectoryLookup) {
		return sharding.Route{
			LogicSlot:   0,
			DBKey:       "legacy",
			TableSuffix: "",
			Version:     "legacy",
			WriteMode:   sharding.WriteModeLegacyPrimary,
			Status:      sharding.RouteStatusStable,
		}, nil
	}

	return sharding.Route{
		LogicSlot:   0,
		DBKey:       "legacy",
		TableSuffix: "",
		Version:     "legacy",
		WriteMode:   sharding.WriteModeLegacyPrimary,
		Status:      sharding.RouteStatusStable,
	}, nil
}
