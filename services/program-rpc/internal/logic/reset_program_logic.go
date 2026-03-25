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
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResetProgramLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetProgramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetProgramLogic {
	return &ResetProgramLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResetProgramLogic) ResetProgram(in *pb.ProgramResetReq) (*pb.BoolResp, error) {
	if in.GetProgramId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	var programGroupID int64
	now := time.Now()
	err := l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		conn := sqlx.NewSqlConnFromSession(session)
		programModel := model.NewDProgramModel(conn)

		program, err := programModel.FindOne(ctx, in.GetProgramId())
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return programNotFoundError()
			}
			return err
		}
		programGroupID = program.ProgramGroupId

		if _, err := conn.ExecCtx(
			ctx,
			"UPDATE d_seat SET seat_status = 1, freeze_token = NULL, freeze_expire_time = NULL, edit_time = ? WHERE status = 1 AND program_id = ? AND seat_status IN (2, 3)",
			now,
			in.GetProgramId(),
		); err != nil {
			return err
		}
		if _, err := conn.ExecCtx(
			ctx,
			"UPDATE d_ticket_category SET remain_number = total_number, edit_time = ? WHERE status = 1 AND program_id = ?",
			now,
			in.GetProgramId(),
		); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.ProgramCacheInvalidator.InvalidateProgram(l.ctx, in.GetProgramId(), programGroupID); err != nil {
		l.Errorf("invalidate program caches after reset failed, programID=%d groupID=%d err=%v", in.GetProgramId(), programGroupID, err)
	}
	if err := clearProgramSeatLedgersByProgram(l.ctx, l.svcCtx, in.GetProgramId()); err != nil {
		return nil, err
	}

	return &pb.BoolResp{Success: true}, nil
}
