package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateProgramShowTimeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateProgramShowTimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProgramShowTimeLogic {
	return &UpdateProgramShowTimeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateProgramShowTimeLogic) UpdateProgramShowTime(in *pb.UpdateProgramShowTimeReq) (*pb.BoolResp, error) {
	if in.GetId() <= 0 || strings.TrimSpace(in.GetShowWeekTime()) == "" {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	showEndTime, err := parseNullTime(in.GetShowEndTime())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	var (
		programID      int64
		programGroupID int64
	)
	now := time.Now()

	err = l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		sessionConn := sqlx.NewSqlConnFromSession(session)
		showTimeModel := model.NewDProgramShowTimeModel(sessionConn)
		programModel := model.NewDProgramModel(sessionConn)

		current, err := showTimeModel.FindOne(ctx, in.GetId())
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return status.Error(codes.NotFound, xerr.ErrProgramShowTimeNotFound.Error())
			}
			return err
		}

		programID = current.ProgramId
		program, err := programModel.FindOne(ctx, programID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return programNotFoundError()
			}
			return err
		}
		programGroupID = program.ProgramGroupId
		if err := validateShowTimeAgainstProgramRushSaleOpenTime(program, current.ShowTime); err != nil {
			return err
		}

		current.ShowWeekTime = in.GetShowWeekTime()
		current.ShowEndTime = showEndTime
		current.EditTime = sql.NullTime{Time: now, Valid: true}

		if err := showTimeModel.Update(ctx, current); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.ProgramCacheInvalidator.InvalidateProgram(l.ctx, programID, programGroupID); err != nil {
		l.Errorf("invalidate program caches after update show time failed, showTimeID=%d programID=%d groupID=%d err=%v", in.GetId(), programID, programGroupID, err)
	}

	return &pb.BoolResp{Success: true}, nil
}
