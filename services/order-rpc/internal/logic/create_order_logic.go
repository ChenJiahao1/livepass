package logic

import (
	"context"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
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
		Generation:       claims.Generation,
		SaleWindowEndAt:  time.Unix(claims.SaleWindowEndAt, 0),
		TokenFingerprint: claims.TokenFingerprint,
		CommitCutoffAt:   now.Add(l.svcCtx.Config.RushOrder.CommitCutoff),
		UserDeadlineAt:   now.Add(l.svcCtx.Config.RushOrder.UserDeadline),
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

	attempt, err := l.svcCtx.AttemptStore.Get(l.ctx, admission.OrderNumber)
	if err != nil {
		return nil, mapOrderError(err)
	}

	if l.svcCtx.AsyncCloseClient != nil {
		if err := l.svcCtx.AsyncCloseClient.EnqueueVerifyAttemptDue(l.ctx, attempt.OrderNumber, attempt.ProgramID, attempt.UserDeadlineAt); err != nil {
			l.Errorf("enqueue verify attempt failed, orderNumber=%d err=%v", attempt.OrderNumber, err)
		}
	}

	if admission.Decision == rush.AdmitDecisionReused {
		return &pb.CreateOrderResp{OrderNumber: admission.OrderNumber}, nil
	}

	event, err := buildAttemptCreateEvent(attempt, claims)
	if err != nil {
		return nil, mapOrderError(err)
	}
	body, err := event.Marshal()
	if err != nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}
	if err := l.svcCtx.OrderCreateProducer.Send(l.ctx, event.PartitionKey(), body); err != nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}
	if err := l.svcCtx.AttemptStore.MarkQueued(l.ctx, attempt.OrderNumber, time.Now()); err != nil {
		l.Errorf("mark rush attempt queued failed, orderNumber=%d err=%v", attempt.OrderNumber, err)
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
