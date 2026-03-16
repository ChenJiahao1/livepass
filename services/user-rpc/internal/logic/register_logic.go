package logic

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xid"
	"damai-go/services/user-rpc/internal/model"
	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RegisterLogic) Register(in *pb.RegisterReq) (*pb.BoolResp, error) {
	if in.Mobile == "" || in.Password == "" {
		return nil, xerr.ErrInvalidParam
	}

	_, err := l.svcCtx.DUserModel.FindOneByMobile(l.ctx, in.Mobile)
	switch {
	case err == nil:
		return nil, errors.New("user already exists")
	case !errors.Is(err, model.ErrNotFound):
		return nil, err
	}

	now := time.Now()
	user := &model.DUser{
		Id:       xid.New(),
		Mobile:   in.Mobile,
		Gender:   1,
		Password: sql.NullString{String: md5Hex(in.Password), Valid: true},
		EditTime: sql.NullTime{Time: now, Valid: true},
		Status:   1,
	}
	if _, err := l.svcCtx.DUserModel.Insert(l.ctx, user); err != nil {
		return nil, err
	}

	return &pb.BoolResp{Success: true}, nil
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
