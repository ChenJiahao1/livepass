package svc

import (
	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config                config.Config
	DProgramModel         model.DProgramModel
	DProgramCategoryModel model.DProgramCategoryModel
	DProgramGroupModel    model.DProgramGroupModel
	DProgramShowTimeModel model.DProgramShowTimeModel
	DTicketCategoryModel  model.DTicketCategoryModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.MySQL.DataSource)

	return &ServiceContext{
		Config:                c,
		DProgramModel:         model.NewDProgramModel(conn),
		DProgramCategoryModel: model.NewDProgramCategoryModel(conn),
		DProgramGroupModel:    model.NewDProgramGroupModel(conn),
		DProgramShowTimeModel: model.NewDProgramShowTimeModel(conn),
		DTicketCategoryModel:  model.NewDTicketCategoryModel(conn),
	}
}
