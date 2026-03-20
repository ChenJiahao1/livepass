package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DProgramModel = (*customDProgramModel)(nil)

type (
	// DProgramModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDProgramModel.
	DProgramModel interface {
		dProgramModel
		withSession(session sqlx.Session) DProgramModel
		FindHomeList(ctx context.Context, q *ProgramHomeListQuery) ([]*DProgram, error)
		CountPageList(ctx context.Context, q *ProgramPageListQuery) (int64, error)
		FindPageList(ctx context.Context, q *ProgramPageListQuery) ([]*DProgram, error)
	}

	customDProgramModel struct {
		*defaultDProgramModel
	}

	ProgramHomeListQuery struct {
		AreaId                   int64
		ParentProgramCategoryIds []int64
		Limit                    int64
	}

	ProgramPageListQuery struct {
		PageNumber              int64
		PageSize                int64
		AreaId                  int64
		ParentProgramCategoryId int64
		ProgramCategoryId       int64
		TimeType                int64
		StartDateTime           string
		EndDateTime             string
		Type                    int64
	}
)

// NewDProgramModel returns a model for the database table.
func NewDProgramModel(conn sqlx.SqlConn) DProgramModel {
	return &customDProgramModel{
		defaultDProgramModel: newDProgramModel(conn),
	}
}

func (m *customDProgramModel) withSession(session sqlx.Session) DProgramModel {
	return NewDProgramModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDProgramModel) FindHomeList(ctx context.Context, q *ProgramHomeListQuery) ([]*DProgram, error) {
	if q == nil {
		q = &ProgramHomeListQuery{}
	}

	conditions, args := buildProgramBaseConditions(q.AreaId)
	if len(q.ParentProgramCategoryIds) > 0 {
		inClause, inArgs := buildInt64InClause(q.ParentProgramCategoryIds)
		conditions = append(conditions, fmt.Sprintf("p.`parent_program_category_id` in (%s)", inClause))
		args = append(args, inArgs...)
	}

	query := fmt.Sprintf(
		"select %s from %s p where %s order by p.`high_heat` desc, p.`id` desc",
		dProgramRows,
		m.table,
		strings.Join(conditions, " and "),
	)

	if q.Limit > 0 {
		query += " limit ?"
		args = append(args, q.Limit)
	}

	var resp []*DProgram
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return resp, nil
}

func (m *customDProgramModel) CountPageList(ctx context.Context, q *ProgramPageListQuery) (int64, error) {
	if q == nil {
		q = &ProgramPageListQuery{}
	}

	conditions, args := buildProgramPageConditions(q)
	query := fmt.Sprintf(
		"select count(1) from %s p where %s",
		m.table,
		strings.Join(conditions, " and "),
	)

	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return 0, nil
		}
		return 0, err
	}

	return total, nil
}

func (m *customDProgramModel) FindPageList(ctx context.Context, q *ProgramPageListQuery) ([]*DProgram, error) {
	if q == nil {
		q = &ProgramPageListQuery{}
	}

	conditions, args := buildProgramPageConditions(q)
	orderBy := buildProgramPageOrderBy(q.Type)

	pageNumber := q.PageNumber
	if pageNumber <= 0 {
		pageNumber = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (pageNumber - 1) * pageSize

	query := fmt.Sprintf(
		"select %s from %s p where %s order by %s limit ?, ?",
		dProgramRows,
		m.table,
		strings.Join(conditions, " and "),
		orderBy,
	)
	args = append(args, offset, pageSize)

	var resp []*DProgram
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return resp, nil
}

func buildProgramBaseConditions(areaId int64) ([]string, []interface{}) {
	conditions := []string{
		"p.`status` = 1",
		"p.`program_status` = 1",
	}
	args := make([]interface{}, 0)

	if areaId > 0 {
		conditions = append(conditions, "p.`area_id` = ?")
		args = append(args, areaId)
	} else {
		conditions = append(conditions, "p.`prime` = 1")
	}

	return conditions, args
}

func buildProgramPageConditions(q *ProgramPageListQuery) ([]string, []interface{}) {
	conditions, args := buildProgramBaseConditions(q.AreaId)

	if q.ParentProgramCategoryId > 0 {
		conditions = append(conditions, "p.`parent_program_category_id` = ?")
		args = append(args, q.ParentProgramCategoryId)
	}
	if q.ProgramCategoryId > 0 {
		conditions = append(conditions, "p.`program_category_id` = ?")
		args = append(args, q.ProgramCategoryId)
	}

	if q.TimeType > 0 && q.StartDateTime != "" && q.EndDateTime != "" {
		conditions = append(conditions,
			"exists (select 1 from `d_program_show_time` st where st.`program_id` = p.`id` and st.`status` = 1 and st.`show_day_time` >= ? and st.`show_day_time` <= ?)",
		)
		args = append(args, q.StartDateTime, q.EndDateTime)
	}

	return conditions, args
}

func buildProgramPageOrderBy(tp int64) string {
	switch tp {
	case 2:
		return "p.`high_heat` desc, p.`id` desc"
	case 3:
		return "(select min(st.`show_time`) from `d_program_show_time` st where st.`program_id` = p.`id` and st.`status` = 1) asc, p.`id` asc"
	case 4:
		return "p.`issue_time` asc, p.`id` asc"
	default:
		return "p.`id` desc"
	}
}
