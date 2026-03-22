package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
	programrpc "damai-go/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	orderCreateSendFailedReleaseReason = "order_create_failed"
	orderCreateExpiredReleaseReason    = "order_create_expired"
)

func compensateOrderCreateSendFailureLegacy(ctx context.Context, svcCtx *svc.ServiceContext, freezeToken string) {
	releaseOrderCreateFreeze(ctx, svcCtx, freezeToken, orderCreateSendFailedReleaseReason)
}

func compensateOrderCreateExpired(ctx context.Context, svcCtx *svc.ServiceContext, userID, programID, orderNumber int64, freezeToken string) {
	releaseOrderCreatePurchaseLimit(ctx, svcCtx, userID, programID, orderNumber)
	releaseOrderCreateFreeze(ctx, svcCtx, freezeToken, orderCreateExpiredReleaseReason)
}

func compensateOrderCreateSeatFreezeFailure(ctx context.Context, svcCtx *svc.ServiceContext, userID, programID, orderNumber int64) {
	releaseOrderCreatePurchaseLimit(ctx, svcCtx, userID, programID, orderNumber)
}

func compensateOrderCreateSendFailure(ctx context.Context, svcCtx *svc.ServiceContext, userID, programID, orderNumber int64, freezeToken string) {
	releaseOrderCreatePurchaseLimit(ctx, svcCtx, userID, programID, orderNumber)
	releaseOrderCreateFreeze(ctx, svcCtx, freezeToken, orderCreateSendFailedReleaseReason)
}

func releaseOrderCreateFreeze(ctx context.Context, svcCtx *svc.ServiceContext, freezeToken, reason string) {
	if freezeToken == "" || svcCtx == nil || svcCtx.ProgramRpc == nil {
		return
	}

	if _, err := svcCtx.ProgramRpc.ReleaseSeatFreeze(ctx, &programrpc.ReleaseSeatFreezeReq{
		FreezeToken:   freezeToken,
		ReleaseReason: reason,
	}); err != nil {
		logx.WithContext(ctx).Errorf("release seat freeze compensation failed, freezeToken=%s err=%v", freezeToken, err)
	}
}

func releaseOrderCreatePurchaseLimit(ctx context.Context, svcCtx *svc.ServiceContext, userID, programID, orderNumber int64) {
	if userID <= 0 || programID <= 0 || orderNumber <= 0 || svcCtx == nil || svcCtx.PurchaseLimitStore == nil {
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
