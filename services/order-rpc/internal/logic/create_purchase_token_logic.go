package logic

import (
	"context"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreatePurchaseTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePurchaseTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePurchaseTokenLogic {
	return &CreatePurchaseTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreatePurchaseTokenLogic) CreatePurchaseToken(in *pb.CreatePurchaseTokenReq) (*pb.CreatePurchaseTokenResp, error) {
	if err := validateCreatePurchaseTokenReq(in); err != nil {
		return nil, err
	}
	if l.svcCtx == nil || l.svcCtx.PurchaseTokenCodec == nil || l.svcCtx.ProgramRpc == nil || l.svcCtx.UserRpc == nil || l.svcCtx.OrderRepository == nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}

	preorder, err := l.svcCtx.ProgramRpc.GetProgramPreorder(l.ctx, &programrpc.GetProgramPreorderReq{
		ShowTimeId: in.GetShowTimeId(),
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
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if _, ok := findPreorderTicketCategory(preorder.GetTicketCategoryVoList(), in.GetTicketCategoryId()); !ok {
		return nil, status.Error(codes.NotFound, xerr.ErrProgramTicketCategoryNotFound.Error())
	}

	ticketCount := int64(len(in.GetTicketUserIds()))
	if preorder.GetPerOrderLimitPurchaseCount() > 0 && ticketCount > preorder.GetPerOrderLimitPurchaseCount() {
		return nil, mapOrderError(xerr.ErrOrderPurchaseLimitExceeded)
	}

	if preorder.GetPerAccountLimitPurchaseCount() > 0 {
		activeCount, err := l.svcCtx.OrderRepository.CountActiveTicketsByUserProgram(l.ctx, in.GetUserId(), in.GetShowTimeId())
		if err != nil {
			return nil, mapOrderError(err)
		}
		if activeCount+ticketCount > preorder.GetPerAccountLimitPurchaseCount() {
			return nil, mapOrderError(xerr.ErrOrderPurchaseLimitExceeded)
		}
	}

	orderNumber := allocateRushContractOrderNumber(in.GetUserId())
	token, err := l.svcCtx.PurchaseTokenCodec.Issue(rush.PurchaseTokenClaims{
		OrderNumber:      orderNumber,
		UserID:           in.GetUserId(),
		ProgramID:        preorder.GetProgramId(),
		TicketCategoryID: in.GetTicketCategoryId(),
		TicketUserIDs:    append([]int64(nil), in.GetTicketUserIds()...),
		TicketCount:      ticketCount,
		DistributionMode: in.GetDistributionMode(),
		TakeTicketMode:   in.GetTakeTicketMode(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}

	return &pb.CreatePurchaseTokenResp{PurchaseToken: token}, nil
}

func validateCreatePurchaseTokenReq(in *pb.CreatePurchaseTokenReq) error {
	if in == nil || in.GetUserId() <= 0 || in.GetShowTimeId() <= 0 || in.GetTicketCategoryId() <= 0 || len(in.GetTicketUserIds()) == 0 {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	for _, ticketUserID := range in.GetTicketUserIds() {
		if ticketUserID <= 0 {
			return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
		}
	}

	return nil
}
