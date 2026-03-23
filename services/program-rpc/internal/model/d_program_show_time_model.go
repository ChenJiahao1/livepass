package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
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

// NewCachedDProgramShowTimeModel returns a cached model for the database table.
func NewCachedDProgramShowTimeModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) DProgramShowTimeModel {
	return &customDProgramShowTimeModel{
		defaultDProgramShowTimeModel: newCachedDProgramShowTimeModel(conn, c, opts...),
	}
}

func (m *customDProgramShowTimeModel) withSession(session sqlx.Session) DProgramShowTimeModel {
	return &customDProgramShowTimeModel{
		defaultDProgramShowTimeModel: m.defaultDProgramShowTimeModel.withSession(session),
	}
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
	var resp DProgramShowTime
	var err error

	if m.cached {
		cacheKey := fmt.Sprintf("cache:dProgramShowTime:first:programId:%d", programId)
		err = m.QueryRowCtx(ctx, &resp, cacheKey, func(ctx context.Context, conn sqlx.SqlConn, v any) error {
			query := fmt.Sprintf(
				"select %s from %s where `status` = 1 and `program_id` = ? order by `show_time` asc limit 1",
				dProgramShowTimeRows,
				m.table,
			)
			return conn.QueryRowCtx(ctx, v, query, programId)
		})
	} else {
		query := fmt.Sprintf(
			"select %s from %s where `status` = 1 and `program_id` = ? order by `show_time` asc limit 1",
			dProgramShowTimeRows,
			m.table,
		)
		err = m.conn.QueryRowCtx(ctx, &resp, query, programId)
	}

	switch err {
	case nil:
		return &resp, nil
	case sqlc.ErrNotFound:
		return nil, ErrNotFound
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
