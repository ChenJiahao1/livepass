package svc

import (
	"livepass/jobs/order-close/internal/config"
	"livepass/jobs/order-close/internal/dispatch"
	"livepass/pkg/delaytask"
	"livepass/pkg/xmysql"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type DispatcherServiceContext struct {
	Config    config.Config
	Store     dispatch.Store
	Publisher delaytask.Publisher
}

func NewDispatcherServiceContext(c config.Config) *DispatcherServiceContext {
	return &DispatcherServiceContext{
		Config:    c,
		Store:     dispatch.NewMysqlStore(newShardMysqlConns(c.Shards)),
		Publisher: newDelayTaskPublisher(c.Asynq),
	}
}

func newShardMysqlConns(shards map[string]xmysql.Config) map[string]sqlx.SqlConn {
	if len(shards) == 0 {
		return nil
	}

	conns := make(map[string]sqlx.SqlConn, len(shards))
	for key, shardCfg := range shards {
		conns[key] = mustNewMysqlConn(shardCfg)
	}
	return conns
}

func newDelayTaskPublisher(cfg config.AsynqConfig) delaytask.Publisher {
	if cfg.Redis.Host == "" {
		return nil
	}

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Host,
		Username: cfg.Redis.User,
		Password: cfg.Redis.Pass,
	})
	return delaytask.NewAsynqPublisher(client, delaytask.Options{
		Queue:          cfg.Queue,
		MaxRetry:       cfg.MaxRetry,
		UniqueTTL:      cfg.UniqueTTL,
		EnqueueTimeout: cfg.EnqueueTimeout,
	})
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
