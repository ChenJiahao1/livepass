package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"livepass/jobs/rush-inventory-preheat/internal/dispatch"
	"livepass/jobs/rush-inventory-preheat/internal/outbox"
	"livepass/jobs/rush-inventory-preheat/taskdef"
	"livepass/pkg/delaytask"
	"livepass/pkg/xmysql"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const testRushInventoryPreheatMySQLDataSource = "root:123456@tcp(127.0.0.1:3306)/livepass_program?parseTime=true"

type fakeDelayTaskPublisher struct {
	messages []delaytask.Message
	err      error
}

func (f *fakeDelayTaskPublisher) Publish(_ context.Context, message delaytask.Message) error {
	f.messages = append(f.messages, message)
	return f.err
}

func TestRunOnceMarksPublished(t *testing.T) {
	resetRushInventoryPreheatDelayTaskOutbox(t)

	expectedOpenTime := time.Date(2026, time.April, 13, 16, 5, 0, 0, time.Local)
	message, err := taskdef.NewMessage(91001, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:        101,
		TaskType:  message.Type,
		TaskKey:   message.Key,
		Payload:   string(message.Payload),
		ExecuteAt: message.ExecuteAt,
	})

	publisher := &fakeDelayTaskPublisher{}
	logic := dispatch.NewRunOnceLogic(context.Background(), outbox.NewMysqlStore(map[string]sqlx.SqlConn{
		"program-db-0": sqlx.NewMysql(xmysql.WithLocalTime(testRushInventoryPreheatMySQLDataSource)),
	}), publisher)

	if err := logic.Run(taskdef.TaskTypeRushInventoryPreheat); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(publisher.messages) != 1 {
		t.Fatalf("publisher messages = %d, want 1", len(publisher.messages))
	}

	row := findDelayTaskOutboxRow(t, 101)
	if row.TaskStatus != delaytask.OutboxTaskStatusPublished {
		t.Fatalf("task_status = %d, want %d", row.TaskStatus, delaytask.OutboxTaskStatusPublished)
	}
	if row.PublishAttempts != 1 {
		t.Fatalf("publish_attempts = %d, want 1", row.PublishAttempts)
	}
	if !row.PublishedTime.Valid {
		t.Fatalf("expected published_time to be set")
	}
}

func TestDispatcherTreatsDuplicateTaskConflictAsSuccess(t *testing.T) {
	resetRushInventoryPreheatDelayTaskOutbox(t)

	expectedOpenTime := time.Date(2026, time.April, 13, 16, 10, 0, 0, time.Local)
	message, err := taskdef.NewMessage(92001, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:        201,
		TaskType:  message.Type,
		TaskKey:   message.Key,
		Payload:   string(message.Payload),
		ExecuteAt: message.ExecuteAt,
	})

	publisher := &fakeDelayTaskPublisher{err: asynq.ErrTaskIDConflict}
	logic := dispatch.NewRunOnceLogic(context.Background(), outbox.NewMysqlStore(map[string]sqlx.SqlConn{
		"program-db-0": sqlx.NewMysql(xmysql.WithLocalTime(testRushInventoryPreheatMySQLDataSource)),
	}), publisher)

	if err := logic.Run(taskdef.TaskTypeRushInventoryPreheat); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	row := findDelayTaskOutboxRow(t, 201)
	if row.TaskStatus != delaytask.OutboxTaskStatusPublished {
		t.Fatalf("task_status = %d, want %d", row.TaskStatus, delaytask.OutboxTaskStatusPublished)
	}
}

func TestDispatcherMarksPublishFailed(t *testing.T) {
	resetRushInventoryPreheatDelayTaskOutbox(t)

	expectedOpenTime := time.Date(2026, time.April, 13, 16, 15, 0, 0, time.Local)
	message, err := taskdef.NewMessage(93001, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:        301,
		TaskType:  message.Type,
		TaskKey:   message.Key,
		Payload:   string(message.Payload),
		ExecuteAt: message.ExecuteAt,
	})

	publisher := &fakeDelayTaskPublisher{err: fmt.Errorf("redis unavailable")}
	logic := dispatch.NewRunOnceLogic(context.Background(), outbox.NewMysqlStore(map[string]sqlx.SqlConn{
		"program-db-0": sqlx.NewMysql(xmysql.WithLocalTime(testRushInventoryPreheatMySQLDataSource)),
	}), publisher)

	if err := logic.Run(taskdef.TaskTypeRushInventoryPreheat); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	row := findDelayTaskOutboxRow(t, 301)
	if row.TaskStatus != delaytask.OutboxTaskStatusFailed {
		t.Fatalf("task_status = %d, want %d", row.TaskStatus, delaytask.OutboxTaskStatusFailed)
	}
	if row.PublishAttempts != 1 {
		t.Fatalf("publish_attempts = %d, want 1", row.PublishAttempts)
	}
	if !strings.Contains(row.LastPublishError, "redis unavailable") {
		t.Fatalf("last_publish_error = %s", row.LastPublishError)
	}
}

func TestDispatcherRepublishesPublishedAndFailedButSkipsProcessed(t *testing.T) {
	resetRushInventoryPreheatDelayTaskOutbox(t)

	expectedOpenTime := time.Date(2026, time.April, 13, 16, 20, 0, 0, time.Local)
	message1, err := taskdef.NewMessage(94001, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	message2, err := taskdef.NewMessage(94002, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	message3, err := taskdef.NewMessage(94003, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}

	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:              401,
		TaskType:        message1.Type,
		TaskKey:         message1.Key,
		Payload:         string(message1.Payload),
		ExecuteAt:       message1.ExecuteAt,
		TaskStatus:      delaytask.OutboxTaskStatusPublished,
		PublishAttempts: 1,
	})
	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:              402,
		TaskType:        message2.Type,
		TaskKey:         message2.Key,
		Payload:         string(message2.Payload),
		ExecuteAt:       message2.ExecuteAt,
		TaskStatus:      delaytask.OutboxTaskStatusFailed,
		PublishAttempts: 2,
	})
	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:              403,
		TaskType:        message3.Type,
		TaskKey:         message3.Key,
		Payload:         string(message3.Payload),
		ExecuteAt:       message3.ExecuteAt,
		TaskStatus:      delaytask.OutboxTaskStatusProcessed,
		PublishAttempts: 3,
	})

	publisher := &fakeDelayTaskPublisher{}
	logic := dispatch.NewRunOnceLogic(context.Background(), outbox.NewMysqlStore(map[string]sqlx.SqlConn{
		"program-db-0": sqlx.NewMysql(xmysql.WithLocalTime(testRushInventoryPreheatMySQLDataSource)),
	}), publisher)

	if err := logic.Run(taskdef.TaskTypeRushInventoryPreheat); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(publisher.messages) != 2 {
		t.Fatalf("publisher messages = %d, want 2", len(publisher.messages))
	}

	published := findDelayTaskOutboxRow(t, 401)
	if published.TaskStatus != delaytask.OutboxTaskStatusPublished || published.PublishAttempts != 2 {
		t.Fatalf("published row = %+v, want status=published attempts=2", published)
	}
	failed := findDelayTaskOutboxRow(t, 402)
	if failed.TaskStatus != delaytask.OutboxTaskStatusPublished || failed.PublishAttempts != 3 {
		t.Fatalf("failed row = %+v, want status=published attempts=3", failed)
	}
	processed := findDelayTaskOutboxRow(t, 403)
	if processed.TaskStatus != delaytask.OutboxTaskStatusProcessed || processed.PublishAttempts != 3 {
		t.Fatalf("processed row = %+v, want unchanged", processed)
	}
}

type delayTaskOutboxFixture struct {
	ID              int64
	TaskType        string
	TaskKey         string
	Payload         string
	ExecuteAt       time.Time
	TaskStatus      int64
	PublishAttempts int64
}

type delayTaskOutboxRow struct {
	ID               int64
	TaskStatus       int64
	PublishAttempts  int64
	LastPublishError string
	PublishedTime    sql.NullTime
	ConsumeAttempts  int64
	LastConsumeError string
	ProcessedTime    sql.NullTime
}

func resetRushInventoryPreheatDelayTaskOutbox(t *testing.T) {
	t.Helper()

	db := openRushInventoryPreheatTestDB(t)
	defer db.Close()

	content, err := os.ReadFile(filepath.Join(rushInventoryPreheatProjectRoot(t), "sql/program/d_delay_task_outbox.sql"))
	if err != nil {
		t.Fatalf("ReadFile(d_delay_task_outbox.sql) error = %v", err)
	}
	for _, stmt := range strings.Split(string(content), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec delay task outbox schema error: %v\nstatement: %s", err, stmt)
		}
	}
}

func seedDelayTaskOutboxRow(t *testing.T, fixture delayTaskOutboxFixture) {
	t.Helper()

	db := openRushInventoryPreheatTestDB(t)
	defer db.Close()

	taskStatus := fixture.TaskStatus
	if taskStatus == 0 {
		taskStatus = delaytask.OutboxTaskStatusPending
	}
	_, err := db.Exec(
		`INSERT INTO d_delay_task_outbox (
			id, task_type, task_key, payload, execute_at, task_status, publish_attempts,
			consume_attempts, last_publish_error, last_consume_error, published_time, processed_time,
			create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, 0, '', '', NULL, NULL, ?, ?, 1)`,
		fixture.ID,
		fixture.TaskType,
		fixture.TaskKey,
		fixture.Payload,
		fixture.ExecuteAt,
		taskStatus,
		fixture.PublishAttempts,
		fixture.ExecuteAt,
		fixture.ExecuteAt,
	)
	if err != nil {
		t.Fatalf("insert delay task outbox row error: %v", err)
	}
}

func findDelayTaskOutboxRow(t *testing.T, id int64) delayTaskOutboxRow {
	t.Helper()

	db := openRushInventoryPreheatTestDB(t)
	defer db.Close()

	var row delayTaskOutboxRow
	err := db.QueryRow(
		`SELECT id, task_status, publish_attempts, last_publish_error, published_time,
			consume_attempts, last_consume_error, processed_time
		FROM d_delay_task_outbox WHERE id = ? LIMIT 1`,
		id,
	).Scan(
		&row.ID,
		&row.TaskStatus,
		&row.PublishAttempts,
		&row.LastPublishError,
		&row.PublishedTime,
		&row.ConsumeAttempts,
		&row.LastConsumeError,
		&row.ProcessedTime,
	)
	if err != nil {
		t.Fatalf("query delay task outbox row error: %v", err)
	}
	return row
}

func openRushInventoryPreheatTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ensureRushInventoryPreheatTestDatabase(t)

	db, err := sql.Open("mysql", xmysql.WithLocalTime(testRushInventoryPreheatMySQLDataSource))
	if err != nil {
		t.Fatalf("sql.Open error: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("db.Ping error: %v", err)
	}
	return db
}

func ensureRushInventoryPreheatTestDatabase(t *testing.T) {
	t.Helper()

	dsn, err := mysqlDriver.ParseDSN(testRushInventoryPreheatMySQLDataSource)
	if err != nil {
		t.Fatalf("ParseDSN error: %v", err)
	}

	dbName := dsn.DBName
	dsn.DBName = ""
	rootDB, err := sql.Open("mysql", dsn.FormatDSN())
	if err != nil {
		t.Fatalf("sql.Open root error: %v", err)
	}
	defer rootDB.Close()

	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS `" + dbName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create database %s error: %v", dbName, err)
	}
}

func rushInventoryPreheatProjectRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
}
