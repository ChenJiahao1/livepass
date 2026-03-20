package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DTicketUserModel = (*customDTicketUserModel)(nil)

type (
	// DTicketUserModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDTicketUserModel.
	DTicketUserModel interface {
		dTicketUserModel
		withSession(session sqlx.Session) DTicketUserModel
		FindByUserId(ctx context.Context, userId int64) ([]*DTicketUser, error)
		FindOneByUserIdAndIdTypeAndIdNumber(ctx context.Context, userId int64, idType int64, idNumber string) (*DTicketUser, error)
	}

	customDTicketUserModel struct {
		*defaultDTicketUserModel
	}
)

// NewDTicketUserModel returns a model for the database table.
func NewDTicketUserModel(conn sqlx.SqlConn) DTicketUserModel {
	return &customDTicketUserModel{
		defaultDTicketUserModel: newDTicketUserModel(conn),
	}
}

func (m *customDTicketUserModel) withSession(session sqlx.Session) DTicketUserModel {
	return NewDTicketUserModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDTicketUserModel) FindByUserId(ctx context.Context, userId int64) ([]*DTicketUser, error) {
	query := fmt.Sprintf("select %s from %s where `user_id` = ? and `status` = 1 order by `id` asc", dTicketUserRows, m.table)
	var resp []*DTicketUser
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, userId); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return resp, nil
}

func (m *customDTicketUserModel) FindOneByUserIdAndIdTypeAndIdNumber(ctx context.Context, userId int64, idType int64, idNumber string) (*DTicketUser, error) {
	query := fmt.Sprintf(
		"select %s from %s where `user_id` = ? and `id_type` = ? and `id_number` = ? and `status` = 1 limit 1",
		dTicketUserRows,
		m.table,
	)
	var resp DTicketUser
	err := m.conn.QueryRowCtx(ctx, &resp, query, userId, idType, idNumber)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
