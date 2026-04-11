package integration_test

import (
	"reflect"
	"testing"
	"time"
	_ "unsafe"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/cache"
	gzredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
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

func TestProgramServiceContextUsesCachedModelsWhenCacheConfigured(t *testing.T) {
	cfg := config.Config{
		MySQL: xmysql.Config{
			DataSource: testProgramMySQLDataSource,
		},
		StoreRedis: xredis.Config{
			Host: "127.0.0.1:6379",
			Type: "node",
		},
	}
	configureProgramLayeredCache(t, &cfg)

	svcCtx := svc.NewServiceContext(cfg)

	requireModelUsesCachedConn(t, svcCtx.DProgramModel, "DProgramModel")
	requireModelUsesCachedConn(t, svcCtx.DProgramGroupModel, "DProgramGroupModel")
	requireModelUsesCachedConn(t, svcCtx.DProgramShowTimeModel, "DProgramShowTimeModel")
}

func TestProgramServiceContextWiresProgramLocalCaches(t *testing.T) {
	cfg := config.Config{
		MySQL: xmysql.Config{
			DataSource: testProgramMySQLDataSource,
		},
		StoreRedis: xredis.Config{
			Host: "127.0.0.1:6379",
			Type: "node",
		},
	}
	configureProgramLayeredCache(t, &cfg)

	svcCtx := svc.NewServiceContext(cfg)

	requireServiceContextDependency(t, svcCtx, "CategorySnapshotCache")
	requireServiceContextDependency(t, svcCtx, "ProgramDetailCache")
}

func TestProgramServiceContextWiresCacheInvalidationDependencies(t *testing.T) {
	cfg := config.Config{
		MySQL: xmysql.Config{
			DataSource: testProgramMySQLDataSource,
		},
		StoreRedis: xredis.Config{
			Host: "127.0.0.1:6379",
			Type: "node",
		},
		CacheInvalidation: config.CacheInvalidationConfig{
			Enabled: true,
		},
	}
	configureProgramLayeredCache(t, &cfg)

	svcCtx := svc.NewServiceContext(cfg)

	requireServiceContextDependency(t, svcCtx, "ProgramCacheRegistry")
	requireServiceContextDependency(t, svcCtx, "ProgramCacheInvalidator")
	requireServiceContextDependency(t, svcCtx, "ProgramCacheSubscriber")
}

func configureProgramLayeredCache(t *testing.T, cfg *config.Config) {
	t.Helper()

	cfgValue := reflect.ValueOf(cfg).Elem()

	cacheField := cfgValue.FieldByName("Cache")
	if !cacheField.IsValid() {
		t.Fatalf("expected config.Config to expose Cache settings")
	}
	cacheField.Set(reflect.ValueOf(cache.CacheConf{
		{
			RedisConf: gzredis.RedisConf{
				Host: "127.0.0.1:6379",
				Type: "node",
			},
			Weight: 100,
		},
	}))

	localCacheField := cfgValue.FieldByName("LocalCache")
	if !localCacheField.IsValid() {
		t.Fatalf("expected config.Config to expose LocalCache settings")
	}

	setStructField(t, localCacheField, "DetailTTL", reflect.ValueOf(20*time.Second))
	setStructField(t, localCacheField, "DetailNotFoundTTL", reflect.ValueOf(5*time.Second))
	setStructField(t, localCacheField, "DetailCacheLimit", reflect.ValueOf(5000))
	setStructField(t, localCacheField, "CategorySnapshotTTL", reflect.ValueOf(5*time.Minute))
}

func requireServiceContextDependency(t *testing.T, svcCtx *svc.ServiceContext, fieldName string) {
	t.Helper()

	field := reflect.ValueOf(svcCtx).Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("expected ServiceContext.%s to be wired", fieldName)
	}

	if field.Kind() == reflect.Pointer && field.IsNil() {
		t.Fatalf("expected ServiceContext.%s to be non-nil", fieldName)
	}
}

func requireModelUsesCachedConn(t *testing.T, model any, modelName string) {
	t.Helper()

	if !containsFieldOfType(reflect.ValueOf(model), reflect.TypeOf(sqlc.CachedConn{})) {
		t.Fatalf("expected %s to embed sqlc.CachedConn when Cache is configured", modelName)
	}
}

func containsFieldOfType(value reflect.Value, target reflect.Type) bool {
	if !value.IsValid() {
		return false
	}

	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return false
		}
		return containsFieldOfType(value.Elem(), target)
	case reflect.Struct:
		if value.Type() == target {
			return true
		}
		for i := 0; i < value.NumField(); i++ {
			if containsFieldOfType(value.Field(i), target) {
				return true
			}
		}
	}

	return false
}

func setStructField(t *testing.T, target reflect.Value, name string, value reflect.Value) {
	t.Helper()

	field := target.FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("expected field %s to exist", name)
	}

	if !value.Type().AssignableTo(field.Type()) {
		t.Fatalf("value type %s is not assignable to field %s (%s)", value.Type(), name, field.Type())
	}

	field.Set(value)
}
