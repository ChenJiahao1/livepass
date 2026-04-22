package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"livepass/jobs/order-close/internal/dispatch"
	"livepass/jobs/order-close/internal/outbox"
	"livepass/pkg/delaytask"
	"livepass/pkg/xmysql"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const testOrderCloseMySQLDataSource = "root:123456@tcp(127.0.0.1:3306)/livepass_order?parseTime=true"

type fakeDelayTaskPublisher struct {
	messages []delaytask.Message
	err      error
}

func (f *fakeDelayTaskPublisher) Publish(_ context.Context, message delaytask.Message) error {
	f.messages = append(f.messages, message)
	return f.err
}

func TestDispatcherMarksPublishedAfterPublish(t *testing.T) {
	resetOrderCloseDelayTaskOutbox(t)

	executeAt := time.Date(2026, time.April, 13, 16, 0, 0, 0, time.Local)
	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:        101,
		TaskType:  "order.close_timeout",
		TaskKey:   "order.close_timeout:91001",
		Payload:   `{"orderNumber":91001}`,
		ExecuteAt: executeAt,
	})

	publisher := &fakeDelayTaskPublisher{}
	logic := dispatch.NewRunOnceLogic(context.Background(), outbox.NewMysqlStore(map[string]sqlx.SqlConn{
		"order-db-0": sqlx.NewMysql(xmysql.WithLocalTime(testOrderCloseMySQLDataSource)),
	}), publisher, 10)

	if err := logic.Run(taskTypeCloseTimeout); err != nil {
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
	resetOrderCloseDelayTaskOutbox(t)

	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:        201,
		TaskType:  "order.close_timeout",
		TaskKey:   "order.close_timeout:92001",
		Payload:   `{"orderNumber":92001}`,
		ExecuteAt: time.Date(2026, time.April, 13, 16, 5, 0, 0, time.Local),
	})

	publisher := &fakeDelayTaskPublisher{err: asynq.ErrTaskIDConflict}
	logic := dispatch.NewRunOnceLogic(context.Background(), outbox.NewMysqlStore(map[string]sqlx.SqlConn{
		"order-db-0": sqlx.NewMysql(xmysql.WithLocalTime(testOrderCloseMySQLDataSource)),
	}), publisher, 10)

	if err := logic.Run(taskTypeCloseTimeout); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	row := findDelayTaskOutboxRow(t, 201)
	if row.TaskStatus != delaytask.OutboxTaskStatusPublished {
		t.Fatalf("task_status = %d, want %d", row.TaskStatus, delaytask.OutboxTaskStatusPublished)
	}
}

func TestDispatcherMarksPublishFailed(t *testing.T) {
	resetOrderCloseDelayTaskOutbox(t)

	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:        301,
		TaskType:  "order.close_timeout",
		TaskKey:   "order.close_timeout:93001",
		Payload:   `{"orderNumber":93001}`,
		ExecuteAt: time.Date(2026, time.April, 13, 16, 10, 0, 0, time.Local),
	})

	publisher := &fakeDelayTaskPublisher{err: fmt.Errorf("redis unavailable")}
	logic := dispatch.NewRunOnceLogic(context.Background(), outbox.NewMysqlStore(map[string]sqlx.SqlConn{
		"order-db-0": sqlx.NewMysql(xmysql.WithLocalTime(testOrderCloseMySQLDataSource)),
	}), publisher, 10)

	if err := logic.Run(taskTypeCloseTimeout); err != nil {
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
	resetOrderCloseDelayTaskOutbox(t)

	executeAt := time.Date(2026, time.April, 13, 16, 20, 0, 0, time.Local)
	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:              401,
		TaskType:        "order.close_timeout",
		TaskKey:         "order.close_timeout:94001",
		Payload:         `{"orderNumber":94001}`,
		ExecuteAt:       executeAt,
		TaskStatus:      delaytask.OutboxTaskStatusPublished,
		PublishAttempts: 1,
	})
	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:              402,
		TaskType:        "order.close_timeout",
		TaskKey:         "order.close_timeout:94002",
		Payload:         `{"orderNumber":94002}`,
		ExecuteAt:       executeAt,
		TaskStatus:      delaytask.OutboxTaskStatusFailed,
		PublishAttempts: 2,
	})
	seedDelayTaskOutboxRow(t, delayTaskOutboxFixture{
		ID:              403,
		TaskType:        "order.close_timeout",
		TaskKey:         "order.close_timeout:94003",
		Payload:         `{"orderNumber":94003}`,
		ExecuteAt:       executeAt,
		TaskStatus:      delaytask.OutboxTaskStatusProcessed,
		PublishAttempts: 3,
	})

	publisher := &fakeDelayTaskPublisher{}
	logic := dispatch.NewRunOnceLogic(context.Background(), outbox.NewMysqlStore(map[string]sqlx.SqlConn{
		"order-db-0": sqlx.NewMysql(xmysql.WithLocalTime(testOrderCloseMySQLDataSource)),
	}), publisher, 10)

	if err := logic.Run(taskTypeCloseTimeout); err != nil {
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

const taskTypeCloseTimeout = "order.close_timeout"

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
}

func resetOrderCloseDelayTaskOutbox(t *testing.T) {
	t.Helper()

	db := openOrderCloseTestDB(t)
	defer db.Close()

	content, err := os.ReadFile(filepath.Join(orderCloseProjectRoot(t), "sql/order/sharding/d_delay_task_outbox.sql"))
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

	db := openOrderCloseTestDB(t)
	defer db.Close()

	taskStatus := fixture.TaskStatus
	if taskStatus == 0 {
		taskStatus = delaytask.OutboxTaskStatusPending
	}
	_, err := db.Exec(
		`INSERT INTO d_delay_task_outbox (
			id, task_type, task_key, payload, execute_at, task_status, publish_attempts,
			consume_attempts, last_publish_error, last_consume_error, published_time,
			processed_time, create_time, edit_time, status
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

	db := openOrderCloseTestDB(t)
	defer db.Close()

	var row delayTaskOutboxRow
	err := db.QueryRow(
		`SELECT id, task_status, publish_attempts, last_publish_error, published_time
		FROM d_delay_task_outbox WHERE id = ? LIMIT 1`,
		id,
	).Scan(&row.ID, &row.TaskStatus, &row.PublishAttempts, &row.LastPublishError, &row.PublishedTime)
	if err != nil {
		t.Fatalf("query delay task outbox row error: %v", err)
	}
	return row
}

func openOrderCloseTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ensureOrderCloseTestDatabase(t)

	db, err := sql.Open("mysql", xmysql.WithLocalTime(testOrderCloseMySQLDataSource))
	if err != nil {
		t.Fatalf("sql.Open error: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("db.Ping error: %v", err)
	}
	return db
}

func ensureOrderCloseTestDatabase(t *testing.T) {
	t.Helper()

	cfg, err := mysqlDriver.ParseDSN(testOrderCloseMySQLDataSource)
	if err != nil {
		t.Fatalf("ParseDSN error: %v", err)
	}

	dbName := cfg.DBName
	cfg.DBName = ""
	adminDB, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatalf("sql.Open admin db error: %v", err)
	}
	defer adminDB.Close()

	stmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci",
		strings.ReplaceAll(dbName, "`", "``"),
	)
	if _, err := adminDB.Exec(stmt); err != nil {
		t.Fatalf("create database error: %v", err)
	}
}
