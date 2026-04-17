package logic

import (
	"context"
	"errors"
	"time"

	"livepass/pkg/xerr"
	"livepass/services/program-rpc/internal/model"
	"livepass/services/program-rpc/internal/svc"
	"livepass/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EvaluateRefundRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEvaluateRefundRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EvaluateRefundRuleLogic {
	return &EvaluateRefundRuleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *EvaluateRefundRuleLogic) EvaluateRefundRule(in *pb.EvaluateRefundRuleReq) (*pb.EvaluateRefundRuleResp, error) {
	if in.GetShowTimeId() <= 0 || in.GetOrderAmount() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	showTime, err := l.svcCtx.DProgramShowTimeModel.FindOne(l.ctx, in.GetShowTimeId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}

	program, err := l.svcCtx.DProgramModel.FindOne(l.ctx, showTime.ProgramId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}
	if isRefundBlockedDuringRushSale(program, time.Time{}) {
		return &pb.EvaluateRefundRuleResp{
			AllowRefund:  false,
			RejectReason: rushSaleRefundBlockedReason,
		}, nil
	}

	switch program.PermitRefund {
	case 0:
		return &pb.EvaluateRefundRuleResp{
			AllowRefund:  false,
			RejectReason: programRefundDisabledReason(program),
		}, nil
	case 2:
		return &pb.EvaluateRefundRuleResp{
			AllowRefund:   true,
			RefundPercent: 100,
			RefundAmount:  in.GetOrderAmount(),
		}, nil
	case 1:
		result, err := evaluateRefundRule(nullStringValue(program.RefundRuleJson), showTime.ShowTime, time.Now(), in.GetOrderAmount())
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		rejectReason := result.RejectReason
		if result.NoMatch {
			rejectReason = programRefundNoMatchReason(program, result.RejectReason)
		}

		return &pb.EvaluateRefundRuleResp{
			AllowRefund:   result.AllowRefund,
			RefundPercent: result.RefundPercent,
			RefundAmount:  result.RefundAmount,
			RejectReason:  rejectReason,
		}, nil
	default:
		return &pb.EvaluateRefundRuleResp{
			AllowRefund:  false,
			RejectReason: programRefundDisabledReason(program),
		}, nil
	}
}
