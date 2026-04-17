package logic

import (
	"context"
	"time"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"
	programrpc "livepass/services/program-rpc/programrpc"
	userrpc "livepass/services/user-rpc/userrpc"

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
		activeCount, err := l.svcCtx.OrderRepository.CountActiveTicketsByUserShowTime(l.ctx, in.GetUserId(), in.GetShowTimeId())
		if err != nil {
			return nil, mapOrderError(err)
		}
		if activeCount+ticketCount > preorder.GetPerAccountLimitPurchaseCount() {
			return nil, mapOrderError(xerr.ErrOrderPurchaseLimitExceeded)
		}
	}

	orderNumber := allocateRushContractOrderNumber(in.GetUserId())
	saleWindowEndAt, err := parsePurchaseTokenTime(preorder.GetRushSaleEndTime(), preorder.GetShowTime())
	if err != nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}
	showEndAt, err := parsePurchaseTokenTime(preorder.GetShowTime(), preorder.GetRushSaleEndTime())
	if err != nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}
	showTimeID := preorder.GetShowTimeId()
	if showTimeID <= 0 {
		showTimeID = in.GetShowTimeId()
	}
	token, err := l.svcCtx.PurchaseTokenCodec.Issue(rush.PurchaseTokenClaims{
		OrderNumber:      orderNumber,
		UserID:           in.GetUserId(),
		ProgramID:        preorder.GetProgramId(),
		ShowTimeID:       showTimeID,
		TicketCategoryID: in.GetTicketCategoryId(),
		TicketUserIDs:    append([]int64(nil), in.GetTicketUserIds()...),
		TicketCount:      ticketCount,
		SaleWindowEndAt:  saleWindowEndAt.Unix(),
		ShowEndAt:        showEndAt.Unix(),
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

func parsePurchaseTokenTime(primary, fallback string) (time.Time, error) {
	if primary != "" {
		return parseOrderTime(primary)
	}
	if fallback != "" {
		return parseOrderTime(fallback)
	}
	return time.Time{}, xerr.ErrInvalidParam
}
