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

func compensateOrderCreateSendFailure(ctx context.Context, svcCtx *svc.ServiceContext, freezeToken string) {
	releaseOrderCreateFreeze(ctx, svcCtx, freezeToken, orderCreateSendFailedReleaseReason)
}

func compensateOrderCreateExpired(ctx context.Context, svcCtx *svc.ServiceContext, freezeToken string) {
	releaseOrderCreateFreeze(ctx, svcCtx, freezeToken, orderCreateExpiredReleaseReason)
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
