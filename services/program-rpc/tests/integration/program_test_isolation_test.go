package integration_test

import (
	"strings"
	"testing"

	"livepass/services/program-rpc/internal/model"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestProgramIntegrationUsesIsolatedDatabaseDSN(t *testing.T) {
	cfg, err := mysqlDriver.ParseDSN(testProgramMySQLDataSource)
	if err != nil {
		t.Fatalf("ParseDSN returned error: %v", err)
	}

	if cfg.DBName == "livepass_program" {
		t.Fatalf("expected integration tests to avoid shared database livepass_program")
	}
	if !strings.HasPrefix(cfg.DBName, "livepass_program_test_") {
		t.Fatalf("expected isolated database name prefix livepass_program_test_, got %q", cfg.DBName)
	}
}

func TestProgramIntegrationUsesIsolationNamespaceForRedisArtifacts(t *testing.T) {
	if got := model.ProgramCacheKey(10001); got == "cache:dProgram:id:10001" {
		t.Fatalf("expected program cache key to avoid shared redis namespace, got %q", got)
	}
	if got := model.ProgramGroupCacheKey(20001); got == "cache:dProgramGroup:id:20001" {
		t.Fatalf("expected program group cache key to avoid shared redis namespace, got %q", got)
	}
	if got := model.ProgramFirstShowTimeCacheKey(30001); got == "cache:dProgramShowTime:first:programId:30001" {
		t.Fatalf("expected first show time cache key to avoid shared redis namespace, got %q", got)
	}

	channel := programCachePubSubChannel(t, "detail")
	if channel == "livepass:test:program:cache:invalidate:TestProgramIntegrationUsesIsolationNamespaceForRedisArtifacts:detail" {
		t.Fatalf("expected cache invalidation pubsub channel to avoid shared redis namespace, got %q", channel)
	}
}
