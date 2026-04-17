package svc

import (
	"time"

	"livepass/pkg/xredis"
	"livepass/services/program-rpc/internal/config"
	"livepass/services/program-rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	programModelCacheTTL         = 5 * time.Minute
	programModelCacheNotFoundTTL = 30 * time.Second
)

type ProgramModels struct {
	DProgramModel         model.DProgramModel
	DProgramCategoryModel model.DProgramCategoryModel
	DProgramGroupModel    model.DProgramGroupModel
	DProgramShowTimeModel model.DProgramShowTimeModel
	DSeatModel            model.DSeatModel
	DTicketCategoryModel  model.DTicketCategoryModel
}

func newProgramModels(conn sqlx.SqlConn, rds *xredis.Client, c config.Config) ProgramModels {
	models := ProgramModels{
		DProgramModel:         model.NewDProgramModel(conn),
		DProgramCategoryModel: model.NewDProgramCategoryModel(conn),
		DProgramGroupModel:    model.NewDProgramGroupModel(conn),
		DProgramShowTimeModel: model.NewDProgramShowTimeModel(conn),
		DSeatModel:            model.NewDSeatModel(conn),
		// TicketCategory 的展示缓存落在 detail query cache，而不是 model cached 层。
		DTicketCategoryModel: model.NewDTicketCategoryModel(conn),
	}

	if rds == nil || len(c.Cache) == 0 {
		return models
	}

	cacheOpts := []cache.Option{
		cache.WithExpiry(programModelCacheTTL),
		cache.WithNotFoundExpiry(programModelCacheNotFoundTTL),
	}
	models.DProgramModel = model.NewCachedDProgramModel(conn, c.Cache, cacheOpts...)
	models.DProgramGroupModel = model.NewCachedDProgramGroupModel(conn, c.Cache, cacheOpts...)
	models.DProgramShowTimeModel = model.NewCachedDProgramShowTimeModel(conn, c.Cache, cacheOpts...)

	return models
}
