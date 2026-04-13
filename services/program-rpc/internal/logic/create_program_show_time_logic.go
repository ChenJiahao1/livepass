package logic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xid"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateProgramShowTimeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateProgramShowTimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProgramShowTimeLogic {
	return &CreateProgramShowTimeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateProgramShowTimeLogic) CreateProgramShowTime(in *pb.ProgramShowTimeAddReq) (*pb.IdResp, error) {
	if in.GetProgramId() <= 0 || in.GetShowTime() == "" || in.GetShowWeekTime() == "" {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	showTime, err := time.ParseInLocation(programDateTimeLayout, in.GetShowTime(), time.Local)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	showDayTime, err := parseNullTime(in.GetShowDayTime())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	rushSaleOpenTime, err := parseNullTime(in.GetRushSaleOpenTime())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	rushSaleEndTime, err := parseNullTime(in.GetRushSaleEndTime())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	showEndTime, err := parseNullTime(in.GetShowEndTime())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	var (
		id             = xid.New()
		programGroupID int64
	)
	now := time.Now()
	createdShowTime := &model.DProgramShowTime{
		Id:                     id,
		ProgramId:              in.GetProgramId(),
		ShowTime:               showTime,
		ShowDayTime:            showDayTime,
		ShowWeekTime:           in.GetShowWeekTime(),
		RushSaleOpenTime:       rushSaleOpenTime,
		RushSaleEndTime:        rushSaleEndTime,
		ShowEndTime:            showEndTime,
		InventoryPreheatStatus: 0,
		EditTime:               sql.NullTime{Time: now, Valid: true},
		Status:                 1,
	}

	err = l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		conn := sqlx.NewSqlConnFromSession(session)
		programModel := model.NewDProgramModel(conn)
		groupModel := model.NewDProgramGroupModel(conn)
		showTimeModel := model.NewDProgramShowTimeModel(conn)

		program, err := programModel.FindOne(ctx, in.GetProgramId())
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return programNotFoundError()
			}
			return err
		}
		programGroupID = program.ProgramGroupId

		if _, err := showTimeModel.Insert(ctx, createdShowTime); err != nil {
			return err
		}

		group, err := groupModel.FindOne(ctx, programGroupID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return programGroupNotFoundError()
			}
			return err
		}
		if group.RecentShowTime.IsZero() || showTime.Before(group.RecentShowTime) {
			group.RecentShowTime = showTime
			group.EditTime = now
			if err := groupModel.Update(ctx, group); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := scheduleRushInventoryPreheat(l.ctx, l.svcCtx, createdShowTime); err != nil {
		return nil, err
	}

	if err := l.svcCtx.ProgramCacheInvalidator.InvalidateProgram(l.ctx, in.GetProgramId(), programGroupID); err != nil {
		l.Errorf("invalidate program caches after create show time failed, programID=%d groupID=%d err=%v", in.GetProgramId(), programGroupID, err)
	}

	return &pb.IdResp{Id: id}, nil
}
