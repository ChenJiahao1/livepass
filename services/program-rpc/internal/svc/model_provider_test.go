package svc

import (
	"reflect"
	"testing"

	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/program-rpc/internal/config"

	"github.com/zeromicro/go-zero/core/stores/cache"
	gzredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestNewProgramModelsKeepsTicketCategoryUncached(t *testing.T) {
	models := newProgramModelsForTest(t, true)

	if models.DTicketCategoryModel == nil {
		t.Fatal("expected ticket category model")
	}

	requireModelUsesCachedConn(t, models.DProgramModel, "DProgramModel")
	requireModelUsesCachedConn(t, models.DProgramGroupModel, "DProgramGroupModel")
	requireModelUsesCachedConn(t, models.DProgramShowTimeModel, "DProgramShowTimeModel")
	requireModelDoesNotUseCachedConn(t, models.DTicketCategoryModel, "DTicketCategoryModel")
	requireModelDoesNotUseCachedConn(t, models.DProgramCategoryModel, "DProgramCategoryModel")
	requireModelDoesNotUseCachedConn(t, models.DSeatModel, "DSeatModel")
}

func newProgramModelsForTest(t *testing.T, enableCache bool) ProgramModels {
	t.Helper()

	conn := sqlx.NewMysql(xmysql.WithLocalTime("root:123456@tcp(127.0.0.1:3306)/damai_program?parseTime=true"))
	rawDB, err := conn.RawDB()
	if err != nil {
		t.Fatalf("raw db error: %v", err)
	}
	t.Cleanup(func() {
		_ = rawDB.Close()
	})

	var rds *xredis.Client
	cfg := config.Config{}
	if enableCache {
		rds = xredis.MustNew(xredis.Config{
			Host: "127.0.0.1:6379",
			Type: "node",
		})
		cfg.Cache = cache.CacheConf{
			{
				RedisConf: gzredis.RedisConf{
					Host: "127.0.0.1:6379",
					Type: "node",
				},
				Weight: 100,
			},
		}
	}

	return newProgramModels(conn, rds, cfg)
}

func requireModelUsesCachedConn(t *testing.T, model any, modelName string) {
	t.Helper()

	if !containsFieldOfType(reflect.ValueOf(model), reflect.TypeOf(sqlc.CachedConn{})) {
		t.Fatalf("expected %s to embed sqlc.CachedConn", modelName)
	}
}

func requireModelDoesNotUseCachedConn(t *testing.T, model any, modelName string) {
	t.Helper()

	if containsFieldOfType(reflect.ValueOf(model), reflect.TypeOf(sqlc.CachedConn{})) {
		t.Fatalf("expected %s to stay uncached", modelName)
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
