package config

import (
	"time"

	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/zrpc"
)

type OrderConfig struct {
	CloseAfter time.Duration `json:",default=15m"`
}

type RepeatGuardConfig struct {
	Prefix             string        `json:",default=/damai-go/repeat-guard/order-create/"`
	SessionTTL         int           `json:",default=10"`
	LockAcquireTimeout time.Duration `json:",default=200ms"`
}

type KafkaConfig struct {
	Brokers          []string `json:",optional"`
	TopicOrderCreate string   `json:",default=order.create.command.v1"`
	ConsumerGroup    string   `json:",default=damai-go-order-create"`
	TopicPartitions  int      `json:",default=1"`
	ConsumerWorkers  int      `json:",default=1"`
	// MaxMessageDelay follows the Java open-source flow:
	// once exceeded, the consumer treats the create command as an expired order,
	// releases the seat freeze, and skips order persistence.
	MaxMessageDelay time.Duration `json:",default=5s"`
	ProducerTimeout time.Duration `json:",default=3s"`
	RetryBackoff    time.Duration `json:",default=1s"`
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
	Version string             `json:",optional"`
	Entries []RouteEntryConfig `json:",optional"`
}

type ShardingConfig struct {
	Mode        string                   `json:",default=legacy_only"`
	LegacyMySQL xmysql.Config            `json:"LegacyMySQL,optional"`
	Shards      map[string]xmysql.Config `json:",optional"`
	RouteMap    RouteMapConfig           `json:"RouteMap,optional"`
}

func (c ShardingConfig) Normalize(legacyMySQL xmysql.Config) ShardingConfig {
	if c.Mode == "" {
		c.Mode = sharding.MigrationModeLegacyOnly
	}
	if c.LegacyMySQL.DataSource == "" {
		c.LegacyMySQL = legacyMySQL
	}
	c.LegacyMySQL = c.LegacyMySQL.Normalize()
	if c.Shards == nil {
		c.Shards = map[string]xmysql.Config{}
	}
	for key, shardCfg := range c.Shards {
		c.Shards[key] = shardCfg.Normalize()
	}

	return c
}

type Config struct {
	zrpc.RpcServerConf
	MySQL       xmysql.Config
	StoreRedis  xredis.Config `json:"StoreRedis,optional"`
	ProgramRpc  zrpc.RpcClientConf
	PayRpc      zrpc.RpcClientConf
	UserRpc     zrpc.RpcClientConf
	Order       OrderConfig
	RepeatGuard RepeatGuardConfig
	Kafka       KafkaConfig
	Sharding    ShardingConfig `json:"Sharding,optional"`
}
