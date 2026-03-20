package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	bundle, err := buildOrderSnapshotBundle(in, preorder, userResp, freezeResp, now, closeAfter)
	if err != nil {
		compensateOrderFreezeRelease(l.ctx, l.svcCtx, freezeResp.GetFreezeToken(), "order_create_failed")
		return nil, mapOrderError(err)
	}

	err = l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := l.svcCtx.DOrderModel.InsertWithSession(ctx, session, bundle.order); err != nil {
			return err
		}
		if err := l.svcCtx.DOrderTicketUserModel.InsertBatch(ctx, session, bundle.orderTickets); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		compensateOrderFreezeRelease(l.ctx, l.svcCtx, freezeResp.GetFreezeToken(), "order_create_failed")
		if errors.Is(err, xerr.ErrOrderTicketUserInvalid) || errors.Is(err, xerr.ErrProgramTicketCategoryNotFound) || status.Code(err) == codes.FailedPrecondition {
			return nil, mapOrderError(err)
		}
		return nil, err
	}

	return &pb.CreateOrderResp{OrderNumber: bundle.order.OrderNumber}, nil
}
