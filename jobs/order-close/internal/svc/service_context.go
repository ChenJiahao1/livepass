package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"damai-go/jobs/order-close/internal/config"
	"damai-go/pkg/xmysql"
	"damai-go/services/order-rpc/closequeue"
	orderrpc "damai-go/services/order-rpc/orderrpc"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

type OutboxRef struct {
	DBKey string
	ID    int64
}

type PendingOrderCreatedOutbox struct {
	Ref         OutboxRef
	OrderNumber int64
	UserID      int64
	CreateTime  time.Time
}

type OutboxStore interface {
	ListPendingOrderCreatedOutboxes(ctx context.Context, limit int64) ([]PendingOrderCreatedOutbox, error)
	MarkOutboxPublished(ctx context.Context, ref OutboxRef, publishedAt time.Time) error
}

type AsyncCloseClient interface {
	EnqueueCloseTimeout(ctx context.Context, orderNumber int64, expireAt time.Time) error
}

type OrderCloseRPC interface {
	GetOrder(ctx context.Context, in *orderrpc.GetOrderReq, opts ...grpc.CallOption) (*orderrpc.OrderDetailInfo, error)
	CloseExpiredOrder(ctx context.Context, in *orderrpc.CloseExpiredOrderReq, opts ...grpc.CallOption) (*orderrpc.BoolResp, error)
}

type ServiceContext struct {
	Config           config.Config
	OutboxStore      OutboxStore
	AsyncCloseClient AsyncCloseClient
	OrderRpc         OrderCloseRPC
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:           c,
		OutboxStore:      newMysqlOrderCloseOutboxStore(c.Shards),
		AsyncCloseClient: newAsyncCloseClient(c.Asynq),
		OrderRpc:         orderrpc.NewOrderRpc(zrpc.MustNewClient(c.OrderRpc)),
	}
}

type orderCloseOutboxRow struct {
	ID          int64     `db:"id"`
	OrderNumber int64     `db:"order_number"`
	Payload     string    `db:"payload"`
	CreateTime  time.Time `db:"create_time"`
}

type orderCloseOutboxPayload struct {
	OrderNumber int64 `json:"orderNumber"`
	UserID      int64 `json:"userId"`
}

type mysqlOrderCloseOutboxStore struct {
	conns map[string]sqlx.SqlConn
}

func newMysqlOrderCloseOutboxStore(shards map[string]xmysql.Config) OutboxStore {
	if len(shards) == 0 {
		return nil
	}

	conns := make(map[string]sqlx.SqlConn, len(shards))
	for key, shardCfg := range shards {
		conns[key] = mustNewMysqlConn(shardCfg)
	}
	return &mysqlOrderCloseOutboxStore{conns: conns}
}

func (s *mysqlOrderCloseOutboxStore) ListPendingOrderCreatedOutboxes(ctx context.Context, limit int64) ([]PendingOrderCreatedOutbox, error) {
	if s == nil || len(s.conns) == 0 || limit <= 0 {
		return nil, nil
	}

	shardKeys := make([]string, 0, len(s.conns))
	for key := range s.conns {
		shardKeys = append(shardKeys, key)
	}
	sort.Strings(shardKeys)

	items := make([]PendingOrderCreatedOutbox, 0, limit)
	for _, key := range shardKeys {
		var rows []orderCloseOutboxRow
		err := s.conns[key].QueryRowsCtx(
			ctx,
			&rows,
			"SELECT `id`, `order_number`, `payload`, `create_time` FROM `d_order_outbox` WHERE `published_status` = 0 AND `event_type` = ? AND `status` = 1 ORDER BY `id` ASC LIMIT ?",
			"order.created",
			limit,
		)
		switch {
		case err == nil:
		case errors.Is(err, sqlx.ErrNotFound):
			continue
		default:
			return nil, err
		}

		for _, row := range rows {
			payload := orderCloseOutboxPayload{}
			if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
				return nil, err
			}
			userID := payload.UserID
			if userID <= 0 {
				return nil, fmt.Errorf("order close outbox payload missing userId, outboxID=%d dbKey=%s", row.ID, key)
			}
			orderNumber := row.OrderNumber
			if payload.OrderNumber > 0 {
				orderNumber = payload.OrderNumber
			}
			items = append(items, PendingOrderCreatedOutbox{
				Ref: OutboxRef{
					DBKey: key,
					ID:    row.ID,
				},
				OrderNumber: orderNumber,
				UserID:      userID,
				CreateTime:  row.CreateTime,
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].CreateTime.Equal(items[j].CreateTime) {
			if items[i].Ref.ID == items[j].Ref.ID {
				return items[i].Ref.DBKey < items[j].Ref.DBKey
			}
			return items[i].Ref.ID < items[j].Ref.ID
		}
		return items[i].CreateTime.Before(items[j].CreateTime)
	})

	if int64(len(items)) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *mysqlOrderCloseOutboxStore) MarkOutboxPublished(ctx context.Context, ref OutboxRef, publishedAt time.Time) error {
	if s == nil {
		return nil
	}
	conn, ok := s.conns[ref.DBKey]
	if !ok {
		return fmt.Errorf("order close outbox shard not configured: %s", ref.DBKey)
	}

	_, err := conn.ExecCtx(
		ctx,
		"UPDATE `d_order_outbox` SET `published_status` = 1, `published_time` = ?, `edit_time` = ? WHERE `id` = ? AND `published_status` = 0 AND `status` = 1",
		publishedAt,
		publishedAt,
		ref.ID,
	)
	return err
}

type asynqEnqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type asynqAsyncCloseClient struct {
	enqueuer       asynqEnqueuer
	queue          string
	maxRetry       int
	uniqueTTL      time.Duration
	enqueueTimeout time.Duration
}

func newAsyncCloseClient(cfg config.AsynqConfig) AsyncCloseClient {
	if cfg.Redis.Host == "" {
		return nil
	}

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Host,
		Username: cfg.Redis.User,
		Password: cfg.Redis.Pass,
	})
	return &asynqAsyncCloseClient{
		enqueuer:       client,
		queue:          cfg.Queue,
		maxRetry:       cfg.MaxRetry,
		uniqueTTL:      cfg.UniqueTTL,
		enqueueTimeout: cfg.EnqueueTimeout,
	}
}

func (c *asynqAsyncCloseClient) EnqueueCloseTimeout(ctx context.Context, orderNumber int64, expireAt time.Time) error {
	if c.enqueueTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.enqueueTimeout)
		defer cancel()
	}

	body, err := closequeue.MarshalCloseTimeoutPayload(orderNumber, expireAt)
	if err != nil {
		return err
	}

	opts := []asynq.Option{
		asynq.Queue(c.queue),
		asynq.ProcessAt(expireAt),
		asynq.TaskID(closequeue.CloseTimeoutTaskID(orderNumber)),
	}
	if c.maxRetry > 0 {
		opts = append(opts, asynq.MaxRetry(c.maxRetry))
	}
	if c.uniqueTTL > 0 {
		opts = append(opts, asynq.Unique(c.uniqueTTL))
	}

	_, err = c.enqueuer.EnqueueContext(ctx, asynq.NewTask(closequeue.TaskTypeCloseTimeout, body), opts...)
	if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
		return nil
	}
	return err
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
