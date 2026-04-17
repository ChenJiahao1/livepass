// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"livepass/services/program-api/internal/svc"
	"livepass/services/program-api/internal/types"
	"livepass/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PageProgramsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPageProgramsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PageProgramsLogic {
	return &PageProgramsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PageProgramsLogic) PagePrograms(req *types.PageProgramsReq) (resp *types.ProgramPageResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.PagePrograms(l.ctx, &programrpc.PageProgramsReq{
		PageNumber:              req.PageNumber,
		PageSize:                req.PageSize,
		AreaId:                  req.AreaId,
		ParentProgramCategoryId: req.ParentProgramCategoryId,
		ProgramCategoryId:       req.ProgramCategoryId,
		TimeType:                req.TimeType,
		StartDateTime:           req.StartDateTime,
		EndDateTime:             req.EndDateTime,
		Type:                    req.Type,
	})
	if err != nil {
		return nil, err
	}

	return mapProgramPageResp(rpcResp), nil
}
