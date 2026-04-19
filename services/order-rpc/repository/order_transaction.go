package repository

import (
	"context"
	"time"

	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type singleOrderTx struct {
	route            sharding.Route
	session          sqlx.Session
	orderModel       model.DOrderModel
	ticketModel      model.DOrderTicketUserModel
	userGuardModel   model.DOrderUserGuardModel
	viewerGuardModel model.DOrderViewerGuardModel
	seatGuardModel   model.DOrderSeatGuardModel
	delayTaskModel   model.DDelayTaskOutboxModel
}

func newSingleOrderTx(route sharding.Route, session sqlx.Session, orderModel model.DOrderModel,
	ticketModel model.DOrderTicketUserModel, userGuardModel model.DOrderUserGuardModel,
	viewerGuardModel model.DOrderViewerGuardModel, seatGuardModel model.DOrderSeatGuardModel,
	delayTaskModel model.DDelayTaskOutboxModel) *singleOrderTx {
	return &singleOrderTx{
		route:            route,
		session:          session,
		orderModel:       orderModel,
		ticketModel:      ticketModel,
		userGuardModel:   userGuardModel,
		viewerGuardModel: viewerGuardModel,
		seatGuardModel:   seatGuardModel,
		delayTaskModel:   delayTaskModel,
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

func (t *singleOrderTx) InsertUserGuard(ctx context.Context, guard *model.DOrderUserGuard) error {
	if guard == nil {
		return nil
	}
	_, err := t.userGuardModel.InsertWithSession(ctx, t.session, guard)
	return err
}

func (t *singleOrderTx) InsertViewerGuards(ctx context.Context, guards []*model.DOrderViewerGuard) error {
	return t.viewerGuardModel.InsertBatch(ctx, t.session, guards)
}

func (t *singleOrderTx) InsertSeatGuards(ctx context.Context, guards []*model.DOrderSeatGuard) error {
	return t.seatGuardModel.InsertBatch(ctx, t.session, guards)
}

func (t *singleOrderTx) InsertDelayTasks(ctx context.Context, rows []*model.DDelayTaskOutbox) error {
	return t.delayTaskModel.InsertBatch(ctx, t.session, rows)
}

func (t *singleOrderTx) DeleteGuardsByOrderNumber(ctx context.Context, orderNumber int64) error {
	if err := t.userGuardModel.DeleteByOrderNumber(ctx, t.session, orderNumber); err != nil {
		return err
	}
	if err := t.viewerGuardModel.DeleteByOrderNumber(ctx, t.session, orderNumber); err != nil {
		return err
	}
	if err := t.seatGuardModel.DeleteByOrderNumber(ctx, t.session, orderNumber); err != nil {
		return err
	}
	return nil
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
