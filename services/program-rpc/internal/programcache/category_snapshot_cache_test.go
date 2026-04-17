package programcache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"livepass/services/program-rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestCategorySnapshotCacheDeduplicatesConcurrentLoads(t *testing.T) {
	fakeConn := &stubCategorySqlConn{
		rows: []*model.DProgramCategory{
			{Id: 1, Name: "演出", Type: 1, Status: 1},
			{Id: 2, ParentId: 1, Name: "音乐会", Type: 2, Status: 1},
		},
	}
	categoryModel := model.NewDProgramCategoryModel(fakeConn)

	cache, err := NewCategorySnapshotCache(categoryModel, 20*time.Second)
	if err != nil {
		t.Fatalf("NewCategorySnapshotCache returned error: %v", err)
	}

	start := make(chan struct{})
	resultCh := make(chan error, 20)
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			categories, err := cache.GetAll(context.Background())
			if err != nil {
				resultCh <- err
				return
			}
			if len(categories) != 2 {
				resultCh <- fmt.Errorf("unexpected categories length: %d", len(categories))
				return
			}
			resultCh <- nil
		}()
	}

	close(start)
	wg.Wait()
	close(resultCh)

	for err := range resultCh {
		if err != nil {
			t.Fatalf("concurrent GetAll returned error: %v", err)
		}
	}

	if calls := atomic.LoadInt32(&fakeConn.calls); calls != 1 {
		t.Fatalf("expected model to be called once, got %d", calls)
	}
}

func TestCategorySnapshotCacheSharedLoadIgnoresCallerCancel(t *testing.T) {
	fakeConn := &cancelAwareCategorySqlConn{
		rows: []*model.DProgramCategory{
			{Id: 11, Name: "演出", Type: 1, Status: 1},
			{Id: 12, ParentId: 11, Name: "戏剧", Type: 2, Status: 1},
		},
		started: make(chan struct{}),
	}
	categoryModel := model.NewDProgramCategoryModel(fakeConn)

	cache, err := NewCategorySnapshotCache(categoryModel, 20*time.Second)
	if err != nil {
		t.Fatalf("NewCategorySnapshotCache returned error: %v", err)
	}

	cancelCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 2)
	respCh := make(chan []*model.DProgramCategory, 1)

	go func() {
		_, err := cache.GetAll(cancelCtx)
		errCh <- err
	}()

	<-fakeConn.started

	go func() {
		categories, err := cache.GetAll(context.Background())
		if err != nil {
			errCh <- err
			return
		}
		respCh <- categories
		errCh <- nil
	}()

	var firstErr error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if !errors.Is(firstErr, context.DeadlineExceeded) {
		t.Fatalf("expected caller cancel to return context deadline exceeded, got %v", firstErr)
	}

	select {
	case categories := <-respCh:
		if len(categories) != 2 {
			t.Fatalf("unexpected categories length: %d", len(categories))
		}
	default:
		t.Fatalf("expected successful response for non-canceled caller")
	}

	if calls := atomic.LoadInt32(&fakeConn.calls); calls != 1 {
		t.Fatalf("expected model to be called once, got %d", calls)
	}
}

type stubCategorySqlConn struct {
	rows  []*model.DProgramCategory
	calls int32
}

func (s *stubCategorySqlConn) Exec(_ string, _ ...any) (sql.Result, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCategorySqlConn) ExecCtx(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCategorySqlConn) Prepare(_ string) (sqlx.StmtSession, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCategorySqlConn) PrepareCtx(_ context.Context, _ string) (sqlx.StmtSession, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCategorySqlConn) QueryRow(_ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *stubCategorySqlConn) QueryRowCtx(_ context.Context, _ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *stubCategorySqlConn) QueryRowPartial(_ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *stubCategorySqlConn) QueryRowPartialCtx(_ context.Context, _ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *stubCategorySqlConn) QueryRows(v any, _ string, _ ...any) error {
	return s.fillRows(v)
}

func (s *stubCategorySqlConn) QueryRowsCtx(_ context.Context, v any, _ string, _ ...any) error {
	return s.fillRows(v)
}

func (s *stubCategorySqlConn) QueryRowsPartial(v any, _ string, _ ...any) error {
	return s.fillRows(v)
}

func (s *stubCategorySqlConn) QueryRowsPartialCtx(_ context.Context, v any, _ string, _ ...any) error {
	return s.fillRows(v)
}

func (s *stubCategorySqlConn) RawDB() (*sql.DB, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCategorySqlConn) Transact(_ func(sqlx.Session) error) error {
	return errors.New("not implemented")
}

func (s *stubCategorySqlConn) TransactCtx(_ context.Context, _ func(context.Context, sqlx.Session) error) error {
	return errors.New("not implemented")
}

func (s *stubCategorySqlConn) fillRows(v any) error {
	atomic.AddInt32(&s.calls, 1)
	time.Sleep(50 * time.Millisecond)
	dest, ok := v.(*[]*model.DProgramCategory)
	if !ok {
		return fmt.Errorf("unexpected dest type %T", v)
	}
	*dest = cloneProgramCategories(s.rows)
	return nil
}

type cancelAwareCategorySqlConn struct {
	rows    []*model.DProgramCategory
	calls   int32
	started chan struct{}
}

func (s *cancelAwareCategorySqlConn) Exec(_ string, _ ...any) (sql.Result, error) {
	return nil, errors.New("not implemented")
}

func (s *cancelAwareCategorySqlConn) ExecCtx(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, errors.New("not implemented")
}

func (s *cancelAwareCategorySqlConn) Prepare(_ string) (sqlx.StmtSession, error) {
	return nil, errors.New("not implemented")
}

func (s *cancelAwareCategorySqlConn) PrepareCtx(_ context.Context, _ string) (sqlx.StmtSession, error) {
	return nil, errors.New("not implemented")
}

func (s *cancelAwareCategorySqlConn) QueryRow(_ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *cancelAwareCategorySqlConn) QueryRowCtx(_ context.Context, _ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *cancelAwareCategorySqlConn) QueryRowPartial(_ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *cancelAwareCategorySqlConn) QueryRowPartialCtx(_ context.Context, _ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *cancelAwareCategorySqlConn) QueryRows(v any, _ string, _ ...any) error {
	return s.fillRows(context.Background(), v)
}

func (s *cancelAwareCategorySqlConn) QueryRowsCtx(ctx context.Context, v any, _ string, _ ...any) error {
	return s.fillRows(ctx, v)
}

func (s *cancelAwareCategorySqlConn) QueryRowsPartial(v any, _ string, _ ...any) error {
	return s.fillRows(context.Background(), v)
}

func (s *cancelAwareCategorySqlConn) QueryRowsPartialCtx(ctx context.Context, v any, _ string, _ ...any) error {
	return s.fillRows(ctx, v)
}

func (s *cancelAwareCategorySqlConn) RawDB() (*sql.DB, error) {
	return nil, errors.New("not implemented")
}

func (s *cancelAwareCategorySqlConn) Transact(_ func(sqlx.Session) error) error {
	return errors.New("not implemented")
}

func (s *cancelAwareCategorySqlConn) TransactCtx(_ context.Context, _ func(context.Context, sqlx.Session) error) error {
	return errors.New("not implemented")
}

func (s *cancelAwareCategorySqlConn) fillRows(ctx context.Context, v any) error {
	if atomic.AddInt32(&s.calls, 1) == 1 && s.started != nil {
		close(s.started)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(50 * time.Millisecond):
	}
	dest, ok := v.(*[]*model.DProgramCategory)
	if !ok {
		return fmt.Errorf("unexpected dest type %T", v)
	}
	*dest = cloneProgramCategories(s.rows)
	return nil
}
