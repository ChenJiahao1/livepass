package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xid"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type CreateProgramLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateProgramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProgramLogic {
	return &CreateProgramLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateProgramLogic) CreateProgram(in *pb.CreateProgramReq) (*pb.CreateProgramResp, error) {
	values := newCreateProgramValues(in)
	applyCreateProgramDefaults(&values)
	if err := validateProgramWriteValues(values, false); err != nil {
		return nil, err
	}

	now := time.Now()
	programID := xid.New()
	values.id = programID

	data, err := buildProgramModel(values, now)
	if err != nil {
		return nil, err
	}

	err = l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		groupModel := model.NewDProgramGroupModel(sqlx.NewSqlConnFromSession(session))
		if _, err := groupModel.FindOne(ctx, values.programGroupId); err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return programGroupNotFoundError()
			}
			return err
		}

		programModel := model.NewDProgramModel(sqlx.NewSqlConnFromSession(session))
		_, err := programModel.InsertWithSession(ctx, session, data)
		return err
	})
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.ProgramCacheInvalidator.InvalidateProgram(l.ctx, programID, values.programGroupId); err != nil {
		l.Errorf("invalidate program caches after create failed, programID=%d groupID=%d err=%v", programID, values.programGroupId, err)
	}

	return &pb.CreateProgramResp{Id: programID}, nil
}
