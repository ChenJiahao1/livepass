package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DProgramGroupModel = (*customDProgramGroupModel)(nil)

type (
	// DProgramGroupModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDProgramGroupModel.
	DProgramGroupModel interface {
		dProgramGroupModel
		withSession(session sqlx.Session) DProgramGroupModel
		FindOne(ctx context.Context, id int64) (*DProgramGroup, error)
	}

	customDProgramGroupModel struct {
		*defaultDProgramGroupModel
	}
)

// NewDProgramGroupModel returns a model for the database table.
func NewDProgramGroupModel(conn sqlx.SqlConn) DProgramGroupModel {
	return &customDProgramGroupModel{
		defaultDProgramGroupModel: newDProgramGroupModel(conn),
	}
}

func (m *customDProgramGroupModel) withSession(session sqlx.Session) DProgramGroupModel {
	return NewDProgramGroupModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDProgramGroupModel) FindOne(ctx context.Context, id int64) (*DProgramGroup, error) {
	query := fmt.Sprintf("select %s from %s where `id` = ? and `status` = 1 limit 1", dProgramGroupRows, m.table)
	var resp DProgramGroup
	err := m.conn.QueryRowCtx(ctx, &resp, query, id)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
