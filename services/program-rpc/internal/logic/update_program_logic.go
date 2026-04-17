package logic

import (
	"context"
	"errors"
	"time"

	"livepass/services/program-rpc/internal/model"
	"livepass/services/program-rpc/internal/svc"
	"livepass/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type UpdateProgramLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateProgramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProgramLogic {
	return &UpdateProgramLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateProgramLogic) UpdateProgram(in *pb.UpdateProgramReq) (*pb.BoolResp, error) {
	values := newUpdateProgramValues(in)
	if err := validateProgramWriteValues(values, true); err != nil {
		return nil, err
	}

	now := time.Now()
	data, err := buildProgramModel(values, now)
	if err != nil {
		return nil, err
	}

	groupIDs := []int64{values.programGroupId}
	err = l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		groupModel := model.NewDProgramGroupModel(sqlx.NewSqlConnFromSession(session))
		if _, err := groupModel.FindOne(ctx, values.programGroupId); err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return programGroupNotFoundError()
			}
			return err
		}

		programModel := model.NewDProgramModel(sqlx.NewSqlConnFromSession(session))
		showTimeModel := model.NewDProgramShowTimeModel(sqlx.NewSqlConnFromSession(session))
		current, err := programModel.FindOneForUpdate(ctx, session, values.id)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return programNotFoundError()
			}
			return err
		}
		data.CreateTime = current.CreateTime
		data.InventoryPreheatStatus = current.InventoryPreheatStatus
		if current.ProgramGroupId != values.programGroupId {
			groupIDs = append(groupIDs, current.ProgramGroupId)
		}
		if err := validateProgramRushSaleWindowAgainstExistingShowTimes(ctx, showTimeModel, values.id, values.rushSaleOpenTime); err != nil {
			return err
		}
		shouldReschedule := current.RushSaleOpenTime != data.RushSaleOpenTime || current.RushSaleEndTime != data.RushSaleEndTime

		if err := programModel.Update(ctx, data); err != nil {
			return err
		}
		if !shouldReschedule {
			return nil
		}

		return scheduleRushInventoryPreheat(ctx, l.svcCtx, programModel, nil, sqlx.NewSqlConnFromSession(session), data)
	})
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.ProgramCacheInvalidator.InvalidateProgram(l.ctx, values.id, groupIDs...); err != nil {
		l.Errorf("invalidate program caches after update failed, programID=%d groupIDs=%v err=%v", values.id, groupIDs, err)
	}

	return &pb.BoolResp{Success: true}, nil
}
