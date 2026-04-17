package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"livepass/pkg/xerr"
	"livepass/pkg/xid"
	"livepass/services/program-rpc/internal/model"
	"livepass/services/program-rpc/internal/svc"
	"livepass/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BatchCreateProgramCategoriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchCreateProgramCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchCreateProgramCategoriesLogic {
	return &BatchCreateProgramCategoriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BatchCreateProgramCategoriesLogic) BatchCreateProgramCategories(in *pb.ProgramCategoryBatchSaveReq) (*pb.BoolResp, error) {
	if len(in.GetList()) == 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	seen := make(map[string]struct{}, len(in.GetList()))
	items := make([]*pb.ProgramCategoryBatchItem, 0, len(in.GetList()))
	for _, item := range in.GetList() {
		if item == nil || item.GetType() <= 0 || strings.TrimSpace(item.GetName()) == "" {
			return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
		}
		key := fmt.Sprintf("%d|%s|%d", item.GetParentId(), strings.TrimSpace(item.GetName()), item.GetType())
		if _, ok := seen[key]; ok {
			return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
		}
		seen[key] = struct{}{}
		items = append(items, &pb.ProgramCategoryBatchItem{
			ParentId: item.GetParentId(),
			Name:     strings.TrimSpace(item.GetName()),
			Type:     item.GetType(),
		})
	}

	now := time.Now()
	err := l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		categoryModel := model.NewDProgramCategoryModel(sqlx.NewSqlConnFromSession(session))

		for _, item := range items {
			_, err := categoryModel.FindOneByParentIdNameType(ctx, item.GetParentId(), item.GetName(), item.GetType())
			if err == nil {
				return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
			}
			if err != nil && !errors.Is(err, model.ErrNotFound) {
				return err
			}
		}

		for _, item := range items {
			if _, err := categoryModel.InsertWithCreateTime(ctx, &model.DProgramCategory{
				Id:         xid.New(),
				ParentId:   item.GetParentId(),
				Name:       item.GetName(),
				Type:       item.GetType(),
				CreateTime: now,
				EditTime:   now,
				Status:     1,
			}); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if l.svcCtx.ProgramCacheInvalidator != nil {
		if err := l.svcCtx.ProgramCacheInvalidator.InvalidateCategorySnapshot(l.ctx); err != nil {
			l.Errorf("invalidate category snapshot after batch create failed, err=%v", err)
		}
	}

	return &pb.BoolResp{Success: true}, nil
}
