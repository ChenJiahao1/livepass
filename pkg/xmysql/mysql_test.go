package xmysql

import (
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
