package logic

import (
	"context"
	"time"

	"damai-go/services/order-rpc/internal/svc"
	programrpc "damai-go/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	orderCreateSendFailedReleaseReason = "order_create_failed"
	orderCreateExpiredReleaseReason    = "order_create_expired"
	orderCreateCompensationTimeout     = 2 * time.Second
)

func compensateOrderCreateSendFailureFallback(ctx context.Context, svcCtx *svc.ServiceContext, freezeToken string) {
	_ = ctx
	_ = svcCtx
	_ = freezeToken
}

func compensateOrderCreateExpired(ctx context.Context, svcCtx *svc.ServiceContext, userID, programID, orderNumber int64, freezeToken string) {
	releaseOrderCreatePurchaseLimit(ctx, svcCtx, userID, programID, orderNumber)
	releaseOrderCreateFreeze(ctx, svcCtx, freezeToken, orderCreateExpiredReleaseReason)
}

func compensateOrderCreateSeatFreezeFailure(ctx context.Context, svcCtx *svc.ServiceContext, userID, programID, orderNumber int64, freezeReq *programrpc.AutoAssignAndFreezeSeatsReq) {
	compensationCtx, cancel := newOrderCreateCompensationContext()
	defer cancel()

	releaseOrderCreatePurchaseLimit(compensationCtx, svcCtx, userID, programID, orderNumber)
	recoverAndReleaseOrderCreateFreeze(compensationCtx, svcCtx, freezeReq)
}

func compensateOrderCreateSendFailure(ctx context.Context, svcCtx *svc.ServiceContext, userID, programID, orderNumber int64, freezeToken string) {
	_ = ctx
	_ = svcCtx
	_ = userID
	_ = programID
	_ = orderNumber
	_ = freezeToken
}

func compensateOrderCreateBeforeSendFailure(ctx context.Context, svcCtx *svc.ServiceContext, userID, programID, orderNumber int64, freezeToken string) {
	releaseOrderCreatePurchaseLimit(ctx, svcCtx, userID, programID, orderNumber)
	releaseOrderCreateFreeze(ctx, svcCtx, freezeToken, orderCreateSendFailedReleaseReason)
}

func releaseOrderCreateFreeze(ctx context.Context, svcCtx *svc.ServiceContext, freezeToken, reason string) {
	releaseOrderCreateFreezeWithOwner(ctx, svcCtx, freezeToken, reason, 0, 0)
}

func releaseOrderCreateFreezeWithOwner(ctx context.Context, svcCtx *svc.ServiceContext, freezeToken, reason string, ownerOrderNumber, ownerEpoch int64) {
	if freezeToken == "" || svcCtx == nil || svcCtx.ProgramRpc == nil {
		return
	}

	if _, err := svcCtx.ProgramRpc.ReleaseSeatFreeze(ctx, &programrpc.ReleaseSeatFreezeReq{
		FreezeToken:      freezeToken,
		ReleaseReason:    reason,
		OwnerOrderNumber: ownerOrderNumber,
		OwnerEpoch:       ownerEpoch,
	}); err != nil {
		logx.WithContext(ctx).Errorf("release seat freeze compensation failed, freezeToken=%s err=%v", freezeToken, err)
	}
}

func recoverAndReleaseOrderCreateFreeze(ctx context.Context, svcCtx *svc.ServiceContext, freezeReq *programrpc.AutoAssignAndFreezeSeatsReq) {
	if svcCtx == nil || svcCtx.ProgramRpc == nil || freezeReq == nil {
		return
	}
	if freezeReq.GetProgramId() <= 0 || freezeReq.GetTicketCategoryId() <= 0 || freezeReq.GetCount() <= 0 || freezeReq.GetRequestNo() == "" {
		return
	}

	freezeResp, err := svcCtx.ProgramRpc.AutoAssignAndFreezeSeats(ctx, freezeReq)
	if err != nil {
		logx.WithContext(ctx).Errorf(
			"recover seat freeze compensation failed, requestNo=%s programId=%d ticketCategoryId=%d err=%v",
			freezeReq.GetRequestNo(),
			freezeReq.GetProgramId(),
			freezeReq.GetTicketCategoryId(),
			err,
		)
		return
	}

	releaseOrderCreateFreeze(ctx, svcCtx, freezeResp.GetFreezeToken(), orderCreateSendFailedReleaseReason)
}

func newOrderCreateCompensationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), orderCreateCompensationTimeout)
}

func releaseOrderCreatePurchaseLimit(ctx context.Context, svcCtx *svc.ServiceContext, userID, programID, orderNumber int64) {
	if userID <= 0 || programID <= 0 || orderNumber <= 0 || svcCtx == nil || svcCtx.PurchaseLimitStore == nil {
		return
	}

	snapshot, err := svcCtx.PurchaseLimitStore.Snapshot(ctx, userID, programID)
	if err != nil {
		logx.WithContext(ctx).Errorf(
			"snapshot purchase limit compensation failed, userID=%d programID=%d orderNumber=%d err=%v",
			userID,
			programID,
			orderNumber,
			err,
		)
		return
	}
	if !snapshot.Ready || snapshot.Reservations[orderNumber] <= 0 {
		return
	}

	if err := svcCtx.PurchaseLimitStore.Release(ctx, userID, programID, orderNumber); err != nil {
		logx.WithContext(ctx).Errorf(
			"release purchase limit compensation failed, userID=%d programID=%d orderNumber=%d err=%v",
			userID,
			programID,
			orderNumber,
			err,
		)
	}
}
