package svc

import (
	"fmt"
	"os"

	"damai-go/jobs/order-migrate/internal/config"
	"damai-go/pkg/xmysql"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"gopkg.in/yaml.v3"
)

type ServiceContext struct {
	Config         config.Config
	LegacySqlConn  sqlx.SqlConn
	ShardSqlConns  map[string]sqlx.SqlConn
	RouteMapConfig config.RouteMapConfig
	RouteMap       *sharding.RouteMap
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	c.LegacyMySQL = c.LegacyMySQL.Normalize()
	if c.Shards == nil {
		c.Shards = map[string]config.MySQLConfig{}
	}
	for key, shardCfg := range c.Shards {
		c.Shards[key] = shardCfg.Normalize()
	}

	routeMapConfig, routeMap, err := loadRouteMap(c.RouteMap)
	if err != nil {
		return nil, err
	}
	if err := validateRouteMapShards(routeMapConfig, c.Shards); err != nil {
		return nil, err
	}

	shardConns := make(map[string]sqlx.SqlConn, len(c.Shards))
	for key, shardCfg := range c.Shards {
		shardConns[key] = mustNewMysqlConn(shardCfg)
	}

	return &ServiceContext{
		Config:         c,
		LegacySqlConn:  mustNewMysqlConn(c.LegacyMySQL),
		ShardSqlConns:  shardConns,
		RouteMapConfig: routeMapConfig,
		RouteMap:       routeMap,
	}, nil
}

func (s *ServiceContext) SaveRouteMapConfig(routeMapConfig config.RouteMapConfig) error {
	if routeMapConfig.File == "" {
		routeMapConfig.File = s.RouteMapConfig.File
	}
	content, err := yaml.Marshal(routeMapConfig)
	if err != nil {
		return err
	}
	if err := os.WriteFile(routeMapConfig.File, content, 0o644); err != nil {
		return err
	}

	loadedConfig, routeMap, err := loadRouteMap(routeMapConfig)
	if err != nil {
		return err
	}
	s.RouteMapConfig = loadedConfig
	s.RouteMap = routeMap
	s.Config.RouteMap = loadedConfig
	return nil
}

func loadRouteMap(routeMapConfig config.RouteMapConfig) (config.RouteMapConfig, *sharding.RouteMap, error) {
	loadedConfig := routeMapConfig
	if routeMapConfig.File != "" {
		content, err := os.ReadFile(routeMapConfig.File)
		if err != nil {
			return config.RouteMapConfig{}, nil, err
		}
		if err := yaml.Unmarshal(content, &loadedConfig); err != nil {
			return config.RouteMapConfig{}, nil, err
		}
		loadedConfig.File = routeMapConfig.File
	}

	entries := make([]sharding.RouteEntry, 0, len(loadedConfig.Entries))
	for _, entry := range loadedConfig.Entries {
		entries = append(entries, sharding.RouteEntry{
			Version:     entry.Version,
			LogicSlot:   entry.LogicSlot,
			DBKey:       entry.DBKey,
			TableSuffix: entry.TableSuffix,
			Status:      entry.Status,
			WriteMode:   entry.WriteMode,
		})
	}

	routeMap, err := sharding.NewRouteMap(loadedConfig.Version, entries)
	if err != nil {
		return config.RouteMapConfig{}, nil, err
	}

	return loadedConfig, routeMap, nil
}

func validateRouteMapShards(routeMapConfig config.RouteMapConfig, shards map[string]config.MySQLConfig) error {
	for _, entry := range routeMapConfig.Entries {
		if _, ok := shards[entry.DBKey]; ok {
			continue
		}
		return fmt.Errorf("shard db key not configured for logic slot %d: %s", entry.LogicSlot, entry.DBKey)
	}
	return nil
}

func mustNewMysqlConn(cfg xmysql.Config) sqlx.SqlConn {
	cfg = cfg.Normalize()
	cfg.DataSource = xmysql.WithLocalTime(cfg.DataSource)

	conn := sqlx.NewMysql(cfg.DataSource)
	rawDB, err := conn.RawDB()
	if err != nil {
		panic(err)
	}
	xmysql.ApplyPool(rawDB, cfg)
	return conn
}
