package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	kafkaHandoffReasonTimeout = "KAFKA_HANDOFF_TIMEOUT"
	kafkaHandoffReasonError   = "KAFKA_HANDOFF_ERROR"
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
	if l.svcCtx == nil || l.svcCtx.PurchaseTokenCodec == nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}

	claims, err := l.svcCtx.PurchaseTokenCodec.Verify(in.GetPurchaseToken())
	if err != nil {
		return nil, mapOrderError(xerr.ErrInvalidParam)
	}
	if claims.UserID != in.GetUserId() {
		return nil, mapOrderError(xerr.ErrInvalidParam)
	}

	if l.svcCtx.AttemptStore == nil || l.svcCtx.OrderCreateProducer == nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}

	now := time.Now()
	admission, err := l.svcCtx.AttemptStore.Admit(l.ctx, rush.AdmitAttemptRequest{
		OrderNumber:      claims.OrderNumber,
		UserID:           claims.UserID,
		ProgramID:        claims.ProgramID,
		ShowTimeID:       claims.ShowTimeID,
		TicketCategoryID: claims.TicketCategoryID,
		ViewerIDs:        append([]int64(nil), claims.TicketUserIDs...),
		TicketCount:      claims.TicketCount,
		SaleWindowEndAt:  time.Unix(claims.SaleWindowEndAt, 0),
		TokenFingerprint: claims.TokenFingerprint,
		ShowEndAt:        time.Unix(claims.ShowEndAt, 0),
		Now:              now,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}
	if admission.Decision == rush.AdmitDecisionRejected {
		return nil, mapOrderError(mapAdmissionRejectCode(admission.RejectCode))
	}
	if admission.OrderNumber <= 0 {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}

	if admission.Decision == rush.AdmitDecisionReused {
		return &pb.CreateOrderResp{OrderNumber: admission.OrderNumber}, nil
	}

	event, err := buildAttemptCreateEvent(admission.OrderNumber, claims, now)
	if err != nil {
		return nil, mapOrderError(err)
	}
	body, err := event.Marshal()
	if err != nil {
		return nil, mapOrderError(err)
	}
	sendCtx, cancel := l.buildKafkaSendContext()
	defer cancel()
	if err := l.svcCtx.OrderCreateProducer.Send(sendCtx, event.PartitionKey(), body); err != nil {
		l.Errorf("handoff order create event failed, orderNumber=%d err=%v", admission.OrderNumber, err)
		return l.handleKafkaHandoffFailure(admission.OrderNumber, err, now)
	}

	return &pb.CreateOrderResp{OrderNumber: admission.OrderNumber}, nil
}

func mapAdmissionRejectCode(code int64) error {
	switch code {
	case rush.AdmitRejectUserInflightConflict, rush.AdmitRejectViewerInflightConflict:
		return xerr.ErrOrderOperateTooFrequent
	case rush.AdmitRejectQuotaExhausted:
		return xerr.ErrSeatInventoryInsufficient
	default:
		return xerr.ErrInternal
	}
}

func (l *CreateOrderLogic) buildKafkaSendContext() (context.Context, context.CancelFunc) {
	baseCtx := l.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	if l == nil || l.svcCtx == nil || l.svcCtx.Config.Kafka.ProducerTimeout <= 0 {
		return baseCtx, func() {}
	}
	if _, hasDeadline := baseCtx.Deadline(); hasDeadline {
		return baseCtx, func() {}
	}

	return context.WithTimeout(baseCtx, l.svcCtx.Config.Kafka.ProducerTimeout)
}

func (l *CreateOrderLogic) handleKafkaHandoffFailure(orderNumber int64, sendErr error, now time.Time) (*pb.CreateOrderResp, error) {
	record, err := l.svcCtx.AttemptStore.Get(l.ctx, orderNumber)
	if err != nil {
		l.Errorf("load rush attempt before fast fail failed, orderNumber=%d err=%v", orderNumber, err)
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}
	outcome, err := l.svcCtx.AttemptStore.FailBeforeProcessing(l.ctx, record, mapKafkaHandoffReason(sendErr), now)
	if err != nil {
		l.Errorf("fail before processing for order create failed, orderNumber=%d err=%v", orderNumber, err)
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}

	switch outcome {
	case rush.AttemptTransitioned, rush.AttemptAlreadyFailed:
		return nil, mapOrderError(mapKafkaHandoffErr(sendErr))
	case rush.AttemptLostOwnership, rush.AttemptAlreadySucceeded:
		return &pb.CreateOrderResp{OrderNumber: orderNumber}, nil
	default:
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}
}

func mapKafkaHandoffReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return kafkaHandoffReasonTimeout
	}
	return kafkaHandoffReasonError
}

func mapKafkaHandoffErr(err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return xerr.ErrOrderSubmitTooFrequent
	}
	return xerr.ErrOrderOperateTooFrequent
}
