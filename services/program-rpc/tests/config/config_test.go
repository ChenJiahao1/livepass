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
