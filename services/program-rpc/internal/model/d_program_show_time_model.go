package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DProgramShowTimeModel = (*customDProgramShowTimeModel)(nil)

type (
	// DProgramShowTimeModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDProgramShowTimeModel.
	DProgramShowTimeModel interface {
		dProgramShowTimeModel
		withSession(session sqlx.Session) DProgramShowTimeModel
		FindByProgramIds(ctx context.Context, programIds []int64) ([]*DProgramShowTime, error)
		FindFirstByProgramId(ctx context.Context, programId int64) (*DProgramShowTime, error)
	}

	customDProgramShowTimeModel struct {
		*defaultDProgramShowTimeModel
	}
)

// NewDProgramShowTimeModel returns a model for the database table.
func NewDProgramShowTimeModel(conn sqlx.SqlConn) DProgramShowTimeModel {
	return &customDProgramShowTimeModel{
		defaultDProgramShowTimeModel: newDProgramShowTimeModel(conn),
	}
}

func (m *customDProgramShowTimeModel) withSession(session sqlx.Session) DProgramShowTimeModel {
	return NewDProgramShowTimeModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDProgramShowTimeModel) FindByProgramIds(ctx context.Context, programIds []int64) ([]*DProgramShowTime, error) {
	if len(programIds) == 0 {
		return []*DProgramShowTime{}, nil
	}

	inClause, args := buildInt64InClause(programIds)
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `program_id` in (%s) order by `program_id` asc, `show_time` asc",
		dProgramShowTimeRows,
		m.table,
		inClause,
	)

	var resp []*DProgramShowTime
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return resp, nil
}

func (m *customDProgramShowTimeModel) FindFirstByProgramId(ctx context.Context, programId int64) (*DProgramShowTime, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `program_id` = ? order by `show_time` asc limit 1",
		dProgramShowTimeRows,
		m.table,
	)
	var resp DProgramShowTime
	err := m.conn.QueryRowCtx(ctx, &resp, query, programId)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
