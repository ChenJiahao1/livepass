package config

import (
	"time"

	"damai-go/pkg/xmysql"

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
	Brokers          []string      `json:",optional"`
	TopicOrderCreate string        `json:",default=order.create.command.v1"`
	ConsumerGroup    string        `json:",default=damai-go-order-create"`
	MaxMessageDelay  time.Duration `json:",default=5s"`
	ProducerTimeout  time.Duration `json:",default=3s"`
	RetryBackoff     time.Duration `json:",default=1s"`
}

type Config struct {
	zrpc.RpcServerConf
	MySQL       xmysql.Config
	ProgramRpc  zrpc.RpcClientConf
	PayRpc      zrpc.RpcClientConf
	UserRpc     zrpc.RpcClientConf
	Order       OrderConfig
	RepeatGuard RepeatGuardConfig
	Kafka       KafkaConfig
}
