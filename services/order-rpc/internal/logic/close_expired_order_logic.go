package logic

import (
	"context"
	"errors"
	"time"

	"livepass/jobs/order-close/taskdef"
	"livepass/pkg/delaytask"
	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"
	"livepass/services/order-rpc/repository"
	programrpc "livepass/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CloseExpiredOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseExpiredOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseExpiredOrderLogic {
	return &CloseExpiredOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CloseExpiredOrderLogic) CloseExpiredOrder(in *pb.CloseExpiredOrderReq) (*pb.BoolResp, error) {
	if in == nil || in.GetOrderNumber() <= 0 {
		return nil, mapOrderError(xerr.ErrInvalidParam)
	}

	unlock, err := lockOrderStatusGuard(l.ctx, l.svcCtx, in.GetOrderNumber())
	if err != nil {
		return nil, mapOrderError(err)
	}
	if unlock != nil {
		defer unlock()
	}

	order, changed, fromStatus, consumeAttempts, err := finalizeCloseTimeoutTask(l.ctx, l.svcCtx, in.GetOrderNumber(), time.Now())
	if err != nil {
		if errors.Is(err, xerr.ErrOrderStatusInvalid) {
			return &pb.BoolResp{Success: true}, nil
		}
		return nil, mapOrderError(err)
	}
	if !changed || order == nil {
		logDelayTaskConsumeTransition(l.Logger, taskdef.TaskTypeCloseTimeout, taskdef.TaskKey(in.GetOrderNumber()), fromStatus, delaytask.OutboxTaskStatusProcessed, consumeAttempts)
		return &pb.BoolResp{Success: true}, nil
	}

	logDelayTaskConsumeTransition(l.Logger, taskdef.TaskTypeCloseTimeout, taskdef.TaskKey(in.GetOrderNumber()), fromStatus, delaytask.OutboxTaskStatusProcessed, consumeAttempts)

	if err := syncClosedRushAttempt(l.ctx, l.svcCtx, order.ShowTimeId, order.OrderNumber, time.Now()); err != nil {
		l.Errorf("sync closed rush attempt failed, orderNumber=%d err=%v", order.OrderNumber, err)
	}

	if _, err := l.svcCtx.ProgramRpc.ReleaseSeatFreeze(l.ctx, &programrpc.ReleaseSeatFreezeReq{
		FreezeToken:   order.FreezeToken,
		ReleaseReason: "order_expired_close",
	}); err != nil {
		l.Errorf("release seat freeze after close timeout failed, orderNumber=%d freezeToken=%s err=%v", order.OrderNumber, order.FreezeToken, err)
	}

	return &pb.BoolResp{Success: true}, nil
}

func finalizeCloseTimeoutTask(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64, now time.Time) (*model.DOrder, bool, int64, int64, error) {
	if svcCtx == nil || svcCtx.OrderRepository == nil {
		return nil, false, 0, 0, xerr.ErrInternal
	}

	var (
		snapshot        *model.DOrder
		changed         bool
		fromStatus      int64
		consumeAttempts int64
	)
	err := svcCtx.OrderRepository.TransactByOrderNumber(ctx, orderNumber, func(txCtx context.Context, tx repository.OrderTx) error {
		order, err := tx.FindOrderByNumberForUpdate(txCtx, orderNumber)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				fromStatus, consumeAttempts, err = tx.MarkDelayTaskProcessed(txCtx, taskdef.TaskTypeCloseTimeout, taskdef.TaskKey(orderNumber), now)
				return err
			}
			return err
		}
		snapshot = cloneOrderSnapshot(order)

		if order.OrderStatus != orderStatusUnpaid {
			fromStatus, consumeAttempts, err = tx.MarkDelayTaskProcessed(txCtx, taskdef.TaskTypeCloseTimeout, taskdef.TaskKey(orderNumber), now)
			return err
		}
		if order.OrderExpireTime.After(now) {
			return nil
		}

		if err := tx.UpdateCancelStatus(txCtx, order.OrderNumber, now); err != nil {
			return err
		}
		if err := tx.DeleteGuardsByOrderNumber(txCtx, order.OrderNumber); err != nil {
			return err
		}
		fromStatus, consumeAttempts, err = tx.MarkDelayTaskProcessed(txCtx, taskdef.TaskTypeCloseTimeout, taskdef.TaskKey(orderNumber), now)
		if err != nil {
			return err
		}
		snapshot.OrderStatus = orderStatusCancelled
		snapshot.CancelOrderTime.Valid = true
		snapshot.CancelOrderTime.Time = now
		changed = true
		return nil
	})
	if err != nil {
		return nil, false, 0, 0, err
	}

	return snapshot, changed, fromStatus, consumeAttempts, nil
}
