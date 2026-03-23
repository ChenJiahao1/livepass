package xmysql

import (
	"database/sql"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestWithLocalTimeSetsDSNLocationToLocal(t *testing.T) {
	originalLocal := time.Local
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}
	time.Local = shanghai
	defer func() {
		time.Local = originalLocal
	}()

	dsn := WithLocalTime("root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true")

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN returned error: %v", err)
	}
	if cfg.Loc.String() != time.Local.String() {
		t.Fatalf("cfg.Loc = %q, want %q", cfg.Loc.String(), time.Local.String())
	}
	if !cfg.ParseTime {
		t.Fatalf("expected parseTime=true in normalized dsn")
	}
}

func TestConfigNormalizeAppliesPoolDefaults(t *testing.T) {
	t.Parallel()

	cfg := NewConfig("root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true").Normalize()

	if cfg.MaxOpenConns != DefaultMaxOpenConns {
		t.Fatalf("expected default max open conns %d, got %d", DefaultMaxOpenConns, cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != DefaultMaxIdleConns {
		t.Fatalf("expected default max idle conns %d, got %d", DefaultMaxIdleConns, cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != DefaultConnMaxLifetime {
		t.Fatalf("expected default conn max lifetime %s, got %s", DefaultConnMaxLifetime, cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != DefaultConnMaxIdleTime {
		t.Fatalf("expected default conn max idle time %s, got %s", DefaultConnMaxIdleTime, cfg.ConnMaxIdleTime)
	}
}

func TestApplyPoolSetsMaxOpenConnections(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("mysql", "root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true")
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	defer db.Close()

	ApplyPool(db, Config{
		MaxOpenConns:    23,
		MaxIdleConns:    11,
		ConnMaxLifetime: 3 * time.Minute,
		ConnMaxIdleTime: 90 * time.Second,
	})

	if got := db.Stats().MaxOpenConnections; got != 23 {
		t.Fatalf("expected max open connections 23, got %d", got)
	}
}
