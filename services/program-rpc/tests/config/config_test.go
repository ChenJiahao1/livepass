package config_test

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/conf"

	"damai-go/services/program-rpc/internal/config"
)

func TestLoadProgramRPCConfigUsesDedicatedListenPort(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "program-rpc.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.ListenOn != "0.0.0.0:8083" {
		t.Fatalf("expected dedicated program-rpc listen address 0.0.0.0:8083, got %q", c.ListenOn)
	}
	if c.MySQL.MaxOpenConns != 14 {
		t.Fatalf("expected mysql max open conns 14, got %d", c.MySQL.MaxOpenConns)
	}
	if c.MySQL.MaxIdleConns != 4 {
		t.Fatalf("expected mysql max idle conns 4, got %d", c.MySQL.MaxIdleConns)
	}

	cacheField := reflect.ValueOf(c).FieldByName("Cache")
	if !cacheField.IsValid() {
		t.Fatalf("expected config.Config to expose Cache for model L2 cache configuration")
	}
	if cacheField.Len() != 1 {
		t.Fatalf("expected one cache node, got %d", cacheField.Len())
	}

	cacheNode := cacheField.Index(0)
	if host := requireStringField(t, cacheNode, "Host"); host != "127.0.0.1:6379" {
		t.Fatalf("expected cache node host 127.0.0.1:6379, got %q", host)
	}
	if redisType := requireStringField(t, cacheNode, "Type"); redisType != "node" {
		t.Fatalf("expected cache node type node, got %q", redisType)
	}

	localCacheField := reflect.ValueOf(c).FieldByName("LocalCache")
	if !localCacheField.IsValid() {
		t.Fatalf("expected config.Config to expose LocalCache settings")
	}

	assertDurationField(t, localCacheField, "DetailTTL", 20*time.Second)
	assertDurationField(t, localCacheField, "DetailNotFoundTTL", 5*time.Second)
	assertIntField(t, localCacheField, "DetailCacheLimit", 5000)
	assertDurationField(t, localCacheField, "CategorySnapshotTTL", 5*time.Minute)
}

func TestLoadProgramRPCConfigExposesCacheInvalidationDefaults(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "program-rpc.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	cacheInvalidationField := reflect.ValueOf(c).FieldByName("CacheInvalidation")
	if !cacheInvalidationField.IsValid() {
		t.Fatalf("expected config.Config to expose CacheInvalidation settings")
	}

	assertBoolField(t, cacheInvalidationField, "Enabled", true)
	if channel := requireStringField(t, cacheInvalidationField, "Channel"); channel != "damai-go:program:cache:invalidate" {
		t.Fatalf("expected cache invalidation channel damai-go:program:cache:invalidate, got %q", channel)
	}
	assertDurationField(t, cacheInvalidationField, "PublishTimeout", 200*time.Millisecond)
	assertDurationField(t, cacheInvalidationField, "ReconnectBackoff", time.Second)
}

func TestLoadProgramRPCConfigExposesRushInventoryPreheatDefaults(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "program-rpc.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if !c.RushInventoryPreheat.Enable {
		t.Fatalf("expected rush inventory preheat enabled in runtime config")
	}
	if c.RushInventoryPreheat.LeadTime != 5*time.Minute {
		t.Fatalf("expected rush inventory preheat lead time 5m, got %s", c.RushInventoryPreheat.LeadTime)
	}
	if c.RushInventoryPreheat.Queue != "rush_inventory_preheat" {
		t.Fatalf("expected rush inventory preheat queue rush_inventory_preheat, got %s", c.RushInventoryPreheat.Queue)
	}
	if c.RushInventoryPreheat.MaxRetry != 8 {
		t.Fatalf("expected rush inventory preheat max retry 8, got %d", c.RushInventoryPreheat.MaxRetry)
	}
	if c.RushInventoryPreheat.UniqueTTL != 30*time.Minute {
		t.Fatalf("expected rush inventory preheat unique ttl 30m, got %s", c.RushInventoryPreheat.UniqueTTL)
	}
	if c.RushInventoryPreheat.Redis.Host != "127.0.0.1:6379" {
		t.Fatalf("expected rush inventory preheat redis host 127.0.0.1:6379, got %q", c.RushInventoryPreheat.Redis.Host)
	}
	if c.RushInventoryPreheat.Redis.Type != "node" {
		t.Fatalf("expected rush inventory preheat redis type node, got %q", c.RushInventoryPreheat.Redis.Type)
	}
}

func TestLoadProgramRPCConfigIncludesStaticXid(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "program-rpc.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Xid.Provider != "static" {
		t.Fatalf("expected xid provider static, got %q", c.Xid.Provider)
	}
	if c.Xid.NodeId != 128 {
		t.Fatalf("expected xid node id 128, got %d", c.Xid.NodeId)
	}
	if len(c.Etcd.Hosts) == 0 || c.Etcd.Key == "" {
		t.Fatal("expected rpc etcd config to remain for service discovery")
	}
}

func assertDurationField(t *testing.T, value reflect.Value, name string, expected time.Duration) {
	t.Helper()

	field, ok := findStructField(value, name)
	if !ok {
		t.Fatalf("expected field %s to exist", name)
	}

	if got := time.Duration(field.Int()); got != expected {
		t.Fatalf("expected %s = %s, got %s", name, expected, got)
	}
}

func assertBoolField(t *testing.T, value reflect.Value, name string, expected bool) {
	t.Helper()

	field, ok := findStructField(value, name)
	if !ok {
		t.Fatalf("expected field %s to exist", name)
	}

	if got := field.Bool(); got != expected {
		t.Fatalf("expected %s = %v, got %v", name, expected, got)
	}
}

func assertIntField(t *testing.T, value reflect.Value, name string, expected int64) {
	t.Helper()

	field, ok := findStructField(value, name)
	if !ok {
		t.Fatalf("expected field %s to exist", name)
	}

	if got := field.Int(); got != expected {
		t.Fatalf("expected %s = %d, got %d", name, expected, got)
	}
}

func requireStringField(t *testing.T, value reflect.Value, name string) string {
	t.Helper()

	field, ok := findStructField(value, name)
	if !ok {
		t.Fatalf("expected field %s to exist", name)
	}

	return field.String()
}

func findStructField(value reflect.Value, name string) (reflect.Value, bool) {
	if !value.IsValid() {
		return reflect.Value{}, false
	}
	if value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		return findStructField(value.Elem(), name)
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	if field := value.FieldByName(name); field.IsValid() {
		return field, true
	}

	for i := 0; i < value.NumField(); i++ {
		if field, ok := findStructField(value.Field(i), name); ok {
			return field, true
		}
	}

	return reflect.Value{}, false
}
