package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DOrderRouteLegacyModel = (*customDOrderRouteLegacyModel)(nil)

type (
	// DOrderRouteLegacyModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDOrderRouteLegacyModel.
	DOrderRouteLegacyModel interface {
		dOrderRouteLegacyModel
		withSession(session sqlx.Session) DOrderRouteLegacyModel
		InsertWithSession(ctx context.Context, session sqlx.Session, data *DOrderRouteLegacy) (sql.Result, error)
	}

	customDOrderRouteLegacyModel struct {
		*defaultDOrderRouteLegacyModel
	}
)

// NewDOrderRouteLegacyModel returns a model for the database table.
func NewDOrderRouteLegacyModel(conn sqlx.SqlConn) DOrderRouteLegacyModel {
	return &customDOrderRouteLegacyModel{
		defaultDOrderRouteLegacyModel: newDOrderRouteLegacyModel(conn),
	}
}

func NewDOrderRouteLegacyModelWithTable(conn sqlx.SqlConn, table string) DOrderRouteLegacyModel {
	m := newDOrderRouteLegacyModel(conn)
	m.table = normalizeTableName(table)
	return &customDOrderRouteLegacyModel{
		defaultDOrderRouteLegacyModel: m,
	}
}

func (m *customDOrderRouteLegacyModel) withSession(session sqlx.Session) DOrderRouteLegacyModel {
	return NewDOrderRouteLegacyModelWithTable(sqlx.NewSqlConnFromSession(session), rawTableName(m.table))
}

func (m *customDOrderRouteLegacyModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *DOrderRouteLegacy) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`order_number`, `user_id`, `logic_slot`, `route_version`, `status`, `create_time`, `edit_time`) values (?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return m.withSession(session).(*customDOrderRouteLegacyModel).conn.ExecCtx(
		ctx,
		query,
		data.OrderNumber,
		data.UserId,
		data.LogicSlot,
		data.RouteVersion,
		data.Status,
		data.CreateTime,
		data.EditTime,
	)
}
