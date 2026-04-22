package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"livepass/pkg/delaytask"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestDispatchableStatusesExcludeProcessed(t *testing.T) {
	statuses := DispatchableStatuses()
	for _, status := range statuses {
		if status == delaytask.OutboxTaskStatusProcessed {
			t.Fatalf("DispatchableStatuses() includes processed")
		}
	}
	if len(statuses) != 3 {
		t.Fatalf("DispatchableStatuses() len = %d, want 3", len(statuses))
	}
}

func TestListDispatchableByTaskTypeScansEveryShardWithoutLimit(t *testing.T) {
	executeAt := time.Date(2026, time.April, 22, 10, 0, 0, 0, time.Local)
	firstConn := &listDispatchableConn{
		rows: []delayTaskOutboxRow{{
			ID:         101,
			TaskType:   "order.close_timeout",
			TaskKey:    "order.close_timeout:101",
			Payload:    `{"orderNumber":101}`,
			ExecuteAt:  executeAt,
			TaskStatus: delaytask.OutboxTaskStatusPending,
		}},
	}
	secondConn := &listDispatchableConn{
		rows: []delayTaskOutboxRow{{
			ID:         201,
			TaskType:   "order.close_timeout",
			TaskKey:    "order.close_timeout:201",
			Payload:    `{"orderNumber":201}`,
			ExecuteAt:  executeAt,
			TaskStatus: delaytask.OutboxTaskStatusPending,
		}},
	}

	store := NewMysqlStore(map[string]sqlx.SqlConn{
		"order-db-0": firstConn,
		"order-db-1": secondConn,
	})
	tasks, err := store.ListDispatchableByTaskType(context.Background(), "order.close_timeout")
	if err != nil {
		t.Fatalf("ListDispatchableByTaskType() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("ListDispatchableByTaskType() len = %d, want 2", len(tasks))
	}
	assertListQueryHasNoLimit(t, firstConn)
	assertListQueryHasNoLimit(t, secondConn)
}

func assertListQueryHasNoLimit(t *testing.T, conn *listDispatchableConn) {
	t.Helper()

	if conn.query == "" {
		t.Fatalf("expected shard to be queried")
	}
	if strings.Contains(strings.ToUpper(conn.query), "LIMIT") {
		t.Fatalf("expected query without LIMIT, got %s", conn.query)
	}
	if len(conn.args) != 1 {
		t.Fatalf("query args len = %d, want 1", len(conn.args))
	}
	if conn.args[0] != "order.close_timeout" {
		t.Fatalf("task type arg = %v, want order.close_timeout", conn.args[0])
	}
}

type listDispatchableConn struct {
	rows  []delayTaskOutboxRow
	query string
	args  []any
}

func (conn *listDispatchableConn) Exec(string, ...any) (sql.Result, error) {
	return nil, fmt.Errorf("Exec should not be called")
}

func (conn *listDispatchableConn) ExecCtx(context.Context, string, ...any) (sql.Result, error) {
	return nil, fmt.Errorf("ExecCtx should not be called")
}

func (conn *listDispatchableConn) Prepare(string) (sqlx.StmtSession, error) {
	return nil, fmt.Errorf("Prepare should not be called")
}

func (conn *listDispatchableConn) PrepareCtx(context.Context, string) (sqlx.StmtSession, error) {
	return nil, fmt.Errorf("PrepareCtx should not be called")
}

func (conn *listDispatchableConn) QueryRow(any, string, ...any) error {
	return fmt.Errorf("QueryRow should not be called")
}

func (conn *listDispatchableConn) QueryRowCtx(context.Context, any, string, ...any) error {
	return fmt.Errorf("QueryRowCtx should not be called")
}

func (conn *listDispatchableConn) QueryRowPartial(any, string, ...any) error {
	return fmt.Errorf("QueryRowPartial should not be called")
}

func (conn *listDispatchableConn) QueryRowPartialCtx(context.Context, any, string, ...any) error {
	return fmt.Errorf("QueryRowPartialCtx should not be called")
}

func (conn *listDispatchableConn) QueryRows(any, string, ...any) error {
	return fmt.Errorf("QueryRows should not be called")
}

func (conn *listDispatchableConn) QueryRowsCtx(_ context.Context, target any, query string, args ...any) error {
	conn.query = query
	conn.args = args

	rows, ok := target.(*[]delayTaskOutboxRow)
	if !ok {
		return fmt.Errorf("unexpected query target %T", target)
	}
	*rows = append(*rows, conn.rows...)
	return nil
}

func (conn *listDispatchableConn) QueryRowsPartial(any, string, ...any) error {
	return fmt.Errorf("QueryRowsPartial should not be called")
}

func (conn *listDispatchableConn) QueryRowsPartialCtx(context.Context, any, string, ...any) error {
	return fmt.Errorf("QueryRowsPartialCtx should not be called")
}

func (conn *listDispatchableConn) RawDB() (*sql.DB, error) {
	return nil, fmt.Errorf("RawDB should not be called")
}

func (conn *listDispatchableConn) Transact(func(sqlx.Session) error) error {
	return fmt.Errorf("Transact should not be called")
}

func (conn *listDispatchableConn) TransactCtx(context.Context, func(context.Context, sqlx.Session) error) error {
	return fmt.Errorf("TransactCtx should not be called")
}
