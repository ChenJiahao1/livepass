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
		cachedConn *sqlc.CachedConn
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
	cachedConn := sqlc.NewConn(conn, c, opts...)
	return &customDProgramShowTimeModel{
		defaultDProgramShowTimeModel: newDProgramShowTimeModel(conn),
		cachedConn:                   &cachedConn,
	}
}

func (m *customDProgramShowTimeModel) withSession(session sqlx.Session) DProgramShowTimeModel {
	sessionConn := sqlx.NewSqlConnFromSession(session)
	if m.cachedConn == nil {
		return &customDProgramShowTimeModel{
			defaultDProgramShowTimeModel: newDProgramShowTimeModel(sessionConn),
		}
	}

	cachedConn := m.cachedConn.WithSession(session)
	return &customDProgramShowTimeModel{
		defaultDProgramShowTimeModel: newDProgramShowTimeModel(sessionConn),
		cachedConn:                   &cachedConn,
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

func (m *customDProgramShowTimeModel) Delete(ctx context.Context, id int64) error {
	if m.cachedConn == nil {
		return m.defaultDProgramShowTimeModel.Delete(ctx, id)
	}

	dProgramShowTimeIdKey := fmt.Sprintf("%s%v", cacheDProgramShowTimeIdPrefix, id)
	_, err := m.cachedConn.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		query := fmt.Sprintf("delete from %s where `id` = ?", m.table)
		return conn.ExecCtx(ctx, query, id)
	}, dProgramShowTimeIdKey)
	return err
}

func (m *customDProgramShowTimeModel) FindOne(ctx context.Context, id int64) (*DProgramShowTime, error) {
	if m.cachedConn == nil {
		return m.defaultDProgramShowTimeModel.FindOne(ctx, id)
	}

	var resp DProgramShowTime
	dProgramShowTimeIdKey := fmt.Sprintf("%s%v", cacheDProgramShowTimeIdPrefix, id)
	err := m.cachedConn.QueryRowCtx(ctx, &resp, dProgramShowTimeIdKey, func(ctx context.Context, conn sqlx.SqlConn, v any) error {
		query := fmt.Sprintf("select %s from %s where `id` = ? limit 1", dProgramShowTimeRows, m.table)
		return conn.QueryRowCtx(ctx, v, query, id)
	})
	switch err {
	case nil:
		return &resp, nil
	case sqlc.ErrNotFound, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customDProgramShowTimeModel) Insert(ctx context.Context, data *DProgramShowTime) (sql.Result, error) {
	if m.cachedConn == nil {
		return m.defaultDProgramShowTimeModel.Insert(ctx, data)
	}

	dProgramShowTimeIdKey := fmt.Sprintf("%s%v", cacheDProgramShowTimeIdPrefix, data.Id)
	return m.cachedConn.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", m.table, dProgramShowTimeRowsExpectAutoSet)
		return conn.ExecCtx(ctx, query, data.Id, data.ProgramId, data.ShowTime, data.ShowDayTime, data.ShowWeekTime, data.RushSaleOpenTime, data.RushSaleEndTime, data.ShowEndTime, data.InventoryPreheatStatus, data.EditTime, data.Status)
	}, dProgramShowTimeIdKey)
}

func (m *customDProgramShowTimeModel) Update(ctx context.Context, data *DProgramShowTime) error {
	if m.cachedConn == nil {
		return m.defaultDProgramShowTimeModel.Update(ctx, data)
	}

	dProgramShowTimeIdKey := fmt.Sprintf("%s%v", cacheDProgramShowTimeIdPrefix, data.Id)
	_, err := m.cachedConn.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		query := fmt.Sprintf("update %s set %s where `id` = ?", m.table, dProgramShowTimeRowsWithPlaceHolder)
		return conn.ExecCtx(ctx, query, data.ProgramId, data.ShowTime, data.ShowDayTime, data.ShowWeekTime, data.RushSaleOpenTime, data.RushSaleEndTime, data.ShowEndTime, data.InventoryPreheatStatus, data.EditTime, data.Status, data.Id)
	}, dProgramShowTimeIdKey)
	return err
}

func (m *customDProgramShowTimeModel) FindFirstByProgramId(ctx context.Context, programId int64) (*DProgramShowTime, error) {
	var resp DProgramShowTime
	var err error

	if m.cachedConn != nil {
		cacheKey := ProgramFirstShowTimeCacheKey(programId)
		err = m.cachedConn.QueryRowCtx(ctx, &resp, cacheKey, func(ctx context.Context, conn sqlx.SqlConn, v any) error {
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
