package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InvalidProgramLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInvalidProgramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InvalidProgramLogic {
	return &InvalidProgramLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InvalidProgramLogic) InvalidProgram(in *pb.ProgramInvalidReq) (*pb.BoolResp, error) {
	if in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	program, err := l.svcCtx.DProgramModel.FindOne(l.ctx, in.GetId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}

	program.ProgramStatus = 0
	program.EditTime = time.Now()
	if err := l.svcCtx.DProgramModel.Update(l.ctx, program); err != nil {
		return nil, err
	}

	if err := l.svcCtx.ProgramCacheInvalidator.InvalidateProgram(l.ctx, program.Id, program.ProgramGroupId); err != nil {
		l.Errorf("invalidate program caches after invalid failed, programID=%d groupID=%d err=%v", program.Id, program.ProgramGroupId, err)
	}
	if err := clearProgramSeatLedgersByProgram(l.ctx, l.svcCtx, program.Id); err != nil {
		return nil, err
	}

	return &pb.BoolResp{Success: true}, nil
}
