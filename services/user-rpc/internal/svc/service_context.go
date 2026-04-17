package svc

import (
	"livepass/pkg/xmysql"
	"livepass/pkg/xredis"
	"livepass/services/user-rpc/internal/config"
	"livepass/services/user-rpc/internal/model"

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
	c.MySQL = c.MySQL.Normalize()
	c.MySQL.DataSource = xmysql.WithLocalTime(c.MySQL.DataSource)
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	rawDB, err := conn.RawDB()
	if err != nil {
		panic(err)
	}
	xmysql.ApplyPool(rawDB, c.MySQL)
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
