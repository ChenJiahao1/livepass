package logic

import (
	"context"
	"errors"
	"strconv"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/repeatguard"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderLogic {
	return &CreateOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateOrderLogic) CreateOrder(in *pb.CreateOrderReq) (*pb.CreateOrderResp, error) {
	if err := validateCreateOrderReq(in); err != nil {
		return nil, err
	}

	if l.svcCtx.RepeatGuard != nil {
		unlock, err := l.svcCtx.RepeatGuard.Lock(l.ctx, repeatguard.OrderCreateKey(in.GetUserId(), in.GetProgramId()))
		if err != nil {
			if errors.Is(err, repeatguard.ErrLocked) {
				return nil, mapOrderError(xerr.ErrOrderSubmitTooFrequent)
			}
			return nil, mapOrderError(err)
		}
		if unlock != nil {
			defer unlock()
		}
	}

	preorder, err := l.svcCtx.ProgramRpc.GetProgramPreorder(l.ctx, &programrpc.GetProgramDetailReq{
		Id: in.GetProgramId(),
	})
	if err != nil {
		return nil, err
	}

	userResp, err := l.svcCtx.UserRpc.GetUserAndTicketUserList(l.ctx, &userrpc.GetUserAndTicketUserListReq{
		UserId: in.GetUserId(),
	})
	if err != nil {
		return nil, err
	}
	if err := validateRequestedTicketUsers(userResp, in.GetUserId(), in.GetTicketUserIds()); err != nil {
		return nil, mapOrderError(err)
	}

	if preorder.GetPerOrderLimitPurchaseCount() > 0 && int64(len(in.GetTicketUserIds())) > preorder.GetPerOrderLimitPurchaseCount() {
		return nil, mapOrderError(xerr.ErrOrderPurchaseLimitExceeded)
	}

	activeTicketCount, err := l.svcCtx.DOrderModel.CountByUserProgramAndStatus(l.ctx, in.GetUserId(), in.GetProgramId(), orderStatusUnpaid)
	if err != nil {
		return nil, err
	}
	if preorder.GetPerAccountLimitPurchaseCount() > 0 && activeTicketCount+int64(len(in.GetTicketUserIds())) > preorder.GetPerAccountLimitPurchaseCount() {
		return nil, mapOrderError(xerr.ErrOrderPurchaseLimitExceeded)
	}

	closeAfter := l.svcCtx.Config.Order.CloseAfter
	if closeAfter <= 0 {
		closeAfter = 15 * time.Minute
	}
	freezeResp, err := l.svcCtx.ProgramRpc.AutoAssignAndFreezeSeats(l.ctx, &programrpc.AutoAssignAndFreezeSeatsReq{
		ProgramId:        in.GetProgramId(),
		TicketCategoryId: in.GetTicketCategoryId(),
		Count:            int64(len(in.GetTicketUserIds())),
		RequestNo:        buildFreezeRequestNo(),
		FreezeSeconds:    int64(closeAfter / time.Second),
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	orderEvent, err := buildOrderCreateEvent(in, preorder, userResp, freezeResp, now, closeAfter)
	if err != nil {
		compensateOrderCreateSendFailure(l.ctx, l.svcCtx, freezeResp.GetFreezeToken())
		return nil, mapOrderError(err)
	}

	if l.svcCtx.OrderCreateProducer == nil {
		compensateOrderCreateSendFailure(l.ctx, l.svcCtx, freezeResp.GetFreezeToken())
		return nil, mapOrderError(xerr.ErrInternal)
	}

	body, err := orderEvent.Marshal()
	if err != nil {
		compensateOrderCreateSendFailure(l.ctx, l.svcCtx, freezeResp.GetFreezeToken())
		return nil, mapOrderError(xerr.ErrInternal)
	}
	if err := l.svcCtx.OrderCreateProducer.Send(l.ctx, strconv.FormatInt(orderEvent.OrderNumber, 10), body); err != nil {
		l.Errorf("send order create event failed, orderNumber=%d err=%v", orderEvent.OrderNumber, err)
		compensateOrderCreateSendFailure(l.ctx, l.svcCtx, freezeResp.GetFreezeToken())
		return nil, mapOrderError(xerr.ErrInternal)
	}

	return &pb.CreateOrderResp{OrderNumber: orderEvent.OrderNumber}, nil
}
