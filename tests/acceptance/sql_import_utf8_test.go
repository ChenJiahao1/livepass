package acceptance_test

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"damai-go/pkg/xmysql"

	_ "github.com/go-sql-driver/mysql"
)

func TestImportSQLScriptPreservesUTF8ProgramSeed(t *testing.T) {
	adminDB := openAcceptanceMySQL(t, xmysql.WithLocalTime("root:123456@tcp(127.0.0.1:3306)/?parseTime=true"))
	defer adminDB.Close()

	dbName := fmt.Sprintf("damai_program_import_utf8_%d", time.Now().UnixNano())
	dropAcceptanceDatabase(t, adminDB, dbName)
	defer dropAcceptanceDatabase(t, adminDB, dbName)

	cmd := exec.Command("bash", filepath.Join(acceptanceProjectRoot(t), "scripts", "import_sql.sh"))
	cmd.Dir = acceptanceProjectRoot(t)
	cmd.Env = append(
		os.Environ(),
		"IMPORT_DOMAINS=program",
		"MYSQL_CONTAINER=docker-compose-mysql-1",
		"MYSQL_PASSWORD=123456",
		"MYSQL_DB_PROGRAM="+dbName,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run import_sql.sh error: %v\noutput:\n%s", err, output)
	}

	programDB := openAcceptanceMySQL(
		t,
		xmysql.WithLocalTime(fmt.Sprintf("root:123456@tcp(127.0.0.1:3306)/%s?parseTime=true", dbName)),
	)
	defer programDB.Close()

	var title, place string
	if err := programDB.QueryRow("SELECT title, place FROM d_program WHERE id = 10001").Scan(&title, &place); err != nil {
		t.Fatalf("query d_program error: %v", err)
	}
	if title != "Phase1 示例演出" {
		t.Fatalf("unexpected program title: %q", title)
	}
	if place != "北京示例剧场" {
		t.Fatalf("unexpected program place: %q", place)
	}

	rows, err := programDB.Query("SELECT id, introduce FROM d_ticket_category ORDER BY id")
	if err != nil {
		t.Fatalf("query d_ticket_category error: %v", err)
	}
	defer rows.Close()

	expected := map[int64]string{
		40001: "普通票",
		40002: "VIP票",
	}
	actual := make(map[int64]string, len(expected))
	for rows.Next() {
		var id int64
		var introduce string
		if err := rows.Scan(&id, &introduce); err != nil {
			t.Fatalf("scan d_ticket_category error: %v", err)
		}
		actual[id] = introduce
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate d_ticket_category error: %v", err)
	}

	for id, want := range expected {
		if got := actual[id]; got != want {
			t.Fatalf("unexpected introduce for %d: got %q want %q", id, got, want)
		}
	}
}

func openAcceptanceMySQL(t *testing.T, dataSource string) *sql.DB {
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

func dropAcceptanceDatabase(t *testing.T, db *sql.DB, dbName string) {
	t.Helper()

	if _, err := db.Exec("DROP DATABASE IF EXISTS `" + dbName + "`"); err != nil {
		t.Fatalf("drop database %s error: %v", dbName, err)
	}
}

func acceptanceProjectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
