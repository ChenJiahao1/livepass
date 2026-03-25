package config

import "damai-go/pkg/xmysql"

type MySQLConfig = xmysql.Config

type RouteEntryConfig struct {
	Version     string `yaml:"Version"`
	LogicSlot   int    `yaml:"LogicSlot"`
	DBKey       string `yaml:"DBKey"`
	TableSuffix string `yaml:"TableSuffix"`
	Status      string `yaml:"Status"`
	WriteMode   string `yaml:"WriteMode"`
}

type RouteMapConfig struct {
	File    string             `json:",optional" yaml:"File"`
	Version string             `json:",optional" yaml:"Version"`
	Entries []RouteEntryConfig `json:",optional" yaml:"Entries"`
}

type BackfillConfig struct {
	BatchSize      int64  `json:",default=100" yaml:"BatchSize"`
	CheckpointFile string `json:",optional" yaml:"CheckpointFile"`
}

type VerifyConfig struct {
	SampleSize int64 `json:",default=10" yaml:"SampleSize"`
	Slots      []int `json:",optional" yaml:"Slots"`
}

type SwitchConfig struct {
	Slots []int `json:",optional" yaml:"Slots"`
}

type RollbackConfig struct {
	Slots []int `json:",optional" yaml:"Slots"`
}

type Config struct {
	LegacyMySQL MySQLConfig            `json:"LegacyMySQL" yaml:"LegacyMySQL"`
	Shards      map[string]MySQLConfig `json:",optional" yaml:"Shards"`
	RouteMap    RouteMapConfig         `json:"RouteMap" yaml:"RouteMap"`
	Backfill    BackfillConfig         `json:"Backfill,optional" yaml:"Backfill"`
	Verify      VerifyConfig           `json:"Verify,optional" yaml:"Verify"`
	Switch      SwitchConfig           `json:"Switch,optional" yaml:"Switch"`
	Rollback    RollbackConfig         `json:"Rollback,optional" yaml:"Rollback"`
}
