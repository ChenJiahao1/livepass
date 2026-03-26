package repository

import (
	"context"
	"fmt"
	"time"

	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type singleOrderTx struct {
	route            sharding.Route
	session          sqlx.Session
	orderModel       model.DOrderModel
	ticketModel      model.DOrderTicketUserModel
	legacyRouteModel model.DOrderRouteLegacyModel
}

func newSingleOrderTx(route sharding.Route, session sqlx.Session, orderModel model.DOrderModel,
	ticketModel model.DOrderTicketUserModel, legacyRouteModel model.DOrderRouteLegacyModel) *singleOrderTx {
	return &singleOrderTx{
		route:            route,
		session:          session,
		orderModel:       orderModel,
		ticketModel:      ticketModel,
		legacyRouteModel: legacyRouteModel,
	}
}

func (t *singleOrderTx) Route() sharding.Route {
	return t.route
}

func (t *singleOrderTx) InsertOrder(ctx context.Context, order *model.DOrder) error {
	_, err := t.orderModel.InsertWithSession(ctx, t.session, order)
	return err
}

func (t *singleOrderTx) InsertOrderTickets(ctx context.Context, tickets []*model.DOrderTicketUser) error {
	return t.ticketModel.InsertBatch(ctx, t.session, tickets)
}

func (t *singleOrderTx) InsertLegacyRoute(ctx context.Context, legacyRoute *model.DOrderRouteLegacy) error {
	if t.legacyRouteModel == nil || legacyRoute == nil {
		return nil
	}
	_, err := t.legacyRouteModel.InsertWithSession(ctx, t.session, legacyRoute)
	return err
}

func (t *singleOrderTx) FindOrderByNumberForUpdate(ctx context.Context, orderNumber int64) (*model.DOrder, error) {
	return t.orderModel.FindOneByOrderNumberForUpdate(ctx, t.session, orderNumber)
}

func (t *singleOrderTx) UpdateCancelStatus(ctx context.Context, orderNumber int64, cancelTime time.Time) error {
	if err := t.orderModel.UpdateCancelStatus(ctx, t.session, orderNumber, cancelTime); err != nil {
		return err
	}
	if err := t.ticketModel.UpdateCancelStatusByOrderNumber(ctx, t.session, orderNumber, cancelTime); err != nil {
		return err
	}
	return nil
}

func (t *singleOrderTx) UpdatePayStatus(ctx context.Context, orderNumber int64, payTime time.Time) error {
	if err := t.orderModel.UpdatePayStatus(ctx, t.session, orderNumber, payTime); err != nil {
		return err
	}
	if err := t.ticketModel.UpdatePayStatusByOrderNumber(ctx, t.session, orderNumber, payTime); err != nil {
		return err
	}
	return nil
}

func (t *singleOrderTx) UpdateRefundStatus(ctx context.Context, orderNumber int64, refundTime time.Time) error {
	if err := t.orderModel.UpdateRefundStatus(ctx, t.session, orderNumber, refundTime); err != nil {
		return err
	}
	if err := t.ticketModel.UpdateRefundStatusByOrderNumber(ctx, t.session, orderNumber, refundTime); err != nil {
		return err
	}
	return nil
}

type ShadowWriteError struct {
	Route sharding.Route
	Err   error
}

func (e *ShadowWriteError) Error() string {
	return fmt.Sprintf("shadow write failed for db=%s table=%s: %v", e.Route.DBKey, e.Route.TableSuffix, e.Err)
}

func (e *ShadowWriteError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type dualWriteOrderTx struct {
	route     sharding.Route
	primary   *singleOrderTx
	shadow    *singleOrderTx
	shadowErr error
}

func newDualWriteOrderTx(route sharding.Route, primary, shadow *singleOrderTx) *dualWriteOrderTx {
	return &dualWriteOrderTx{
		route:   route,
		primary: primary,
		shadow:  shadow,
	}
}

func (t *dualWriteOrderTx) Route() sharding.Route {
	return t.route
}

func (t *dualWriteOrderTx) InsertOrder(ctx context.Context, order *model.DOrder) error {
	if err := t.primary.InsertOrder(ctx, order); err != nil {
		return err
	}
	t.captureShadowError(func() error {
		return t.shadow.InsertOrder(ctx, order)
	})
	return nil
}

func (t *dualWriteOrderTx) InsertOrderTickets(ctx context.Context, tickets []*model.DOrderTicketUser) error {
	if err := t.primary.InsertOrderTickets(ctx, tickets); err != nil {
		return err
	}
	t.captureShadowError(func() error {
		return t.shadow.InsertOrderTickets(ctx, tickets)
	})
	return nil
}

func (t *dualWriteOrderTx) InsertLegacyRoute(ctx context.Context, legacyRoute *model.DOrderRouteLegacy) error {
	return t.primary.InsertLegacyRoute(ctx, legacyRoute)
}

func (t *dualWriteOrderTx) FindOrderByNumberForUpdate(ctx context.Context, orderNumber int64) (*model.DOrder, error) {
	order, err := t.primary.FindOrderByNumberForUpdate(ctx, orderNumber)
	if err != nil {
		return nil, err
	}
	t.captureShadowError(func() error {
		_, err := t.shadow.FindOrderByNumberForUpdate(ctx, orderNumber)
		return err
	})
	return order, nil
}

func (t *dualWriteOrderTx) UpdateCancelStatus(ctx context.Context, orderNumber int64, cancelTime time.Time) error {
	if err := t.primary.UpdateCancelStatus(ctx, orderNumber, cancelTime); err != nil {
		return err
	}
	t.captureShadowError(func() error {
		return t.shadow.UpdateCancelStatus(ctx, orderNumber, cancelTime)
	})
	return nil
}

func (t *dualWriteOrderTx) UpdatePayStatus(ctx context.Context, orderNumber int64, payTime time.Time) error {
	if err := t.primary.UpdatePayStatus(ctx, orderNumber, payTime); err != nil {
		return err
	}
	t.captureShadowError(func() error {
		return t.shadow.UpdatePayStatus(ctx, orderNumber, payTime)
	})
	return nil
}

func (t *dualWriteOrderTx) UpdateRefundStatus(ctx context.Context, orderNumber int64, refundTime time.Time) error {
	if err := t.primary.UpdateRefundStatus(ctx, orderNumber, refundTime); err != nil {
		return err
	}
	t.captureShadowError(func() error {
		return t.shadow.UpdateRefundStatus(ctx, orderNumber, refundTime)
	})
	return nil
}

func (t *dualWriteOrderTx) ShadowError() error {
	return t.shadowErr
}

func (t *dualWriteOrderTx) captureShadowError(fn func() error) {
	if t.shadow == nil || t.shadowErr != nil {
		return
	}
	if err := fn(); err != nil {
		t.shadowErr = &ShadowWriteError{
			Route: t.shadow.route,
			Err:   err,
		}
	}
}
