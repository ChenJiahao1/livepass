package logic

import (
	"context"
	"errors"
	"net"
	"time"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	asyncKafkaSendReasonTimeout = "KAFKA_ASYNC_SEND_TIMEOUT"
	asyncKafkaSendReasonError   = "KAFKA_ASYNC_SEND_ERROR"
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
		ShowEndAt:        time.Unix(claims.ShowEndAt, 0),
		Now:              now,
	})
	if err != nil {
		if isUnknownAdmissionResult(err) {
			l.Errorf("admit attempt unknown result, orderNumber=%d err=%v", claims.OrderNumber, err)
			return &pb.CreateOrderResp{OrderNumber: claims.OrderNumber, ShowTimeId: claims.ShowTimeID}, nil
		}
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}
	if admission.Decision == rush.AdmitDecisionRejected {
		return nil, mapOrderError(mapAdmissionRejectCode(admission.RejectCode))
	}
	if admission.OrderNumber <= 0 {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}

	if admission.Decision == rush.AdmitDecisionReused {
		return &pb.CreateOrderResp{OrderNumber: admission.OrderNumber, ShowTimeId: claims.ShowTimeID}, nil
	}

	event, err := buildAttemptCreateEvent(admission.OrderNumber, claims, now)
	if err != nil {
		return nil, mapOrderError(err)
	}
	body, err := event.Marshal()
	if err != nil {
		return nil, mapOrderError(err)
	}
	dispatchOrderCreateEventAsync(l.svcCtx, l.Logger, asyncOrderCreateEvent{
		orderNumber: admission.OrderNumber,
		showTimeID:  claims.ShowTimeID,
		key:         event.PartitionKey(),
		body:        body,
		reasonAt:    now,
	})

	return &pb.CreateOrderResp{OrderNumber: admission.OrderNumber, ShowTimeId: claims.ShowTimeID}, nil
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

func mapAsyncKafkaSendReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return asyncKafkaSendReasonTimeout
	}
	return asyncKafkaSendReasonError
}

func isUnknownAdmissionResult(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
