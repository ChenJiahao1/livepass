package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

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
	if in.GetProgramId() <= 0 || in.GetOrderShowTime() == "" || in.GetOrderAmount() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	showTime, err := time.ParseInLocation(programDateTimeLayout, in.GetOrderShowTime(), time.Local)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	program, err := l.svcCtx.DProgramModel.FindOne(l.ctx, in.GetProgramId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}

	switch program.PermitRefund {
	case 0:
		return &pb.EvaluateRefundRuleResp{
			AllowRefund:  false,
			RejectReason: "program does not permit refund",
		}, nil
	case 2:
		return &pb.EvaluateRefundRuleResp{
			AllowRefund:   true,
			RefundPercent: 100,
			RefundAmount:  in.GetOrderAmount(),
		}, nil
	case 1:
		result, err := evaluateRefundRule(nullStringValue(program.RefundRuleJson), showTime, time.Now(), in.GetOrderAmount())
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		return &pb.EvaluateRefundRuleResp{
			AllowRefund:   result.AllowRefund,
			RefundPercent: result.RefundPercent,
			RefundAmount:  result.RefundAmount,
			RejectReason:  result.RejectReason,
		}, nil
	default:
		return &pb.EvaluateRefundRuleResp{
			AllowRefund:  false,
			RejectReason: "program does not permit refund",
		}, nil
	}
}
