package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
)

func PrimeAdmissionQuota(ctx context.Context, svcCtx *svc.ServiceContext, showTimeID int64) error {
	return PrimeRushRuntime(ctx, svcCtx, showTimeID)
}
