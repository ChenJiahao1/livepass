package logic

import (
	"context"
	"sort"
	"time"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"
	programrpc "livepass/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

const primeRushRuntimeGuardBatchSize = 256

type PrimeRushRuntimeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPrimeRushRuntimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PrimeRushRuntimeLogic {
	return &PrimeRushRuntimeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PrimeRushRuntimeLogic) PrimeRushRuntime(in *pb.PrimeRushRuntimeReq) (*pb.BoolResp, error) {
	if in == nil || in.GetProgramId() <= 0 {
		return nil, mapOrderError(xerr.ErrInvalidParam)
	}
	if err := PrimeRushRuntime(l.ctx, l.svcCtx, in.GetProgramId()); err != nil {
		return nil, mapOrderError(err)
	}

	return &pb.BoolResp{Success: true}, nil
}

func PrimeRushRuntime(ctx context.Context, svcCtx *svc.ServiceContext, programID int64) error {
	if svcCtx == nil || svcCtx.AttemptStore == nil || svcCtx.ProgramRpc == nil || svcCtx.OrderRepository == nil {
		return xerr.ErrInternal
	}
	if programID <= 0 {
		return xerr.ErrInvalidParam
	}

	showTimesResp, err := svcCtx.ProgramRpc.ListProgramShowTimesForRush(ctx, &programrpc.ListProgramShowTimesForRushReq{
		ProgramId: programID,
	})
	if err != nil {
		return err
	}

	showTimeIDs := make([]int64, 0, len(showTimesResp.GetList()))
	seen := make(map[int64]struct{}, len(showTimesResp.GetList()))
	for _, item := range showTimesResp.GetList() {
		if item == nil || item.GetShowTimeId() <= 0 {
			continue
		}
		if _, ok := seen[item.GetShowTimeId()]; ok {
			continue
		}
		seen[item.GetShowTimeId()] = struct{}{}
		showTimeIDs = append(showTimeIDs, item.GetShowTimeId())
	}
	if len(showTimeIDs) == 0 {
		return xerr.ErrProgramShowTimeNotFound
	}
	sort.Slice(showTimeIDs, func(i, j int) bool { return showTimeIDs[i] < showTimeIDs[j] })

	for _, showTimeID := range showTimeIDs {
		if err := primeRushRuntimeByShowTime(ctx, svcCtx, showTimeID); err != nil {
			return err
		}
	}

	return nil
}

func primeRushRuntimeByShowTime(ctx context.Context, svcCtx *svc.ServiceContext, showTimeID int64) error {
	preorder, err := svcCtx.ProgramRpc.GetProgramPreorder(ctx, &programrpc.GetProgramPreorderReq{
		ShowTimeId: showTimeID,
	})
	if err != nil {
		return err
	}

	resolvedShowTimeID := preorder.GetShowTimeId()
	if resolvedShowTimeID <= 0 {
		resolvedShowTimeID = showTimeID
	}

	if err := svcCtx.AttemptStore.ClearUserInflightByShowTime(ctx, resolvedShowTimeID); err != nil {
		return err
	}
	if err := svcCtx.AttemptStore.ClearViewerInflightByShowTime(ctx, resolvedShowTimeID); err != nil {
		return err
	}
	if err := svcCtx.AttemptStore.ClearQuotaByShowTime(ctx, resolvedShowTimeID); err != nil {
		return err
	}

	now := time.Now()
	activeTTLSeconds := computePrimeActiveTTLSeconds(preorder, now)

	userRows := make(map[int64]int64)
	if err := svcCtx.OrderRepository.WalkActiveUserGuardsByShowTime(ctx, resolvedShowTimeID, primeRushRuntimeGuardBatchSize, func(batch []*model.DOrderUserGuard) error {
		for _, item := range batch {
			if item == nil || item.UserId <= 0 || item.OrderNumber <= 0 {
				continue
			}
			userRows[item.UserId] = item.OrderNumber
		}
		return nil
	}); err != nil {
		return err
	}
	if err := svcCtx.AttemptStore.ReplaceUserActiveByShowTime(ctx, resolvedShowTimeID, userRows, activeTTLSeconds); err != nil {
		return err
	}

	viewerRows := make(map[int64]int64)
	if err := svcCtx.OrderRepository.WalkActiveViewerGuardsByShowTime(ctx, resolvedShowTimeID, primeRushRuntimeGuardBatchSize, func(batch []*model.DOrderViewerGuard) error {
		for _, item := range batch {
			if item == nil || item.ViewerId <= 0 || item.OrderNumber <= 0 {
				continue
			}
			viewerRows[item.ViewerId] = item.OrderNumber
		}
		return nil
	}); err != nil {
		return err
	}
	if err := svcCtx.AttemptStore.ReplaceViewerActiveByShowTime(ctx, resolvedShowTimeID, viewerRows, activeTTLSeconds); err != nil {
		return err
	}

	for _, ticketCategory := range preorder.GetTicketCategoryVoList() {
		if ticketCategory == nil || ticketCategory.GetId() <= 0 {
			continue
		}
		if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, resolvedShowTimeID, ticketCategory.GetId(), ticketCategory.GetAdmissionQuota()); err != nil {
			return err
		}
	}

	return nil
}

func computePrimeActiveTTLSeconds(preorder *programrpc.ProgramPreorderInfo, now time.Time) int {
	if preorder == nil {
		return 0
	}

	if parsed, err := parsePrimeRuntimeTime(preorder.GetRushSaleEndTime()); err == nil {
		return rushTTLSeconds(now, parsed)
	}
	if parsed, err := parsePrimeRuntimeTime(preorder.GetShowTime()); err == nil {
		return rushTTLSeconds(now, parsed)
	}

	return 0
}

func parsePrimeRuntimeTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, xerr.ErrInvalidParam
	}
	return time.ParseInLocation(orderDateTimeLayout, raw, time.Local)
}

func rushTTLSeconds(now, endAt time.Time) int {
	const retention = 7 * 24 * 60 * 60
	if now.IsZero() || endAt.IsZero() {
		return retention
	}

	seconds := int(endAt.Sub(now).Seconds()) + retention
	if seconds < retention {
		return retention
	}

	return seconds
}
