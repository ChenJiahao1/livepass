package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DTicketCategoryModel = (*customDTicketCategoryModel)(nil)

type (
	// DTicketCategoryModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDTicketCategoryModel.
	DTicketCategoryModel interface {
		dTicketCategoryModel
		withSession(session sqlx.Session) DTicketCategoryModel
		InsertWithCreateTime(ctx context.Context, data *DTicketCategory) (sql.Result, error)
		FindByProgramIds(ctx context.Context, programIds []int64) ([]*DTicketCategory, error)
		FindByProgramId(ctx context.Context, programId int64) ([]*DTicketCategory, error)
		FindByShowTimeId(ctx context.Context, showTimeId int64) ([]*DTicketCategory, error)
		FindPriceAggregateByProgramIds(ctx context.Context, programIds []int64) ([]*TicketCategoryPriceAggregate, error)
	}

	customDTicketCategoryModel struct {
		*defaultDTicketCategoryModel
	}

	TicketCategoryPriceAggregate struct {
		ProgramId int64   `db:"program_id"`
		MinPrice  float64 `db:"min_price"`
		MaxPrice  float64 `db:"max_price"`
	}
)

// NewDTicketCategoryModel returns a model for the database table.
func NewDTicketCategoryModel(conn sqlx.SqlConn) DTicketCategoryModel {
	return &customDTicketCategoryModel{
		defaultDTicketCategoryModel: newDTicketCategoryModel(conn),
	}
}

func (m *customDTicketCategoryModel) withSession(session sqlx.Session) DTicketCategoryModel {
	return NewDTicketCategoryModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDTicketCategoryModel) InsertWithCreateTime(ctx context.Context, data *DTicketCategory) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`id`, `program_id`, `show_time_id`, `introduce`, `price`, `total_number`, `remain_number`, `create_time`, `edit_time`, `status`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return m.conn.ExecCtx(
		ctx,
		query,
		data.Id,
		data.ProgramId,
		data.ShowTimeId,
		data.Introduce,
		data.Price,
		data.TotalNumber,
		data.RemainNumber,
		data.CreateTime,
		data.EditTime,
		data.Status,
	)
}

func (m *customDTicketCategoryModel) FindByProgramIds(ctx context.Context, programIds []int64) ([]*DTicketCategory, error) {
	if len(programIds) == 0 {
		return []*DTicketCategory{}, nil
	}

	inClause, args := buildInt64InClause(programIds)
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `program_id` in (%s) order by `program_id` asc, `price` asc, `id` asc",
		dTicketCategoryRows,
		m.table,
		inClause,
	)
	var resp []*DTicketCategory
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return resp, nil
}

func (m *customDTicketCategoryModel) FindByProgramId(ctx context.Context, programId int64) ([]*DTicketCategory, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `program_id` = ? order by `price` asc, `id` asc",
		dTicketCategoryRows,
		m.table,
	)
	var resp []*DTicketCategory
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, programId); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return resp, nil
}

func (m *customDTicketCategoryModel) FindByShowTimeId(ctx context.Context, showTimeId int64) ([]*DTicketCategory, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `show_time_id` = ? order by `price` asc, `id` asc",
		dTicketCategoryRows,
		m.table,
	)
	var resp []*DTicketCategory
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, showTimeId); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return resp, nil
}

func (m *customDTicketCategoryModel) FindPriceAggregateByProgramIds(ctx context.Context, programIds []int64) ([]*TicketCategoryPriceAggregate, error) {
	if len(programIds) == 0 {
		return []*TicketCategoryPriceAggregate{}, nil
	}

	inClause, args := buildInt64InClause(programIds)
	query := fmt.Sprintf(
		"select `program_id`, min(`price`) as `min_price`, max(`price`) as `max_price` from %s where `status` = 1 and `program_id` in (%s) group by `program_id`",
		m.table,
		inClause,
	)
	var resp []*TicketCategoryPriceAggregate
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return resp, nil
}
