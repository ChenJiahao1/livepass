package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DUserModel = (*customDUserModel)(nil)

type (
	// DUserModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDUserModel.
	DUserModel interface {
		dUserModel
		withSession(session sqlx.Session) DUserModel
		FindOneByMobile(ctx context.Context, mobile string) (*DUser, error)
	}

	customDUserModel struct {
		*defaultDUserModel
	}
)

// NewDUserModel returns a model for the database table.
func NewDUserModel(conn sqlx.SqlConn) DUserModel {
	return &customDUserModel{
		defaultDUserModel: newDUserModel(conn),
	}
}

func (m *customDUserModel) withSession(session sqlx.Session) DUserModel {
	return NewDUserModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDUserModel) FindOneByMobile(ctx context.Context, mobile string) (*DUser, error) {
	query := fmt.Sprintf("select %s from %s where `mobile` = ? and `status` = 1 limit 1", dUserRows, m.table)
	var resp DUser
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
