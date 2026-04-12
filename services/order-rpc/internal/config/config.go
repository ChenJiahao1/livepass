package config

import (
	"path/filepath"
	"time"

	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/zrpc"
)

type OrderConfig struct {
	CloseAfter time.Duration `json:",default=15m"`
}

type RushOrderConfig struct {
	Enabled       bool          `json:",default=false"`
	TokenSecret   string        `json:",optional"`
	TokenTTL      time.Duration `json:",default=2m"`
	InFlightTTL   time.Duration `json:",default=30s"`
	FinalStateTTL time.Duration `json:",default=30m"`
}

type RepeatGuardConfig struct {
	Prefix             string        `json:",default=/damai-go/repeat-guard/order-create/"`
	SessionTTL         int           `json:",default=10"`
	LockAcquireTimeout time.Duration `json:",default=200ms"`
}

type KafkaConfig struct {
	Brokers          []string      `json:",optional"`
	TopicOrderCreate string        `json:",default=ticketing.attempt.command.v1"`
	ConsumerGroup    string        `json:",default=damai-go-ticketing-attempt"`
	TopicPartitions  int           `json:",default=1"`
	ConsumerWorkers  int           `json:",default=1"`
	ProducerTimeout  time.Duration `json:",default=3s"`
	RetryBackoff     time.Duration `json:",default=1s"`
}

type AsyncCloseConfig struct {
	Enable         bool          `json:",default=false"`
	Queue          string        `json:",default=order_close"`
	EnqueueTimeout time.Duration `json:",default=500ms"`
	UniqueTTL      time.Duration `json:",default=30m"`
	MaxRetry       int           `json:",default=8"`
	Redis          xredis.Config `json:"Redis,optional"`
}

type RouteEntryConfig struct {
	Version     string
	LogicSlot   int
	DBKey       string
	TableSuffix string
	Status      string
	WriteMode   string
}

type RouteMapConfig struct {
	File    string             `json:",optional"`
	Version string             `json:",optional"`
	Entries []RouteEntryConfig `json:",optional"`
}

type ShardingConfig struct {
	Mode     string                   `json:",default=shard_only"`
	Shards   map[string]xmysql.Config `json:",optional"`
	RouteMap RouteMapConfig           `json:"RouteMap,optional"`
}

func (c ShardingConfig) Normalize() ShardingConfig {
	if c.Mode == "" {
		c.Mode = "shard_only"
	}
	if c.Shards == nil {
		c.Shards = map[string]xmysql.Config{}
	}
	for key, shardCfg := range c.Shards {
		c.Shards[key] = shardCfg.Normalize()
	}

	return c
}

type XidConf struct {
	Provider          string `json:",default=static"`
	NodeId            int64  `json:",optional"`
	ServiceBaseNodeId int64  `json:",optional"`
	MaxReplicas       int64  `json:",optional"`
	PodName           string `json:",optional"`
}

type Config struct {
	zrpc.RpcServerConf
	MySQL       xmysql.Config
	StoreRedis  xredis.Config `json:"StoreRedis,optional"`
	ProgramRpc  zrpc.RpcClientConf
	PayRpc      zrpc.RpcClientConf
	UserRpc     zrpc.RpcClientConf
	Order       OrderConfig
	RushOrder   RushOrderConfig `json:"RushOrder,optional"`
	RepeatGuard RepeatGuardConfig
	Kafka       KafkaConfig
	AsyncClose  AsyncCloseConfig `json:"AsyncClose,optional"`
	Sharding    ShardingConfig   `json:"Sharding,optional"`
	Xid         XidConf          `json:"Xid,optional"`
}

func Load(configFile string) (Config, error) {
	var c Config
	if err := conf.Load(configFile, &c); err != nil {
		return Config{}, err
	}
	if err := loadRouteMapFile(filepath.Dir(configFile), &c.Sharding.RouteMap); err != nil {
		return Config{}, err
	}
	return c, nil
}

func loadRouteMapFile(baseDir string, routeMap *RouteMapConfig) error {
	if routeMap == nil || routeMap.File == "" {
		return nil
	}

	routeMapFile := routeMap.File
	if !filepath.IsAbs(routeMapFile) {
		routeMapFile = filepath.Join(baseDir, routeMapFile)
	}

	var loaded RouteMapConfig
	if err := conf.Load(routeMapFile, &loaded); err != nil {
		return err
	}

	routeMap.File = routeMapFile
	routeMap.Version = loaded.Version
	routeMap.Entries = loaded.Entries
	return nil
}
