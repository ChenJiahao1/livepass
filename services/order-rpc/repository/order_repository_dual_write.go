package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type dualWriteOrderRepository struct {
	mode    string
	primary *legacyOrderRepository
	shadow  *shardedOrderRepository
}

func newDualWriteOrderRepository(mode string, primary *legacyOrderRepository, shadow *shardedOrderRepository) *dualWriteOrderRepository {
	return &dualWriteOrderRepository{
		mode:    mode,
		primary: primary,
		shadow:  shadow,
	}
}

func (r *dualWriteOrderRepository) TransactByOrderNumber(ctx context.Context, orderNumber int64, fn func(context.Context, OrderTx) error) error {
	primaryRoute, err := r.primary.RouteByOrderNumber(ctx, orderNumber)
	if err != nil {
		return err
	}
	shadowRoute, fallbackToPrimary, err := r.shadowRouteByOrderNumber(ctx, orderNumber)
	if err != nil {
		return err
	}
	if fallbackToPrimary {
		return r.primary.TransactByOrderNumber(ctx, orderNumber, fn)
	}
	return r.transact(ctx, primaryRoute, shadowRoute, fn)
}

func (r *dualWriteOrderRepository) TransactByUserID(ctx context.Context, userID int64, fn func(context.Context, OrderTx) error) error {
	primaryRoute, err := r.primary.RouteByUserID(ctx, userID)
	if err != nil {
		return err
	}
	shadowRoute, err := r.shadow.RouteByUserID(ctx, userID)
	if err != nil {
		return err
	}
	return r.transact(ctx, primaryRoute, shadowRoute, fn)
}

func (r *dualWriteOrderRepository) FindOrderByNumber(ctx context.Context, orderNumber int64) (*model.DOrder, error) {
	route, fallbackToPrimary, err := r.shadowRouteByOrderNumber(ctx, orderNumber)
	if err != nil {
		return nil, err
	}
	if fallbackToPrimary {
		return r.primary.FindOrderByNumber(ctx, orderNumber)
	}
	return r.readRepoForRoute(route).FindOrderByNumber(ctx, orderNumber)
}

func (r *dualWriteOrderRepository) FindOrderTicketsByNumber(ctx context.Context, orderNumber int64) ([]*model.DOrderTicketUser, error) {
	route, fallbackToPrimary, err := r.shadowRouteByOrderNumber(ctx, orderNumber)
	if err != nil {
		return nil, err
	}
	if fallbackToPrimary {
		return r.primary.FindOrderTicketsByNumber(ctx, orderNumber)
	}
	return r.readRepoForRoute(route).FindOrderTicketsByNumber(ctx, orderNumber)
}

func (r *dualWriteOrderRepository) FindOrderPageByUser(ctx context.Context, userID, orderStatus, pageNumber, pageSize int64) ([]*model.DOrder, int64, error) {
	route, err := r.shadow.RouteByUserID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return r.readRepoForRoute(route).FindOrderPageByUser(ctx, userID, orderStatus, pageNumber, pageSize)
}

func (r *dualWriteOrderRepository) FindExpiredUnpaidBySlot(ctx context.Context, logicSlot int, before time.Time, limit int64) ([]*model.DOrder, error) {
	if r.shadow.deps.RouteMap == nil {
		return r.readRepo().FindExpiredUnpaidBySlot(ctx, logicSlot, before, limit)
	}
	route, err := r.shadow.deps.RouteMap.RouteByLogicSlot(logicSlot)
	if err != nil {
		return nil, err
	}
	return r.readRepoForRoute(route).FindExpiredUnpaidBySlot(ctx, logicSlot, before, limit)
}

func (r *dualWriteOrderRepository) CountActiveTicketsByUserProgram(ctx context.Context, userID, programID int64) (int64, error) {
	route, err := r.shadow.RouteByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return r.readRepoForRoute(route).CountActiveTicketsByUserProgram(ctx, userID, programID)
}

func (r *dualWriteOrderRepository) ListUnpaidReservationsByUserProgram(ctx context.Context, userID, programID int64) (map[int64]int64, error) {
	route, err := r.shadow.RouteByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return r.readRepoForRoute(route).ListUnpaidReservationsByUserProgram(ctx, userID, programID)
}

func (r *dualWriteOrderRepository) RouteByUserID(ctx context.Context, userID int64) (sharding.Route, error) {
	return r.shadow.RouteByUserID(ctx, userID)
}

func (r *dualWriteOrderRepository) RouteByOrderNumber(ctx context.Context, orderNumber int64) (sharding.Route, error) {
	route, _, err := r.shadowRouteByOrderNumber(ctx, orderNumber)
	return route, err
}

func (r *dualWriteOrderRepository) shadowRouteByOrderNumber(ctx context.Context, orderNumber int64) (sharding.Route, bool, error) {
	route, err := r.shadow.RouteByOrderNumber(ctx, orderNumber)
	if err == nil {
		return route, false, nil
	}
	return sharding.Route{}, false, err
}

func (r *dualWriteOrderRepository) readRepo() OrderRepository {
	if r.mode == sharding.MigrationModeDualWriteReadNew {
		return r.shadow
	}
	return r.primary
}

func (r *dualWriteOrderRepository) readRepoForRoute(route sharding.Route) OrderRepository {
	switch route.Status {
	case sharding.RouteStatusPrimaryNew:
		return r.shadow
	case sharding.RouteStatusRollback:
		return r.primary
	default:
		return r.readRepo()
	}
}

func (r *dualWriteOrderRepository) transact(ctx context.Context, primaryRoute, shadowRoute sharding.Route, fn func(context.Context, OrderTx) error) error {
	plan, err := r.beginWritePlan(ctx, primaryRoute, shadowRoute)
	if err != nil {
		return err
	}

	tx := newDualWriteOrderTx(shadowRoute, plan.primaryTx, plan.shadowTx)

	if err := fn(ctx, tx); err != nil {
		if plan.shadowSQLTx != nil {
			_ = plan.shadowSQLTx.Rollback()
		}
		_ = plan.primarySQLTx.Rollback()
		return err
	}

	if shadowErr := tx.ShadowError(); shadowErr != nil {
		if err := plan.primarySQLTx.Commit(); err != nil {
			if plan.shadowSQLTx != nil {
				_ = plan.shadowSQLTx.Rollback()
			}
			return err
		}
		if plan.shadowSQLTx != nil {
			_ = plan.shadowSQLTx.Rollback()
		}
		return shadowErr
	}

	if err := plan.primarySQLTx.Commit(); err != nil {
		if plan.shadowSQLTx != nil {
			_ = plan.shadowSQLTx.Rollback()
		}
		return err
	}
	if plan.shadowSQLTx == nil {
		return nil
	}
	if err := plan.shadowSQLTx.Commit(); err != nil {
		return &ShadowWriteError{
			Route: plan.shadowRoute,
			Err:   err,
		}
	}

	return nil
}

type writeTransactionPlan struct {
	primaryTx    *singleOrderTx
	shadowTx     *singleOrderTx
	primarySQLTx *sql.Tx
	shadowSQLTx  *sql.Tx
	shadowRoute  sharding.Route
}

func (r *dualWriteOrderRepository) beginWritePlan(ctx context.Context, primaryRoute, shadowRoute sharding.Route) (*writeTransactionPlan, error) {
	switch shadowRoute.WriteMode {
	case sharding.WriteModeLegacyPrimary:
		primaryTx, primarySQLTx, err := r.beginLegacyWriteTx(ctx, legacyWriteRoute(primaryRoute, shadowRoute))
		if err != nil {
			return nil, err
		}
		return &writeTransactionPlan{
			primaryTx:    primaryTx,
			primarySQLTx: primarySQLTx,
		}, nil
	case sharding.WriteModeDualWrite:
		primaryTx, primarySQLTx, err := r.beginLegacyWriteTx(ctx, legacyWriteRoute(primaryRoute, shadowRoute))
		if err != nil {
			return nil, err
		}
		shadowTx, shadowSQLTx, err := r.beginShardWriteTx(ctx, shadowRoute)
		if err != nil {
			_ = primarySQLTx.Rollback()
			return nil, err
		}
		return &writeTransactionPlan{
			primaryTx:    primaryTx,
			shadowTx:     shadowTx,
			primarySQLTx: primarySQLTx,
			shadowSQLTx:  shadowSQLTx,
			shadowRoute:  shadowRoute,
		}, nil
	case sharding.WriteModeShardPrimary:
		primaryTx, primarySQLTx, err := r.beginShardWriteTx(ctx, shadowRoute)
		if err != nil {
			return nil, err
		}
		legacyShadowRoute := legacyWriteRoute(primaryRoute, shadowRoute)
		shadowTx, shadowSQLTx, err := r.beginLegacyWriteTx(ctx, legacyShadowRoute)
		if err != nil {
			_ = primarySQLTx.Rollback()
			return nil, err
		}
		return &writeTransactionPlan{
			primaryTx:    primaryTx,
			shadowTx:     shadowTx,
			primarySQLTx: primarySQLTx,
			shadowSQLTx:  shadowSQLTx,
			shadowRoute:  legacyShadowRoute,
		}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported write mode %s", sharding.ErrInvalidRouteEntry, shadowRoute.WriteMode)
	}
}

func (r *dualWriteOrderRepository) beginLegacyWriteTx(ctx context.Context, route sharding.Route) (*singleOrderTx, *sql.Tx, error) {
	legacyDB, err := r.primary.deps.LegacyConn.RawDB()
	if err != nil {
		return nil, nil, err
	}
	legacySQLTx, err := legacyDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}

	return newSingleOrderTx(
		route,
		sqlx.NewSessionFromTx(legacySQLTx),
		r.primary.deps.LegacyOrderModel,
		r.primary.deps.LegacyOrderTicketUserModel,
	), legacySQLTx, nil
}

func (r *dualWriteOrderRepository) beginShardWriteTx(ctx context.Context, route sharding.Route) (*singleOrderTx, *sql.Tx, error) {
	shadowStore, err := r.shadow.storeForRoute(route)
	if err != nil {
		return nil, nil, err
	}
	shadowDB, err := shadowStore.conn.RawDB()
	if err != nil {
		return nil, nil, err
	}
	shadowSQLTx, err := shadowDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}

	return newSingleOrderTx(
		route,
		sqlx.NewSessionFromTx(shadowSQLTx),
		shadowStore.orderModel,
		shadowStore.ticketModel,
	), shadowSQLTx, nil
}

func legacyWriteRoute(primaryRoute, shadowRoute sharding.Route) sharding.Route {
	route := shadowRoute
	if primaryRoute.LogicSlot != 0 {
		route.LogicSlot = primaryRoute.LogicSlot
	}
	route.DBKey = "legacy"
	route.TableSuffix = ""
	route.WriteMode = sharding.WriteModeLegacyPrimary
	return route
}
