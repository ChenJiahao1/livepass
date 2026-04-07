package logic

import (
	"context"
	"errors"
	"sort"
	"strconv"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"
)

func buildProgramCategoryListResp(categories []*model.DProgramCategory) *pb.ProgramCategoryListResp {
	if len(categories) == 0 {
		return &pb.ProgramCategoryListResp{}
	}

	list := make([]*pb.ProgramCategoryInfo, 0, len(categories))
	for _, category := range categories {
		if category == nil {
			continue
		}
		list = append(list, &pb.ProgramCategoryInfo{
			Id:       category.Id,
			ParentId: category.ParentId,
			Name:     category.Name,
			Type:     category.Type,
		})
	}

	return &pb.ProgramCategoryListResp{List: list}
}

func clearProgramSeatLedgersByProgram(ctx context.Context, svcCtx *svc.ServiceContext, programID int64) error {
	categories, err := svcCtx.DTicketCategoryModel.FindByProgramId(ctx, programID)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return err
	}

	ids := make([]int64, 0, len(categories))
	for _, category := range categories {
		if category == nil {
			continue
		}
		ids = append(ids, category.Id)
	}

	return clearProgramSeatLedgers(ctx, svcCtx, programID, ids)
}

func clearProgramSeatLedgers(ctx context.Context, svcCtx *svc.ServiceContext, programID int64, ticketCategoryIDs []int64) error {
	if svcCtx.SeatStockStore == nil {
		return nil
	}

	categories, err := svcCtx.DTicketCategoryModel.FindByProgramId(ctx, programID)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return err
	}

	showTimeByCategoryID := make(map[int64]int64, len(categories))
	for _, category := range categories {
		if category == nil || category.Id <= 0 || category.ShowTimeId <= 0 {
			continue
		}
		showTimeByCategoryID[category.Id] = category.ShowTimeId
	}

	for _, ticketCategoryID := range uniqueInt64Values(ticketCategoryIDs) {
		if ticketCategoryID <= 0 {
			continue
		}
		showTimeID := showTimeByCategoryID[ticketCategoryID]
		if showTimeID <= 0 {
			continue
		}
		if err := svcCtx.SeatStockStore.Clear(ctx, showTimeID, ticketCategoryID); err != nil && !errors.Is(err, xerr.ErrProgramSeatLedgerNotReady) {
			return err
		}
	}

	return nil
}

func uniqueInt64Values(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

func ticketCategoryPriceString(price float64) string {
	return strconv.FormatInt(int64(price), 10)
}
