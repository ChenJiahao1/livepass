package svc

import (
	"damai-go/pkg/xredis"
	"damai-go/services/user-rpc/internal/config"
	"damai-go/services/user-rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config           config.Config
	Redis            *xredis.Client
	DUserModel       model.DUserModel
	DUserMobileModel model.DUserMobileModel
	DUserEmailModel  model.DUserEmailModel
	DTicketUserModel model.DTicketUserModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	var rds *xredis.Client
	if c.StoreRedis.Host != "" {
		rds = xredis.MustNew(c.StoreRedis)
	}

	return &ServiceContext{
		Config:           c,
		Redis:            rds,
		DUserModel:       model.NewDUserModel(conn),
		DUserMobileModel: model.NewDUserMobileModel(conn),
		DUserEmailModel:  model.NewDUserEmailModel(conn),
		DTicketUserModel: model.NewDTicketUserModel(conn),
	}
}
