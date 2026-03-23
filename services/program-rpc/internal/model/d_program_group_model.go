package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
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

// NewCachedDProgramGroupModel returns a cached model for the database table.
func NewCachedDProgramGroupModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) DProgramGroupModel {
	return &customDProgramGroupModel{
		defaultDProgramGroupModel: newCachedDProgramGroupModel(conn, c, opts...),
	}
}

func (m *customDProgramGroupModel) withSession(session sqlx.Session) DProgramGroupModel {
	return &customDProgramGroupModel{
		defaultDProgramGroupModel: m.defaultDProgramGroupModel.withSession(session),
	}
}

func (m *customDProgramGroupModel) FindOne(ctx context.Context, id int64) (*DProgramGroup, error) {
	var resp DProgramGroup
	var err error

	if m.cached {
		dProgramGroupIdKey := fmt.Sprintf("%s%v", cacheDProgramGroupIdPrefix, id)
		err = m.QueryRowCtx(ctx, &resp, dProgramGroupIdKey, func(ctx context.Context, conn sqlx.SqlConn, v any) error {
			query := fmt.Sprintf("select %s from %s where `id` = ? and `status` = 1 limit 1", dProgramGroupRows, m.table)
			return conn.QueryRowCtx(ctx, v, query, id)
		})
	} else {
		query := fmt.Sprintf("select %s from %s where `id` = ? and `status` = 1 limit 1", dProgramGroupRows, m.table)
		err = m.conn.QueryRowCtx(ctx, &resp, query, id)
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
