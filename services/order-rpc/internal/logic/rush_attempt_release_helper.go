package logic

import (
	"context"
	"errors"
	"time"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/svc"
	programrpc "livepass/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

func syncClosedRushAttempt(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64, now time.Time) error {
	if svcCtx == nil || svcCtx.AttemptStore == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	record, err := svcCtx.AttemptStore.Get(ctx, orderNumber)
	if err != nil {
		if errors.Is(err, xerr.ErrOrderNotFound) {
			return nil
		}
		return err
	}

	outcome, err := svcCtx.AttemptStore.FinalizeClosedOrder(ctx, record, now)
	if err != nil {
		return err
	}

	switch outcome {
	case rush.AttemptTransitioned, rush.AttemptAlreadyFailed:
		return nil
	case rush.AttemptAlreadySucceeded, rush.AttemptLostOwnership, rush.AttemptStateMissing:
		return xerr.ErrInternal
	default:
		return xerr.ErrInternal
	}
}

func releaseOrderCreateFreeze(ctx context.Context, svcCtx *svc.ServiceContext, freezeToken, reason string) {
	if freezeToken == "" || svcCtx == nil || svcCtx.ProgramRpc == nil {
		return
	}

	if _, err := svcCtx.ProgramRpc.ReleaseSeatFreeze(ctx, &programrpc.ReleaseSeatFreezeReq{
		FreezeToken:   freezeToken,
		ReleaseReason: reason,
	}); err != nil {
		logx.WithContext(ctx).Errorf("release seat freeze failed, freezeToken=%s err=%v", freezeToken, err)
	}
}
