package model

import (
	"context"
	"database/sql"
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
		InsertWithCreateTime(ctx context.Context, data *DProgramCategory) (sql.Result, error)
		FindAll(ctx context.Context) ([]*DProgramCategory, error)
		FindByType(ctx context.Context, tp int64) ([]*DProgramCategory, error)
		FindByParentID(ctx context.Context, parentID int64) ([]*DProgramCategory, error)
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

func (m *customDProgramCategoryModel) InsertWithCreateTime(ctx context.Context, data *DProgramCategory) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`parent_id`, `name`, `type`, `create_time`, `edit_time`, `status`) values (?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return m.conn.ExecCtx(
		ctx,
		query,
		data.ParentId,
		data.Name,
		data.Type,
		data.CreateTime,
		data.EditTime,
		data.Status,
	)
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

func (m *customDProgramCategoryModel) FindByType(ctx context.Context, tp int64) ([]*DProgramCategory, error) {
	query := fmt.Sprintf("select %s from %s where `status` = 1 and `type` = ? order by `parent_id` asc, `id` asc", dProgramCategoryRows, m.table)
	var resp []*DProgramCategory
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, tp); err != nil {
		if err == sqlx.ErrNotFound {
			return []*DProgramCategory{}, nil
		}
		return nil, err
	}

	return resp, nil
}

func (m *customDProgramCategoryModel) FindByParentID(ctx context.Context, parentID int64) ([]*DProgramCategory, error) {
	query := fmt.Sprintf("select %s from %s where `status` = 1 and `parent_id` = ? order by `type` asc, `id` asc", dProgramCategoryRows, m.table)
	var resp []*DProgramCategory
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, parentID); err != nil {
		if err == sqlx.ErrNotFound {
			return []*DProgramCategory{}, nil
		}
		return nil, err
	}

	return resp, nil
}
