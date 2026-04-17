package config_test

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"livepass/services/order-rpc/internal/config"
	"livepass/services/order-rpc/sharding"
)

func TestLoadOrderRPCRuntimeConfigIncludesTimeoutBudgetAndMySQLPool(t *testing.T) {
	t.Parallel()

	configFile := filepath.Join("..", "..", "etc", "order.yaml")
	c, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Timeout != 5500 {
		t.Fatalf("expected order-rpc runtime timeout 5500, got %d", c.Timeout)
	}
	if c.ProgramRpc.Timeout != 4000 {
		t.Fatalf("expected program rpc timeout 4000, got %d", c.ProgramRpc.Timeout)
	}
	if c.UserRpc.Timeout != 4000 {
		t.Fatalf("expected user rpc timeout 4000, got %d", c.UserRpc.Timeout)
	}
	if c.PayRpc.Timeout != 4000 {
		t.Fatalf("expected pay rpc timeout 4000, got %d", c.PayRpc.Timeout)
	}
	if c.MySQL.MaxOpenConns != 24 {
		t.Fatalf("expected mysql max open conns 24, got %d", c.MySQL.MaxOpenConns)
	}
	if c.MySQL.MaxIdleConns != 8 {
		t.Fatalf("expected mysql max idle conns 8, got %d", c.MySQL.MaxIdleConns)
	}
	if c.MySQL.ConnMaxLifetime != 3*time.Minute {
		t.Fatalf("expected mysql conn max lifetime 3m, got %s", c.MySQL.ConnMaxLifetime)
	}
	if c.MySQL.ConnMaxIdleTime != time.Minute {
		t.Fatalf("expected mysql conn max idle time 1m, got %s", c.MySQL.ConnMaxIdleTime)
	}
	if c.Kafka.TopicPartitions != 5 {
		t.Fatalf("expected kafka topic partitions 5, got %d", c.Kafka.TopicPartitions)
	}
	if c.Kafka.ConsumerWorkers != 1 {
		t.Fatalf("expected kafka consumer workers 1, got %d", c.Kafka.ConsumerWorkers)
	}
	if !c.RushOrder.Enabled {
		t.Fatalf("expected rush order to be enabled in runtime config")
	}
	if c.RushOrder.TokenSecret != "rush-order-local-secret" {
		t.Fatalf("expected rush order token secret rush-order-local-secret, got %q", c.RushOrder.TokenSecret)
	}
	if c.RushOrder.TokenTTL != 2*time.Minute {
		t.Fatalf("expected rush order token ttl 2m, got %s", c.RushOrder.TokenTTL)
	}
	if c.RushOrder.InFlightTTL != 30*time.Second {
		t.Fatalf("expected rush order inflight ttl 30s, got %s", c.RushOrder.InFlightTTL)
	}
	if c.RushOrder.FinalStateTTL != 30*time.Minute {
		t.Fatalf("expected rush order final state ttl 30m, got %s", c.RushOrder.FinalStateTTL)
	}
	if c.Sharding.Mode != "shard_only" {
		t.Fatalf("expected sharding mode shard_only, got %s", c.Sharding.Mode)
	}
	if c.Sharding.RouteMap.Version != "v1" {
		t.Fatalf("expected sharding route map version v1, got %s", c.Sharding.RouteMap.Version)
	}
	if len(c.Sharding.Shards) != 2 {
		t.Fatalf("expected 2 sharding mysql configs, got %d", len(c.Sharding.Shards))
	}
	if _, ok := c.Sharding.Shards["order-db-0"]; !ok {
		t.Fatalf("expected shard config order-db-0 to exist")
	}
	if len(c.Sharding.RouteMap.Entries) != 1024 {
		t.Fatalf("expected 1024 route map entries, got %d", len(c.Sharding.RouteMap.Entries))
	}

	routeMap, err := sharding.NewRouteMap(c.Sharding.RouteMap.Version, toRouteEntries(c.Sharding.RouteMap.Entries))
	if err != nil {
		t.Fatalf("build route map from runtime config: %v", err)
	}
	userID := int64(3001)
	route, err := routeMap.RouteByLogicSlot(sharding.LogicSlotByUserID(userID))
	if err != nil {
		t.Fatalf("route runtime config user %d: %v", userID, err)
	}
	if route.DBKey == "" || route.TableSuffix == "" {
		t.Fatalf("expected runtime config route for user %d to be complete, got %+v", userID, route)
	}
}

func TestLoadOrderRPCPerfConfigIncludesTimeoutBudgetAndMySQLPool(t *testing.T) {
	t.Parallel()

	configFile := filepath.Join("..", "..", "etc", "order.perf.yaml")
	c, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Timeout != 5500 {
		t.Fatalf("expected order-rpc perf timeout 5500, got %d", c.Timeout)
	}
	if c.ProgramRpc.Timeout != 4000 {
		t.Fatalf("expected program rpc timeout 4000, got %d", c.ProgramRpc.Timeout)
	}
	if c.UserRpc.Timeout != 4000 {
		t.Fatalf("expected user rpc timeout 4000, got %d", c.UserRpc.Timeout)
	}
	if c.PayRpc.Timeout != 4000 {
		t.Fatalf("expected pay rpc timeout 4000, got %d", c.PayRpc.Timeout)
	}
	if c.MySQL.MaxOpenConns != 24 {
		t.Fatalf("expected mysql max open conns 24, got %d", c.MySQL.MaxOpenConns)
	}
	if c.MySQL.MaxIdleConns != 8 {
		t.Fatalf("expected mysql max idle conns 8, got %d", c.MySQL.MaxIdleConns)
	}
	if c.MySQL.ConnMaxLifetime != 5*time.Minute {
		t.Fatalf("expected mysql conn max lifetime 5m, got %s", c.MySQL.ConnMaxLifetime)
	}
	if c.MySQL.ConnMaxIdleTime != 2*time.Minute {
		t.Fatalf("expected mysql conn max idle time 2m, got %s", c.MySQL.ConnMaxIdleTime)
	}
	if c.Kafka.TopicPartitions != 5 {
		t.Fatalf("expected kafka topic partitions 5, got %d", c.Kafka.TopicPartitions)
	}
	if c.Kafka.ConsumerWorkers != 1 {
		t.Fatalf("expected kafka consumer workers 1, got %d", c.Kafka.ConsumerWorkers)
	}
	if !c.RushOrder.Enabled {
		t.Fatalf("expected rush order to be enabled in perf config")
	}
	if c.RushOrder.TokenSecret != "rush-order-local-secret" {
		t.Fatalf("expected rush order token secret rush-order-local-secret, got %q", c.RushOrder.TokenSecret)
	}
	if c.RushOrder.TokenTTL != 2*time.Minute {
		t.Fatalf("expected rush order token ttl 2m, got %s", c.RushOrder.TokenTTL)
	}
	if c.RushOrder.InFlightTTL != 30*time.Second {
		t.Fatalf("expected rush order inflight ttl 30s, got %s", c.RushOrder.InFlightTTL)
	}
	if c.RushOrder.FinalStateTTL != 30*time.Minute {
		t.Fatalf("expected rush order final state ttl 30m, got %s", c.RushOrder.FinalStateTTL)
	}
	if c.Sharding.Mode != "shard_only" {
		t.Fatalf("expected perf sharding mode shard_only, got %s", c.Sharding.Mode)
	}
	if len(c.Sharding.Shards) != 2 {
		t.Fatalf("expected 2 perf sharding mysql configs, got %d", len(c.Sharding.Shards))
	}
	if c.Sharding.RouteMap.Version != "v1" {
		t.Fatalf("expected perf sharding route map version v1, got %s", c.Sharding.RouteMap.Version)
	}
	if len(c.Sharding.RouteMap.Entries) != 1024 {
		t.Fatalf("expected perf route map entries 1024, got %d", len(c.Sharding.RouteMap.Entries))
	}
}

func TestLoadOrderMCPConfigIncludesShardingRouteMapAndShards(t *testing.T) {
	t.Parallel()

	configFile := filepath.Join("..", "..", "etc", "order-mcp.yaml")
	c, err := config.LoadMCP(configFile)
	if err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Sharding.Mode != "shard_only" {
		t.Fatalf("expected order-mcp sharding mode shard_only, got %s", c.Sharding.Mode)
	}
	if len(c.Sharding.Shards) != 2 {
		t.Fatalf("expected 2 order-mcp sharding mysql configs, got %d", len(c.Sharding.Shards))
	}
	if c.Sharding.RouteMap.Version != "v1" {
		t.Fatalf("expected order-mcp sharding route map version v1, got %s", c.Sharding.RouteMap.Version)
	}
	if len(c.Sharding.RouteMap.Entries) != 1024 {
		t.Fatalf("expected order-mcp route map entries 1024, got %d", len(c.Sharding.RouteMap.Entries))
	}

	routeMap, err := sharding.NewRouteMap(c.Sharding.RouteMap.Version, toRouteEntries(c.Sharding.RouteMap.Entries))
	if err != nil {
		t.Fatalf("build route map from mcp config: %v", err)
	}
	userID := int64(3001)
	route, err := routeMap.RouteByLogicSlot(sharding.LogicSlotByUserID(userID))
	if err != nil {
		t.Fatalf("route mcp config user %d: %v", userID, err)
	}
	if route.DBKey == "" || route.TableSuffix == "" {
		t.Fatalf("expected mcp config route for user %d to be complete, got %+v", userID, route)
	}
}

func TestOrderCreateAcceptAsyncConfigRemovesLegacyTimeDrivenFields(t *testing.T) {
	t.Parallel()

	rushType := reflect.TypeOf(config.RushOrderConfig{})
	if _, ok := rushType.FieldByName("CommitCutoff"); ok {
		t.Fatalf("RushOrderConfig should not keep CommitCutoff")
	}
	if _, ok := rushType.FieldByName("UserDeadline"); ok {
		t.Fatalf("RushOrderConfig should not keep UserDeadline")
	}

	kafkaType := reflect.TypeOf(config.KafkaConfig{})
	if _, ok := kafkaType.FieldByName("MaxMessageDelay"); ok {
		t.Fatalf("KafkaConfig should not keep MaxMessageDelay")
	}
}

func TestLoadOrderRPCConfigIncludesStaticXid(t *testing.T) {
	t.Parallel()

	configFile := filepath.Join("..", "..", "etc", "order.yaml")
	c, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Xid.Provider != "static" {
		t.Fatalf("expected xid provider static, got %q", c.Xid.Provider)
	}
	if c.Xid.NodeId != 256 {
		t.Fatalf("expected xid node id 256, got %d", c.Xid.NodeId)
	}
	if len(c.Etcd.Hosts) == 0 || c.Etcd.Key == "" {
		t.Fatal("expected server etcd config to remain for service discovery")
	}
	if len(c.ProgramRpc.Etcd.Hosts) == 0 || c.ProgramRpc.Etcd.Key == "" {
		t.Fatal("expected downstream rpc etcd config to remain")
	}
	if c.RepeatGuard.Prefix == "" {
		t.Fatal("expected repeat guard config to remain")
	}
}

func toRouteEntries(entries []config.RouteEntryConfig) []sharding.RouteEntry {
	routeEntries := make([]sharding.RouteEntry, 0, len(entries))
	for _, entry := range entries {
		routeEntries = append(routeEntries, sharding.RouteEntry{
			Version:     entry.Version,
			LogicSlot:   entry.LogicSlot,
			DBKey:       entry.DBKey,
			TableSuffix: entry.TableSuffix,
			Status:      entry.Status,
			WriteMode:   entry.WriteMode,
		})
	}

	return routeEntries
}
