package svc

import (
	"damai-go/services/user-rpc/internal/config"
	"damai-go/services/user-rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config     config.Config
	DUserModel model.DUserModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.MySQL.DataSource)

	return &ServiceContext{
		Config:     c,
		DUserModel: model.NewDUserModel(conn),
	}
}
