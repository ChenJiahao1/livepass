package svc

import (
	"damai-go/pkg/xmysql"
	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SeatFreezeLocker interface {
	Lock(key string) func()
}

type ServiceContext struct {
	Config                config.Config
	SqlConn               sqlx.SqlConn
	DProgramModel         model.DProgramModel
	DProgramCategoryModel model.DProgramCategoryModel
	DProgramGroupModel    model.DProgramGroupModel
	DProgramShowTimeModel model.DProgramShowTimeModel
	DSeatModel            model.DSeatModel
	DSeatFreezeModel      model.DSeatFreezeModel
	DTicketCategoryModel  model.DTicketCategoryModel
	SeatFreezeLocker      SeatFreezeLocker
}

func NewServiceContext(c config.Config) *ServiceContext {
	c.MySQL.DataSource = xmysql.WithLocalTime(c.MySQL.DataSource)
	conn := sqlx.NewMysql(c.MySQL.DataSource)

	return &ServiceContext{
		Config:                c,
		SqlConn:               conn,
		DProgramModel:         model.NewDProgramModel(conn),
		DProgramCategoryModel: model.NewDProgramCategoryModel(conn),
		DProgramGroupModel:    model.NewDProgramGroupModel(conn),
		DProgramShowTimeModel: model.NewDProgramShowTimeModel(conn),
		DSeatModel:            model.NewDSeatModel(conn),
		DSeatFreezeModel:      model.NewDSeatFreezeModel(conn),
		DTicketCategoryModel:  model.NewDTicketCategoryModel(conn),
	}
}
