package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DUserEmailModel = (*customDUserEmailModel)(nil)

type (
	// DUserEmailModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDUserEmailModel.
	DUserEmailModel interface {
		dUserEmailModel
		withSession(session sqlx.Session) DUserEmailModel
	}

	customDUserEmailModel struct {
		*defaultDUserEmailModel
	}
)

// NewDUserEmailModel returns a model for the database table.
func NewDUserEmailModel(conn sqlx.SqlConn) DUserEmailModel {
	return &customDUserEmailModel{
		defaultDUserEmailModel: newDUserEmailModel(conn),
	}
}

func (m *customDUserEmailModel) withSession(session sqlx.Session) DUserEmailModel {
	return NewDUserEmailModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDUserEmailModel) FindOneByEmail(ctx context.Context, email string) (*DUserEmail, error) {
	query := fmt.Sprintf("select %s from %s where `email` = ? and `status` = 1 limit 1", dUserEmailRows, m.table)
	var resp DUserEmail
	err := m.conn.QueryRowCtx(ctx, &resp, query, email)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
