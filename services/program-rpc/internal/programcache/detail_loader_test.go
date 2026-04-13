package programcache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"damai-go/services/program-rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestNewDetailLoaderUsesDepsStruct(t *testing.T) {
	deps := DetailLoaderDeps{
		ProgramModel:          model.NewDProgramModel(&noopDetailLoaderSqlConn{}),
		ProgramShowTimeModel:  model.NewDProgramShowTimeModel(&noopDetailLoaderSqlConn{}),
		ProgramGroupModel:     model.NewDProgramGroupModel(&noopDetailLoaderSqlConn{}),
		CategorySnapshotCache: mustNewCategorySnapshotCacheForTest(t),
		TicketCategoryModel:   model.NewDTicketCategoryModel(&noopDetailLoaderSqlConn{}),
	}

	loader := NewDetailLoader(deps)

	if loader == nil {
		t.Fatal("expected loader")
	}
}

func TestDetailLoaderLoadKeepsTicketCategoryAggregationBehavior(t *testing.T) {
	t.Run("returns empty ticket categories when model misses", func(t *testing.T) {
		loader := newDetailLoaderForTest(t, nil, sqlx.ErrNotFound)

		resp, err := loader.Load(context.Background(), 1001)
		if err != nil {
			t.Fatalf("load returned error: %v", err)
		}
		if resp == nil {
			t.Fatal("expected detail response")
		}
		if len(resp.TicketCategoryVoList) != 0 {
			t.Fatalf("expected empty ticket categories, got %+v", resp.TicketCategoryVoList)
		}
	})

	t.Run("maps ticket categories when model hits", func(t *testing.T) {
		loader := newDetailLoaderForTest(t, []*model.DTicketCategory{
			{Id: 501, ProgramId: 1001, ShowTimeId: 3001, Introduce: "看台", Price: 380},
			{Id: 502, ProgramId: 1001, ShowTimeId: 3001, Introduce: "内场", Price: 680},
		}, nil)

		resp, err := loader.Load(context.Background(), 1001)
		if err != nil {
			t.Fatalf("load returned error: %v", err)
		}
		if resp == nil {
			t.Fatal("expected detail response")
		}
		if resp.ProgramCategoryName != "音乐会" {
			t.Fatalf("expected program category name 音乐会, got %q", resp.ProgramCategoryName)
		}
		if len(resp.TicketCategoryVoList) != 2 {
			t.Fatalf("expected 2 ticket categories, got %+v", resp.TicketCategoryVoList)
		}
		if resp.TicketCategoryVoList[0].Id != 501 || resp.TicketCategoryVoList[0].Introduce != "看台" || resp.TicketCategoryVoList[0].Price != 380 {
			t.Fatalf("unexpected first ticket category: %+v", resp.TicketCategoryVoList[0])
		}
		if resp.TicketCategoryVoList[1].Id != 502 || resp.TicketCategoryVoList[1].Introduce != "内场" || resp.TicketCategoryVoList[1].Price != 680 {
			t.Fatalf("unexpected second ticket category: %+v", resp.TicketCategoryVoList[1])
		}
	})
}

func mustNewCategorySnapshotCacheForTest(t *testing.T) *CategorySnapshotCache {
	t.Helper()

	cache, err := NewCategorySnapshotCache(
		model.NewDProgramCategoryModel(&stubCategorySqlConn{
			rows: []*model.DProgramCategory{
				{Id: 1, ParentId: 0, Name: "演出", Type: 1, Status: 1},
				{Id: 2, ParentId: 1, Name: "音乐会", Type: 2, Status: 1},
			},
		}),
		time.Minute,
	)
	if err != nil {
		t.Fatalf("new category snapshot cache error: %v", err)
	}

	return cache
}

func newDetailLoaderForTest(t *testing.T, ticketCategories []*model.DTicketCategory, ticketCategoryErr error) *DetailLoader {
	t.Helper()

	conn := &detailLoaderQuerySqlConn{
		program: &model.DProgram{
			Id:                      1001,
			ProgramGroupId:          2001,
			AreaId:                  3100,
			ProgramCategoryId:       2,
			ParentProgramCategoryId: 1,
			Title:                   "测试演出",
			Detail:                  "测试详情",
			ProgramStatus:           1,
			Status:                  1,
		},
		firstShowTime: &model.DProgramShowTime{
			Id:           3001,
			ProgramId:    1001,
			ShowTime:     time.Date(2026, 4, 18, 19, 30, 0, 0, time.Local),
			ShowWeekTime: "周六",
			ShowDayTime: sql.NullTime{
				Time:  time.Date(2026, 4, 18, 0, 0, 0, 0, time.Local),
				Valid: true,
			},
			Status: 1,
		},
		group: &model.DProgramGroup{
			Id:             2001,
			ProgramJson:    `[{"programId":1001,"areaId":3100,"areaIdName":"上海"}]`,
			RecentShowTime: time.Date(2026, 4, 18, 19, 30, 0, 0, time.Local),
			Status:         1,
		},
		ticketCategories: ticketCategories,
		ticketCategoryErr: ticketCategoryErr,
	}

	return NewDetailLoader(DetailLoaderDeps{
		ProgramModel:          model.NewDProgramModel(conn),
		ProgramShowTimeModel:  model.NewDProgramShowTimeModel(conn),
		ProgramGroupModel:     model.NewDProgramGroupModel(conn),
		CategorySnapshotCache: mustNewCategorySnapshotCacheForTest(t),
		TicketCategoryModel:   model.NewDTicketCategoryModel(conn),
	})
}

type noopDetailLoaderSqlConn struct{}

func (s *noopDetailLoaderSqlConn) Exec(_ string, _ ...any) (sql.Result, error) {
	return nil, errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) ExecCtx(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) Prepare(_ string) (sqlx.StmtSession, error) {
	return nil, errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) PrepareCtx(_ context.Context, _ string) (sqlx.StmtSession, error) {
	return nil, errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) QueryRow(_ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) QueryRowCtx(_ context.Context, _ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) QueryRowPartial(_ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) QueryRowPartialCtx(_ context.Context, _ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) QueryRows(_ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) QueryRowsCtx(_ context.Context, _ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) QueryRowsPartial(_ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) QueryRowsPartialCtx(_ context.Context, _ any, _ string, _ ...any) error {
	return errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) RawDB() (*sql.DB, error) {
	return nil, errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) Transact(_ func(sqlx.Session) error) error {
	return errors.New("not implemented")
}

func (s *noopDetailLoaderSqlConn) TransactCtx(_ context.Context, _ func(context.Context, sqlx.Session) error) error {
	return errors.New("not implemented")
}

type detailLoaderQuerySqlConn struct {
	program           *model.DProgram
	firstShowTime     *model.DProgramShowTime
	group             *model.DProgramGroup
	ticketCategories  []*model.DTicketCategory
	ticketCategoryErr error
}

func (s *detailLoaderQuerySqlConn) Exec(_ string, _ ...any) (sql.Result, error) {
	return nil, errors.New("not implemented")
}

func (s *detailLoaderQuerySqlConn) ExecCtx(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, errors.New("not implemented")
}

func (s *detailLoaderQuerySqlConn) Prepare(_ string) (sqlx.StmtSession, error) {
	return nil, errors.New("not implemented")
}

func (s *detailLoaderQuerySqlConn) PrepareCtx(_ context.Context, _ string) (sqlx.StmtSession, error) {
	return nil, errors.New("not implemented")
}

func (s *detailLoaderQuerySqlConn) QueryRow(v any, query string, args ...any) error {
	return s.fillQueryRow(v, query, args...)
}

func (s *detailLoaderQuerySqlConn) QueryRowCtx(_ context.Context, v any, query string, args ...any) error {
	return s.fillQueryRow(v, query, args...)
}

func (s *detailLoaderQuerySqlConn) QueryRowPartial(v any, query string, args ...any) error {
	return s.fillQueryRow(v, query, args...)
}

func (s *detailLoaderQuerySqlConn) QueryRowPartialCtx(_ context.Context, v any, query string, args ...any) error {
	return s.fillQueryRow(v, query, args...)
}

func (s *detailLoaderQuerySqlConn) QueryRows(v any, query string, args ...any) error {
	return s.fillQueryRows(v, query, args...)
}

func (s *detailLoaderQuerySqlConn) QueryRowsCtx(_ context.Context, v any, query string, args ...any) error {
	return s.fillQueryRows(v, query, args...)
}

func (s *detailLoaderQuerySqlConn) QueryRowsPartial(v any, query string, args ...any) error {
	return s.fillQueryRows(v, query, args...)
}

func (s *detailLoaderQuerySqlConn) QueryRowsPartialCtx(_ context.Context, v any, query string, args ...any) error {
	return s.fillQueryRows(v, query, args...)
}

func (s *detailLoaderQuerySqlConn) RawDB() (*sql.DB, error) {
	return nil, errors.New("not implemented")
}

func (s *detailLoaderQuerySqlConn) Transact(_ func(sqlx.Session) error) error {
	return errors.New("not implemented")
}

func (s *detailLoaderQuerySqlConn) TransactCtx(_ context.Context, _ func(context.Context, sqlx.Session) error) error {
	return errors.New("not implemented")
}

func (s *detailLoaderQuerySqlConn) fillQueryRow(v any, _ string, _ ...any) error {
	switch dest := v.(type) {
	case *model.DProgram:
		if s.program == nil {
			return sqlx.ErrNotFound
		}
		*dest = *s.program
		return nil
	case *model.DProgramShowTime:
		if s.firstShowTime == nil {
			return sqlx.ErrNotFound
		}
		*dest = *s.firstShowTime
		return nil
	case *model.DProgramGroup:
		if s.group == nil {
			return sqlx.ErrNotFound
		}
		*dest = *s.group
		return nil
	default:
		return fmt.Errorf("unexpected query row dest type %T", v)
	}
}

func (s *detailLoaderQuerySqlConn) fillQueryRows(v any, _ string, _ ...any) error {
	switch dest := v.(type) {
	case *[]*model.DTicketCategory:
		if s.ticketCategoryErr != nil {
			return s.ticketCategoryErr
		}
		*dest = cloneTicketCategories(s.ticketCategories)
		return nil
	default:
		return fmt.Errorf("unexpected query rows dest type %T", v)
	}
}

func cloneTicketCategories(categories []*model.DTicketCategory) []*model.DTicketCategory {
	if len(categories) == 0 {
		return []*model.DTicketCategory{}
	}

	cloned := make([]*model.DTicketCategory, 0, len(categories))
	for _, category := range categories {
		if category == nil {
			continue
		}
		categoryCopy := *category
		cloned = append(cloned, &categoryCopy)
	}

	return cloned
}
