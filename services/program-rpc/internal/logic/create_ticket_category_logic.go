package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xid"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateTicketCategoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTicketCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTicketCategoryLogic {
	return &CreateTicketCategoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateTicketCategoryLogic) CreateTicketCategory(in *pb.TicketCategoryAddReq) (*pb.IdResp, error) {
	if in.GetProgramId() <= 0 || strings.TrimSpace(in.GetIntroduce()) == "" || in.GetPrice() <= 0 || in.GetTotalNumber() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	program, err := l.svcCtx.DProgramModel.FindOne(l.ctx, in.GetProgramId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}
	if err := ensureProgramInventoryMutable(l.ctx, l.svcCtx, in.GetProgramId()); err != nil {
		return nil, mapInventoryMutationError(err)
	}

	remainNumber := in.GetRemainNumber()
	if remainNumber <= 0 {
		remainNumber = in.GetTotalNumber()
	}

	id := xid.New()
	now := time.Now()
	if _, err := l.svcCtx.DTicketCategoryModel.InsertWithCreateTime(l.ctx, &model.DTicketCategory{
		Id:           id,
		ProgramId:    in.GetProgramId(),
		Introduce:    strings.TrimSpace(in.GetIntroduce()),
		Price:        float64(in.GetPrice()),
		TotalNumber:  in.GetTotalNumber(),
		RemainNumber: remainNumber,
		CreateTime:   now,
		EditTime:     now,
		Status:       1,
	}); err != nil {
		return nil, err
	}

	if err := l.svcCtx.ProgramCacheInvalidator.InvalidateProgram(l.ctx, in.GetProgramId(), program.ProgramGroupId); err != nil {
		l.Errorf("invalidate program caches after create ticket category failed, programID=%d groupID=%d err=%v", in.GetProgramId(), program.ProgramGroupId, err)
	}

	return &pb.IdResp{Id: id}, nil
}
