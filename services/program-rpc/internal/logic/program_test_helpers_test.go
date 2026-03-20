package logic

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"damai-go/pkg/xmysql"
	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/internal/svc"
)

const testProgramMySQLDataSource = "root:123456@tcp(127.0.0.1:3306)/damai_program?parseTime=true"

type ticketCategoryFixture struct {
	ID           int64
	Introduce    string
	Price        float64
	TotalNumber  int64
	RemainNumber int64
}

type seatFixture struct {
	ID               int64
	ProgramID        int64
	TicketCategoryID int64
	RowCode          int
	ColCode          int
	SeatType         int
	Price            float64
	SeatStatus       int
	FreezeToken      string
	FreezeExpireTime string
}

type seatFreezeFixture struct {
	ID               int64
	FreezeToken      string
	RequestNo        string
	ProgramID        int64
	TicketCategoryID int64
	SeatCount        int
	FreezeStatus     int
	ExpireTime       string
	ReleaseReason    string
	ReleaseTime      string
}

type programFixture struct {
	ProgramID                 int64
	ProgramGroupID            int64
	ParentProgramCategoryID   int64
	ProgramCategoryID         int64
	AreaID                    int64
	Prime                     int64
	Title                     string
	Actor                     string
	Place                     string
	ItemPicture               string
	Detail                    string
	HighHeat                  int64
	IssueTime                 string
	ShowTime                  string
	ShowDayTime               string
	ShowWeekTime              string
	GroupAreaName             string
	ProgramSimpleInfoAreaName string
	TicketCategories          []ticketCategoryFixture
}

func newProgramTestServiceContext(t *testing.T) *svc.ServiceContext {
	t.Helper()

	return svc.NewServiceContext(config.Config{
		MySQL: xmysql.Config{
			DataSource: testProgramMySQLDataSource,
		},
	})
}

func resetProgramDomainState(t *testing.T) {
	t.Helper()

	db := openProgramTestDB(t, testProgramMySQLDataSource)
	defer db.Close()

	for _, relativePath := range []string{
		"sql/program/d_program_category.sql",
		"sql/program/d_program_group.sql",
		"sql/program/d_program.sql",
		"sql/program/d_program_show_time.sql",
		"sql/program/d_seat.sql",
		"sql/program/d_seat_freeze.sql",
		"sql/program/d_ticket_category.sql",
		"sql/program/dev_seed.sql",
	} {
		execProgramSQLFile(t, db, relativePath)
	}
}

func seedProgramFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...programFixture) {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		insertProgramFixture(t, db, withProgramFixtureDefaults(fixture))
	}
}

func seedSeatFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...seatFixture) {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withSeatFixtureDefaults(fixture)
		mustExecProgramSQL(
			t,
			db,
			`INSERT INTO d_seat (
				id, program_id, ticket_category_id, row_code, col_code, seat_type, price, seat_status,
				freeze_token, freeze_expire_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.ProgramID,
			fixture.TicketCategoryID,
			fixture.RowCode,
			fixture.ColCode,
			fixture.SeatType,
			fixture.Price,
			fixture.SeatStatus,
			nullIfEmpty(fixture.FreezeToken),
			nullIfEmpty(fixture.FreezeExpireTime),
			"2026-01-01 00:00:00",
			"2026-01-01 00:00:00",
			1,
		)
	}
}

func seedSeatFreezeFixture(t *testing.T, svcCtx *svc.ServiceContext, fixture seatFreezeFixture) {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	fixture = withSeatFreezeFixtureDefaults(fixture)
	mustExecProgramSQL(
		t,
		db,
		`INSERT INTO d_seat_freeze (
			id, freeze_token, request_no, program_id, ticket_category_id, seat_count, freeze_status,
			expire_time, release_reason, release_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.ID,
		fixture.FreezeToken,
		fixture.RequestNo,
		fixture.ProgramID,
		fixture.TicketCategoryID,
		fixture.SeatCount,
		fixture.FreezeStatus,
		fixture.ExpireTime,
		nullIfEmpty(fixture.ReleaseReason),
		nullIfEmpty(fixture.ReleaseTime),
		"2026-01-01 00:00:00",
		"2026-01-01 00:00:00",
		1,
	)
}

func seedSeatInventoryProgram(t *testing.T, svcCtx *svc.ServiceContext, programID, ticketCategoryID int64) {
	t.Helper()

	seedProgramFixtures(t, svcCtx, programFixture{
		ProgramID:               programID,
		ProgramGroupID:          programID + 1000,
		ParentProgramCategoryID: 1,
		ProgramCategoryID:       11,
		AreaID:                  1,
		Title:                   "座位库存测试演出",
		ShowTime:                "2026-12-31 19:30:00",
		ShowDayTime:             "2026-12-31 00:00:00",
		ShowWeekTime:            "周四",
		TicketCategories: []ticketCategoryFixture{
			{
				ID:           ticketCategoryID,
				Introduce:    "普通票",
				Price:        299,
				TotalNumber:  100,
				RemainNumber: 100,
			},
		},
	})
}

func openProgramTestDB(t *testing.T, dataSource string) *sql.DB {
	t.Helper()

	db, err := sql.Open("mysql", dataSource)
	if err != nil {
		t.Fatalf("sql.Open error: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("db.Ping error: %v", err)
	}

	return db
}

func withProgramFixtureDefaults(fixture programFixture) programFixture {
	if fixture.Prime == 0 {
		fixture.Prime = 1
	}
	if fixture.Title == "" {
		fixture.Title = fmt.Sprintf("Program-%d", fixture.ProgramID)
	}
	if fixture.Actor == "" {
		fixture.Actor = "测试艺人"
	}
	if fixture.Place == "" {
		fixture.Place = "测试场馆"
	}
	if fixture.ItemPicture == "" {
		fixture.ItemPicture = fmt.Sprintf("https://example.com/program-%d.jpg", fixture.ProgramID)
	}
	if fixture.Detail == "" {
		fixture.Detail = fmt.Sprintf("<p>fixture detail %d</p>", fixture.ProgramID)
	}
	if fixture.IssueTime == "" {
		fixture.IssueTime = "2026-06-01 09:00:00"
	}
	if fixture.ShowWeekTime == "" {
		fixture.ShowWeekTime = "周六"
	}
	if fixture.GroupAreaName == "" {
		fixture.GroupAreaName = "测试城市"
	}
	if fixture.ProgramSimpleInfoAreaName == "" {
		fixture.ProgramSimpleInfoAreaName = fixture.GroupAreaName
	}
	if len(fixture.TicketCategories) == 0 {
		fixture.TicketCategories = []ticketCategoryFixture{
			{
				ID:           fixture.ProgramID + 30000,
				Introduce:    "普通票",
				Price:        199,
				TotalNumber:  100,
				RemainNumber: 80,
			},
			{
				ID:           fixture.ProgramID + 30001,
				Introduce:    "VIP票",
				Price:        399,
				TotalNumber:  50,
				RemainNumber: 40,
			},
		}
	}

	return fixture
}

func withSeatFixtureDefaults(fixture seatFixture) seatFixture {
	if fixture.SeatType == 0 {
		fixture.SeatType = 1
	}
	if fixture.Price == 0 {
		fixture.Price = 299
	}
	if fixture.SeatStatus == 0 {
		fixture.SeatStatus = 1
	}

	return fixture
}

func withSeatFreezeFixtureDefaults(fixture seatFreezeFixture) seatFreezeFixture {
	if fixture.SeatCount == 0 {
		fixture.SeatCount = 1
	}
	if fixture.FreezeStatus == 0 {
		fixture.FreezeStatus = 1
	}
	if fixture.ExpireTime == "" {
		fixture.ExpireTime = "2026-12-31 20:00:00"
	}

	return fixture
}

func insertProgramFixture(t *testing.T, db *sql.DB, fixture programFixture) {
	t.Helper()

	programJSON := fmt.Sprintf(
		`[{"programId":%d,"areaId":%d,"areaIdName":"%s"}]`,
		fixture.ProgramID,
		fixture.AreaID,
		fixture.ProgramSimpleInfoAreaName,
	)
	showTimeID := fixture.ProgramID + 20000

	mustExecProgramSQL(
		t,
		db,
		`INSERT INTO d_program_group (id, program_json, recent_show_time, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?)`,
		fixture.ProgramGroupID,
		programJSON,
		fixture.ShowTime,
		"2026-01-01 00:00:00",
		"2026-01-01 00:00:00",
		1,
	)

	mustExecProgramSQL(
		t,
		db,
		`INSERT INTO d_program (
			id, program_group_id, prime, area_id, program_category_id, parent_program_category_id,
			title, actor, place, item_picture, detail, high_heat, program_status, issue_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.ProgramID,
		fixture.ProgramGroupID,
		fixture.Prime,
		fixture.AreaID,
		fixture.ProgramCategoryID,
		fixture.ParentProgramCategoryID,
		fixture.Title,
		fixture.Actor,
		fixture.Place,
		fixture.ItemPicture,
		fixture.Detail,
		fixture.HighHeat,
		1,
		fixture.IssueTime,
		"2026-01-01 00:00:00",
		"2026-01-01 00:00:00",
		1,
	)

	mustExecProgramSQL(
		t,
		db,
		`INSERT INTO d_program_show_time (id, program_id, show_time, show_day_time, show_week_time, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		showTimeID,
		fixture.ProgramID,
		fixture.ShowTime,
		fixture.ShowDayTime,
		fixture.ShowWeekTime,
		"2026-01-01 00:00:00",
		"2026-01-01 00:00:00",
		1,
	)

	for _, ticketCategory := range fixture.TicketCategories {
		mustExecProgramSQL(
			t,
			db,
			`INSERT INTO d_ticket_category (id, program_id, introduce, price, total_number, remain_number, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ticketCategory.ID,
			fixture.ProgramID,
			ticketCategory.Introduce,
			ticketCategory.Price,
			ticketCategory.TotalNumber,
			ticketCategory.RemainNumber,
			"2026-01-01 00:00:00",
			"2026-01-01 00:00:00",
			1,
		)
	}
}

func mustExecProgramSQL(t *testing.T, db *sql.DB, stmt string, args ...interface{}) {
	t.Helper()

	if _, err := db.Exec(stmt, args...); err != nil {
		t.Fatalf("db.Exec error: %v\nstatement: %s", err, stmt)
	}
}

func execProgramSQLFile(t *testing.T, db *sql.DB, relativePath string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(programProjectRoot(t), relativePath))
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", relativePath, err)
	}

	for _, stmt := range strings.Split(string(content), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %s error: %v\nstatement: %s", relativePath, err, stmt)
		}
	}
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}

	return s
}

func programProjectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
