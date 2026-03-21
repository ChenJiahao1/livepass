package integration_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"damai-go/pkg/xmysql"
	"damai-go/services/pay-rpc/internal/config"
	"damai-go/services/pay-rpc/internal/model"
	"damai-go/services/pay-rpc/internal/svc"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const payDateTimeLayout = "2006-01-02 15:04:05"

const testPayMySQLDataSource = "root:123456@tcp(127.0.0.1:3306)/damai_pay?parseTime=true"
const testPayMySQLAdminDataSource = "root:123456@tcp(127.0.0.1:3306)/?parseTime=true"

type payBillFixture struct {
	ID          int64
	PayBillNo   int64
	OrderNumber int64
	UserID      int64
	Subject     string
	Channel     string
	OrderAmount int64
	PayStatus   int64
	PayTime     string
}

func newPayTestServiceContext(t *testing.T) *svc.ServiceContext {
	t.Helper()

	cfg := config.Config{
		MySQL: xmysql.Config{
			DataSource: testPayMySQLDataSource,
		},
	}
	conn := sqlx.NewMysql(cfg.MySQL.DataSource)

	return &svc.ServiceContext{
		Config:        cfg,
		SqlConn:       conn,
		DPayBillModel: model.NewDPayBillModel(conn),
	}
}

func resetPayDomainState(t *testing.T) {
	t.Helper()

	db := openPayTestDB(t, testPayMySQLDataSource)
	defer db.Close()

	execPaySQLFile(t, db, "sql/pay/d_pay_bill.sql")
}

func seedPayBillFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...payBillFixture) {
	t.Helper()

	db := openPayTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withPayBillFixtureDefaults(fixture)
		mustExecPaySQL(
			t,
			db,
			`INSERT INTO d_pay_bill (
				id, pay_bill_no, order_number, user_id, subject, channel, order_amount, pay_status, pay_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.PayBillNo,
			fixture.OrderNumber,
			fixture.UserID,
			fixture.Subject,
			fixture.Channel,
			fixture.OrderAmount,
			fixture.PayStatus,
			nullTimeIfEmpty(fixture.PayTime),
			"2026-01-01 00:00:00",
			"2026-01-01 00:00:00",
			1,
		)
	}
}

func countPayBillRows(t *testing.T, dataSource string) int64 {
	t.Helper()

	db := openPayTestDB(t, dataSource)
	defer db.Close()

	var count int64
	if err := db.QueryRow("SELECT COUNT(1) FROM d_pay_bill").Scan(&count); err != nil {
		t.Fatalf("QueryRow count error: %v", err)
	}

	return count
}

func openPayTestDB(t *testing.T, dataSource string) *sql.DB {
	t.Helper()

	ensurePayTestDatabase(t)

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

func ensurePayTestDatabase(t *testing.T) {
	t.Helper()

	db, err := sql.Open("mysql", testPayMySQLAdminDataSource)
	if err != nil {
		t.Fatalf("sql.Open admin error: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping admin error: %v", err)
	}
	if _, err := db.Exec("CREATE DATABASE IF NOT EXISTS damai_pay DEFAULT CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create database error: %v", err)
	}
}

func withPayBillFixtureDefaults(fixture payBillFixture) payBillFixture {
	if fixture.ID == 0 {
		fixture.ID = fixture.OrderNumber + 1000
	}
	if fixture.PayBillNo == 0 {
		fixture.PayBillNo = fixture.OrderNumber + 2000
	}
	if fixture.UserID == 0 {
		fixture.UserID = 3001
	}
	if fixture.Subject == "" {
		fixture.Subject = "模拟支付单"
	}
	if fixture.Channel == "" {
		fixture.Channel = "mock"
	}
	if fixture.OrderAmount == 0 {
		fixture.OrderAmount = 399
	}
	if fixture.PayStatus == 0 {
		fixture.PayStatus = 2
	}
	if fixture.PayTime == "" {
		fixture.PayTime = "2026-01-01 10:00:00"
	}

	return fixture
}

func execPaySQLFile(t *testing.T, db *sql.DB, relativePath string) {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", ".."))
	content, err := os.ReadFile(filepath.Join(projectRoot, relativePath))
	if err != nil {
		t.Fatalf("os.ReadFile error: %v", err)
	}

	statements := strings.Split(string(content), ";")
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		mustExecPaySQL(t, db, statement)
	}
}

func mustExecPaySQL(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	t.Helper()

	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("db.Exec error: %v, query=%s", err, query)
	}
}

func nullTimeIfEmpty(value string) sql.NullTime {
	if value == "" {
		return sql.NullTime{}
	}

	return sql.NullTime{
		Time:  mustParsePayTime(value),
		Valid: true,
	}
}

func mustParsePayTime(value string) time.Time {
	parsed, err := time.ParseInLocation(payDateTimeLayout, value, time.Local)
	if err != nil {
		panic(err)
	}

	return parsed
}
