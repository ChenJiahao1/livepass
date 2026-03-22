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

type GetProgramDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetProgramDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProgramDetailLogic {
	return &GetProgramDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProgramDetailLogic) GetProgramDetail(req *types.GetProgramDetailReq) (resp *types.ProgramDetailInfo, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.GetProgramDetail(l.ctx, &programrpc.GetProgramDetailReq{Id: req.ID})
	if err != nil {
		return nil, err
	}

	return mapProgramDetailInfo(rpcResp), nil
}
