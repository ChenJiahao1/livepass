package logic

import (
	"context"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	programrpc "damai-go/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PrimeAdmissionQuotaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPrimeAdmissionQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PrimeAdmissionQuotaLogic {
	return &PrimeAdmissionQuotaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PrimeAdmissionQuotaLogic) PrimeAdmissionQuota(in *pb.PrimeAdmissionQuotaReq) (*pb.BoolResp, error) {
	if in == nil || in.GetShowTimeId() <= 0 {
		return nil, mapOrderError(xerr.ErrInvalidParam)
	}
	if err := PrimeAdmissionQuota(l.ctx, l.svcCtx, in.GetShowTimeId()); err != nil {
		return nil, mapOrderError(err)
	}

	return &pb.BoolResp{Success: true}, nil
}

func PrimeAdmissionQuota(ctx context.Context, svcCtx *svc.ServiceContext, showTimeID int64) error {
	if svcCtx == nil || svcCtx.AttemptStore == nil || svcCtx.ProgramRpc == nil {
		return xerr.ErrInternal
	}
	if showTimeID <= 0 {
		return xerr.ErrInvalidParam
	}

	preorder, err := svcCtx.ProgramRpc.GetProgramPreorder(ctx, &programrpc.GetProgramPreorderReq{
		ShowTimeId: showTimeID,
	})
	if err != nil {
		return err
	}

	resolvedShowTimeID := preorder.GetShowTimeId()
	if resolvedShowTimeID <= 0 {
		resolvedShowTimeID = showTimeID
	}
	for _, ticketCategory := range preorder.GetTicketCategoryVoList() {
		if ticketCategory == nil || ticketCategory.GetId() <= 0 {
			continue
		}
		if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, resolvedShowTimeID, ticketCategory.GetId(), ticketCategory.GetAdmissionQuota()); err != nil {
			return err
		}
	}

	return nil
}
