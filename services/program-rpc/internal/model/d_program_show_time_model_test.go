package model

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/zeromicro/go-zero/core/stores/cache"
	gzredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestDProgramShowTimeModelCacheConstructors(t *testing.T) {
	conn := &stubProgramShowTimeSqlConn{}

	uncached := NewDProgramShowTimeModel(conn)
	if containsFieldOfType(reflect.ValueOf(uncached), reflect.TypeOf(sqlc.CachedConn{})) {
		t.Fatal("expected NewDProgramShowTimeModel to keep uncached shape")
	}

	cached := NewCachedDProgramShowTimeModel(conn, testProgramShowTimeCacheConf())
	if !containsFieldOfType(reflect.ValueOf(cached), reflect.TypeOf(sqlc.CachedConn{})) {
		t.Fatal("expected NewCachedDProgramShowTimeModel to expose cached shape")
	}

	sessionModel := cached.withSession(&stubProgramShowTimeSession{})
	if !containsFieldOfType(reflect.ValueOf(sessionModel), reflect.TypeOf(sqlc.CachedConn{})) {
		t.Fatal("expected cached withSession to preserve cached shape")
	}
}

func testProgramShowTimeCacheConf() cache.CacheConf {
	return cache.CacheConf{
		{
			RedisConf: gzredis.RedisConf{
				Host: "127.0.0.1:6379",
				Type: "node",
			},
			Weight: 100,
		},
	}
}

func containsFieldOfType(value reflect.Value, target reflect.Type) bool {
	if !value.IsValid() {
		return false
	}

	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return false
		}
		return containsFieldOfType(value.Elem(), target)
	case reflect.Struct:
		if value.Type() == target {
			return true
		}
		for i := 0; i < value.NumField(); i++ {
			if containsFieldOfType(value.Field(i), target) {
				return true
			}
		}
	}

	return false
}

type stubProgramShowTimeStmtSession struct{}

func (s *stubProgramShowTimeStmtSession) Close() error { return nil }

func (s *stubProgramShowTimeStmtSession) Exec(args ...any) (sql.Result, error) {
	return nil, nil
}

func (s *stubProgramShowTimeStmtSession) ExecCtx(context.Context, ...any) (sql.Result, error) {
	return nil, nil
}

func (s *stubProgramShowTimeStmtSession) QueryRow(any, ...any) error { return nil }

func (s *stubProgramShowTimeStmtSession) QueryRowCtx(context.Context, any, ...any) error {
	return nil
}

func (s *stubProgramShowTimeStmtSession) QueryRowPartial(any, ...any) error { return nil }

func (s *stubProgramShowTimeStmtSession) QueryRowPartialCtx(context.Context, any, ...any) error {
	return nil
}

func (s *stubProgramShowTimeStmtSession) QueryRows(any, ...any) error { return nil }

func (s *stubProgramShowTimeStmtSession) QueryRowsCtx(context.Context, any, ...any) error {
	return nil
}

func (s *stubProgramShowTimeStmtSession) QueryRowsPartial(any, ...any) error { return nil }

func (s *stubProgramShowTimeStmtSession) QueryRowsPartialCtx(context.Context, any, ...any) error {
	return nil
}

type stubProgramShowTimeSession struct{}

func (s *stubProgramShowTimeSession) Exec(string, ...any) (sql.Result, error) { return nil, nil }

func (s *stubProgramShowTimeSession) ExecCtx(context.Context, string, ...any) (sql.Result, error) {
	return nil, nil
}

func (s *stubProgramShowTimeSession) Prepare(string) (sqlx.StmtSession, error) {
	return &stubProgramShowTimeStmtSession{}, nil
}

func (s *stubProgramShowTimeSession) PrepareCtx(context.Context, string) (sqlx.StmtSession, error) {
	return &stubProgramShowTimeStmtSession{}, nil
}

func (s *stubProgramShowTimeSession) QueryRow(any, string, ...any) error { return nil }

func (s *stubProgramShowTimeSession) QueryRowCtx(context.Context, any, string, ...any) error {
	return nil
}

func (s *stubProgramShowTimeSession) QueryRowPartial(any, string, ...any) error { return nil }

func (s *stubProgramShowTimeSession) QueryRowPartialCtx(context.Context, any, string, ...any) error {
	return nil
}

func (s *stubProgramShowTimeSession) QueryRows(any, string, ...any) error { return nil }

func (s *stubProgramShowTimeSession) QueryRowsCtx(context.Context, any, string, ...any) error {
	return nil
}

func (s *stubProgramShowTimeSession) QueryRowsPartial(any, string, ...any) error { return nil }

func (s *stubProgramShowTimeSession) QueryRowsPartialCtx(context.Context, any, string, ...any) error {
	return nil
}

type stubProgramShowTimeSqlConn struct {
	stubProgramShowTimeSession
}

func (c *stubProgramShowTimeSqlConn) RawDB() (*sql.DB, error) { return nil, nil }

func (c *stubProgramShowTimeSqlConn) Transact(func(sqlx.Session) error) error { return nil }

func (c *stubProgramShowTimeSqlConn) TransactCtx(context.Context, func(context.Context, sqlx.Session) error) error {
	return nil
}

var _ sqlx.StmtSession = (*stubProgramShowTimeStmtSession)(nil)
var _ sqlx.Session = (*stubProgramShowTimeSession)(nil)
var _ sqlx.SqlConn = (*stubProgramShowTimeSqlConn)(nil)
