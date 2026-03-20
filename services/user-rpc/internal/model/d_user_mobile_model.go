package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DUserMobileModel = (*customDUserMobileModel)(nil)

type (
	// DUserMobileModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDUserMobileModel.
	DUserMobileModel interface {
		dUserMobileModel
		withSession(session sqlx.Session) DUserMobileModel
	}

	customDUserMobileModel struct {
		*defaultDUserMobileModel
	}
)

// NewDUserMobileModel returns a model for the database table.
func NewDUserMobileModel(conn sqlx.SqlConn) DUserMobileModel {
	return &customDUserMobileModel{
		defaultDUserMobileModel: newDUserMobileModel(conn),
	}
}

func (m *customDUserMobileModel) withSession(session sqlx.Session) DUserMobileModel {
	return NewDUserMobileModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDUserMobileModel) FindOneByMobile(ctx context.Context, mobile string) (*DUserMobile, error) {
	query := fmt.Sprintf("select %s from %s where `mobile` = ? and `status` = 1 limit 1", dUserMobileRows, m.table)
	var resp DUserMobile
	err := m.conn.QueryRowCtx(ctx, &resp, query, mobile)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
