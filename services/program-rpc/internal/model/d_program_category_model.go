package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DProgramCategoryModel = (*customDProgramCategoryModel)(nil)

type (
	// DProgramCategoryModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDProgramCategoryModel.
	DProgramCategoryModel interface {
		dProgramCategoryModel
		withSession(session sqlx.Session) DProgramCategoryModel
		FindAll(ctx context.Context) ([]*DProgramCategory, error)
	}

	customDProgramCategoryModel struct {
		*defaultDProgramCategoryModel
	}
)

// NewDProgramCategoryModel returns a model for the database table.
func NewDProgramCategoryModel(conn sqlx.SqlConn) DProgramCategoryModel {
	return &customDProgramCategoryModel{
		defaultDProgramCategoryModel: newDProgramCategoryModel(conn),
	}
}

func (m *customDProgramCategoryModel) withSession(session sqlx.Session) DProgramCategoryModel {
	return NewDProgramCategoryModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDProgramCategoryModel) FindAll(ctx context.Context) ([]*DProgramCategory, error) {
	query := fmt.Sprintf("select %s from %s where `status` = 1 order by `type` asc, `parent_id` asc, `id` asc", dProgramCategoryRows, m.table)
	var resp []*DProgramCategory
	if err := m.conn.QueryRowsCtx(ctx, &resp, query); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return resp, nil
}
