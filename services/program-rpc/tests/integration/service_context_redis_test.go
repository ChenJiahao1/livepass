package integration_test

import (
	"testing"
	_ "unsafe"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//go:linkname mapAutoAssignSeatError damai-go/services/program-rpc/internal/logic.mapAutoAssignSeatError
func mapAutoAssignSeatError(err error) error

//go:linkname mapConfirmSeatFreezeError damai-go/services/program-rpc/internal/logic.mapConfirmSeatFreezeError
func mapConfirmSeatFreezeError(err error) error

//go:linkname mapReleaseSeatFreezeError damai-go/services/program-rpc/internal/logic.mapReleaseSeatFreezeError
func mapReleaseSeatFreezeError(err error) error

func TestProgramServiceContextRedis(t *testing.T) {
	baseConfig := config.Config{
		MySQL: xmysql.Config{
			DataSource: testProgramMySQLDataSource,
		},
	}

	t.Run("redis disabled when host empty", func(t *testing.T) {
		svcCtx := svc.NewServiceContext(baseConfig)
		if svcCtx.Redis != nil {
			t.Fatalf("expected redis client to be nil when host is empty")
		}
	})

	t.Run("redis enabled when host configured", func(t *testing.T) {
		cfg := baseConfig
		cfg.StoreRedis = xredis.Config{
			Host: "127.0.0.1:6379",
			Type: "node",
		}
		svcCtx := svc.NewServiceContext(cfg)
		if svcCtx.Redis == nil {
			t.Fatalf("expected redis client to be wired when host is configured")
		}
	})
}

func TestProgramLedgerNotReadyMappedToFailedPrecondition(t *testing.T) {
	for _, mapper := range []struct {
		name string
		fn   func(error) error
	}{
		{name: "auto assign", fn: mapAutoAssignSeatError},
		{name: "confirm freeze", fn: mapConfirmSeatFreezeError},
		{name: "release freeze", fn: mapReleaseSeatFreezeError},
	} {
		t.Run(mapper.name, func(t *testing.T) {
			err := mapper.fn(xerr.ErrProgramSeatLedgerNotReady)
			if status.Code(err) != codes.FailedPrecondition {
				t.Fatalf("expected failed precondition, got %v", err)
			}
		})
	}
}
