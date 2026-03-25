// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"damai-go/services/program-api/internal/svc"
	"damai-go/services/program-api/internal/types"
	"damai-go/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchCreateProgramCategoriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBatchCreateProgramCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchCreateProgramCategoriesLogic {
	return &BatchCreateProgramCategoriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchCreateProgramCategoriesLogic) BatchCreateProgramCategories(req *types.ProgramCategoryBatchSaveReq) (resp *types.BoolResp, err error) {
	items := make([]*programrpc.ProgramCategoryBatchItem, 0, len(req.List))
	for _, item := range req.List {
		items = append(items, &programrpc.ProgramCategoryBatchItem{
			ParentId: item.ParentID,
			Name:     item.Name,
			Type:     item.Type,
		})
	}

	rpcResp, err := l.svcCtx.ProgramRpc.BatchCreateProgramCategories(l.ctx, &programrpc.ProgramCategoryBatchSaveReq{
		List: items,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
