package svc

import (
	"damai-go/services/pay-rpc/internal/config"
	"damai-go/services/pay-rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config        config.Config
	SqlConn       sqlx.SqlConn
	DPayBillModel model.DPayBillModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.MySQL.DataSource)

	return &ServiceContext{
		Config:        c,
		SqlConn:       conn,
		DPayBillModel: model.NewDPayBillModel(conn),
	}
}
