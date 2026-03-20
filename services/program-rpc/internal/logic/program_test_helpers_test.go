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

func programProjectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
