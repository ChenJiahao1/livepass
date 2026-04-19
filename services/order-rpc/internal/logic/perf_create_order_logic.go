package logic

import (
	"context"
	"time"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PerfCreateOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPerfCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PerfCreateOrderLogic {
	return &PerfCreateOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PerfCreateOrderLogic) PerfCreateOrder(in *pb.CreateOrderReq) (*pb.PerfCreateOrderResp, error) {
	perfState := createOrderPerfState{
		userID: in.GetUserId(),
	}
	defer func() {
		if perfState.orderNumber > 0 || perfState.result != "" || perfState.reasonCode != "" || perfState.grpcCode != "" {
			logCreateOrderPerfState(l.ctx, perfState)
		}
	}()

	if err := validateCreateOrderReq(in); err != nil {
		perfState.grpcCode = grpcCodeOf(err)
		perfState.reasonCode = "request_invalid"
		perfState.result = "failed"
		return nil, err
	}
	if l.svcCtx == nil || l.svcCtx.PurchaseTokenCodec == nil {
		orderErr := status.Error(codes.Internal, xerr.ErrInternal.Error())
		perfState.grpcCode = grpcCodeOf(orderErr)
		perfState.reasonCode = "rush_dependencies_missing"
		perfState.result = "failed"
		return nil, orderErr
	}

	verifyStartedAt := time.Now()
	claims, err := l.svcCtx.PurchaseTokenCodec.Verify(in.GetPurchaseToken())
	perfState.purchaseTokenVerifyCost = time.Since(verifyStartedAt)
	if err != nil {
		orderErr := mapOrderError(xerr.ErrInvalidParam)
		perfState.grpcCode = grpcCodeOf(orderErr)
		perfState.reasonCode = "purchase_token_verify_failed"
		perfState.result = "failed"
		return nil, orderErr
	}
	perfState.orderNumber = claims.OrderNumber
	if claims.UserID != in.GetUserId() {
		orderErr := mapOrderError(xerr.ErrInvalidParam)
		perfState.grpcCode = grpcCodeOf(orderErr)
		perfState.reasonCode = "purchase_token_user_mismatch"
		perfState.result = "failed"
		return nil, orderErr
	}

	if l.svcCtx.AttemptStore == nil || l.svcCtx.OrderCreateProducer == nil {
		orderErr := status.Error(codes.Internal, xerr.ErrInternal.Error())
		perfState.grpcCode = grpcCodeOf(orderErr)
		perfState.reasonCode = "rush_dependencies_missing"
		perfState.result = "failed"
		return nil, orderErr
	}

	now := time.Now()
	admitStartedAt := time.Now()
	admission, err := l.svcCtx.AttemptStore.Admit(l.ctx, rush.AdmitAttemptRequest{
		OrderNumber:      claims.OrderNumber,
		UserID:           claims.UserID,
		ProgramID:        claims.ProgramID,
		ShowTimeID:       claims.ShowTimeID,
		TicketCategoryID: claims.TicketCategoryID,
		ViewerIDs:        append([]int64(nil), claims.TicketUserIDs...),
		TicketCount:      claims.TicketCount,
		SaleWindowEndAt:  time.Unix(claims.SaleWindowEndAt, 0),
		ShowEndAt:        time.Unix(claims.ShowEndAt, 0),
		Now:              now,
	})
	perfState.redisAdmitCost = time.Since(admitStartedAt)
	if err != nil {
		if isUnknownAdmissionResult(err) {
			l.Errorf("admit attempt unknown result, orderNumber=%d err=%v", claims.OrderNumber, err)
			perfState.reasonCode = "admission_unknown_result"
			perfState.result = "assumed_success"
			return &pb.PerfCreateOrderResp{
				OrderNumber:             claims.OrderNumber,
				ShowTimeId:              claims.ShowTimeID,
				Result:                  perfState.result,
				ReasonCode:              perfState.reasonCode,
				PurchaseTokenVerifyMs:   perfState.purchaseTokenVerifyCost.Milliseconds(),
				RedisAdmitMs:            perfState.redisAdmitCost.Milliseconds(),
				AsyncDispatchScheduleMs: perfState.asyncDispatchCost.Milliseconds(),
			}, nil
		}
		orderErr := status.Error(codes.Internal, xerr.ErrInternal.Error())
		perfState.grpcCode = grpcCodeOf(orderErr)
		perfState.reasonCode = "admission_internal_error"
		perfState.result = "failed"
		return nil, orderErr
	}
	if admission.Decision == rush.AdmitDecisionRejected {
		orderErr := mapOrderError(mapAdmissionRejectCode(admission.RejectCode))
		perfState.rejectCode = admission.RejectCode
		perfState.orderNumber = admission.OrderNumber
		perfState.grpcCode = grpcCodeOf(orderErr)
		perfState.reasonCode = "admission_rejected"
		perfState.result = "rejected"
		return nil, orderErr
	}
	if admission.OrderNumber <= 0 {
		orderErr := status.Error(codes.Internal, xerr.ErrInternal.Error())
		perfState.grpcCode = grpcCodeOf(orderErr)
		perfState.reasonCode = "admission_invalid_order_number"
		perfState.result = "failed"
		return nil, orderErr
	}

	perfState.orderNumber = admission.OrderNumber
	if admission.Decision == rush.AdmitDecisionReused {
		perfState.reasonCode = "admission_reused"
		perfState.result = "reused"
		return &pb.PerfCreateOrderResp{
			OrderNumber:             admission.OrderNumber,
			ShowTimeId:              claims.ShowTimeID,
			Result:                  perfState.result,
			ReasonCode:              perfState.reasonCode,
			RejectCode:              perfState.rejectCode,
			GrpcCode:                perfState.grpcCode,
			PurchaseTokenVerifyMs:   perfState.purchaseTokenVerifyCost.Milliseconds(),
			RedisAdmitMs:            perfState.redisAdmitCost.Milliseconds(),
			AsyncDispatchScheduleMs: perfState.asyncDispatchCost.Milliseconds(),
		}, nil
	}

	dispatchStartedAt := time.Now()
	event, err := buildAttemptCreateEvent(admission.OrderNumber, claims, now)
	if err != nil {
		orderErr := mapOrderError(err)
		perfState.asyncDispatchCost = time.Since(dispatchStartedAt)
		perfState.grpcCode = grpcCodeOf(orderErr)
		perfState.reasonCode = "build_attempt_create_event_failed"
		perfState.result = "failed"
		return nil, orderErr
	}
	body, err := event.Marshal()
	if err != nil {
		orderErr := mapOrderError(err)
		perfState.asyncDispatchCost = time.Since(dispatchStartedAt)
		perfState.grpcCode = grpcCodeOf(orderErr)
		perfState.reasonCode = "marshal_attempt_create_event_failed"
		perfState.result = "failed"
		return nil, orderErr
	}
	dispatchOrderCreateEventAsync(l.svcCtx, l.Logger, asyncOrderCreateEvent{
		orderNumber: admission.OrderNumber,
		showTimeID:  claims.ShowTimeID,
		key:         event.PartitionKey(),
		body:        body,
		reasonAt:    now,
	})
	perfState.asyncDispatchCost = time.Since(dispatchStartedAt)
	perfState.reasonCode = "async_dispatch_scheduled"
	perfState.result = "success"

	return &pb.PerfCreateOrderResp{
		OrderNumber:             admission.OrderNumber,
		ShowTimeId:              claims.ShowTimeID,
		Result:                  perfState.result,
		ReasonCode:              perfState.reasonCode,
		RejectCode:              perfState.rejectCode,
		GrpcCode:                perfState.grpcCode,
		PurchaseTokenVerifyMs:   perfState.purchaseTokenVerifyCost.Milliseconds(),
		RedisAdmitMs:            perfState.redisAdmitCost.Milliseconds(),
		AsyncDispatchScheduleMs: perfState.asyncDispatchCost.Milliseconds(),
	}, nil
}
