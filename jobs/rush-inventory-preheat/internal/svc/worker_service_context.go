package svc

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"livepass/jobs/rush-inventory-preheat/internal/config"
	"livepass/pkg/xerr"
	"livepass/pkg/xmysql"
	orderrpc "livepass/services/order-rpc/orderrpc"
	programrpc "livepass/services/program-rpc/programrpc"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ShowTimeRecord struct {
	ProgramID              int64        `db:"program_id"`
	ID                     int64        `db:"id"`
	RushSaleOpenTime       sql.NullTime `db:"rush_sale_open_time"`
	InventoryPreheatStatus int64        `db:"inventory_preheat_status"`
}

type ShowTimeStore interface {
	ListByProgramID(ctx context.Context, programID int64) ([]*ShowTimeRecord, error)
	MarkInventoryPreheatedByProgramAndTaskProcessed(ctx context.Context, programID int64, expectedOpenTime time.Time, taskType, taskKey string, updatedAt time.Time) (bool, int64, int64, error)
	MarkTaskProcessed(ctx context.Context, taskType, taskKey string, processedAt time.Time) (int64, int64, error)
	MarkTaskConsumeFailed(ctx context.Context, taskType, taskKey string, failedAt time.Time, consumeErr string) (int64, int64, error)
}

type OrderPreheatRPC interface {
	PrimeRushRuntime(ctx context.Context, in *orderrpc.PrimeRushRuntimeReq) (*orderrpc.BoolResp, error)
}

type ProgramPreheatRPC interface {
	PrimeSeatLedger(ctx context.Context, in *programrpc.PrimeSeatLedgerReq) (*programrpc.BoolResp, error)
}

type WorkerServiceContext struct {
	Config        config.WorkerConfig
	Server        *asynq.Server
	ShowTimeStore ShowTimeStore
	OrderRpc      OrderPreheatRPC
	ProgramRpc    ProgramPreheatRPC
}

type mysqlShowTimeStore struct {
	conn sqlx.SqlConn
}

type orderRPCPrimeAdapter struct {
	client orderrpc.OrderRpc
}

type programRPCPrimeAdapter struct {
	client programrpc.ProgramRpc
}

func NewWorkerServiceContext(c config.WorkerConfig) *WorkerServiceContext {
	c.MySQL = c.MySQL.Normalize()
	c.MySQL.DataSource = xmysql.WithLocalTime(c.MySQL.DataSource)

	conn := sqlx.NewMysql(c.MySQL.DataSource)
	rawDB, err := conn.RawDB()
	if err != nil {
		panic(err)
	}
	xmysql.ApplyPool(rawDB, c.MySQL)

	return &WorkerServiceContext{
		Config: c,
		Server: asynq.NewServer(asynq.RedisClientOpt{
			Addr:     c.Asynq.Redis.Host,
			Username: c.Asynq.Redis.User,
			Password: c.Asynq.Redis.Pass,
		}, asynq.Config{
			Concurrency:     c.Asynq.Concurrency,
			Queues:          map[string]int{c.Asynq.Queue: 1},
			ShutdownTimeout: c.Asynq.ShutdownTimeout,
		}),
		ShowTimeStore: &mysqlShowTimeStore{conn: conn},
		OrderRpc: &orderRPCPrimeAdapter{
			client: orderrpc.NewOrderRpc(zrpc.MustNewClient(c.OrderRpc)),
		},
		ProgramRpc: &programRPCPrimeAdapter{
			client: programrpc.NewProgramRpc(zrpc.MustNewClient(c.ProgramRpc)),
		},
	}
}

func (s *mysqlShowTimeStore) ListByProgramID(ctx context.Context, programID int64) ([]*ShowTimeRecord, error) {
	var rows []*ShowTimeRecord
	err := s.conn.QueryRowsCtx(
		ctx,
		&rows,
		"select st.`program_id`, st.`id`, p.`rush_sale_open_time`, p.`inventory_preheat_status` from `d_program_show_time` st join `d_program` p on p.`id` = st.`program_id` and p.`status` = 1 where st.`program_id` = ? and st.`status` = 1 order by st.`show_time` asc, st.`id` asc",
		programID,
	)
	switch {
	case err == nil:
		return rows, nil
	case errors.Is(err, sqlx.ErrNotFound), errors.Is(err, sql.ErrNoRows):
		return nil, xerr.ErrProgramShowTimeNotFound
	default:
		return nil, err
	}
}

func (s *mysqlShowTimeStore) MarkInventoryPreheatedByProgramAndTaskProcessed(ctx context.Context, programID int64, expectedOpenTime time.Time, taskType, taskKey string, updatedAt time.Time) (bool, int64, int64, error) {
	var (
		updated         bool
		fromStatus      int64
		consumeAttempts int64
		found           bool
	)
	err := s.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		conn := sqlx.NewSqlConnFromSession(session)
		result, err := conn.ExecCtx(
			ctx,
			"update `d_program` set `inventory_preheat_status` = 2, `edit_time` = ? where `id` = ? and `status` = 1 and `rush_sale_open_time` = ?",
			updatedAt,
			programID,
			expectedOpenTime,
		)
		if err != nil {
			return err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		updated = rowsAffected > 0
		fromStatus, consumeAttempts, found, err = getOutboxConsumeState(ctx, conn, taskType, taskKey)
		if err != nil {
			return err
		}
		_, err = conn.ExecCtx(
			ctx,
			"update `d_delay_task_outbox` set `task_status` = 3, `consume_attempts` = `consume_attempts` + 1, `last_consume_error` = '', `processed_time` = ?, `edit_time` = ? where `task_type` = ? and `task_key` = ? and `status` = 1",
			updatedAt,
			updatedAt,
			taskType,
			taskKey,
		)
		return err
	})
	if err != nil {
		return false, 0, 0, err
	}

	if found {
		consumeAttempts++
	}
	return updated, fromStatus, consumeAttempts, nil
}

func (s *mysqlShowTimeStore) MarkTaskProcessed(ctx context.Context, taskType, taskKey string, processedAt time.Time) (int64, int64, error) {
	fromStatus, consumeAttempts, found, err := getOutboxConsumeState(ctx, s.conn, taskType, taskKey)
	if err != nil {
		return 0, 0, err
	}
	_, err = s.conn.ExecCtx(
		ctx,
		"update `d_delay_task_outbox` set `task_status` = 3, `consume_attempts` = `consume_attempts` + 1, `last_consume_error` = '', `processed_time` = ?, `edit_time` = ? where `task_type` = ? and `task_key` = ? and `status` = 1",
		processedAt,
		processedAt,
		taskType,
		taskKey,
	)
	if err != nil {
		return 0, 0, err
	}
	if found {
		consumeAttempts++
	}
	return fromStatus, consumeAttempts, nil
}

func (s *mysqlShowTimeStore) MarkTaskConsumeFailed(ctx context.Context, taskType, taskKey string, failedAt time.Time, consumeErr string) (int64, int64, error) {
	fromStatus, consumeAttempts, found, err := getOutboxConsumeState(ctx, s.conn, taskType, taskKey)
	if err != nil {
		return 0, 0, err
	}
	_, err = s.conn.ExecCtx(
		ctx,
		"update `d_delay_task_outbox` set `task_status` = 4, `consume_attempts` = `consume_attempts` + 1, `last_consume_error` = ?, `edit_time` = ? where `task_type` = ? and `task_key` = ? and `status` = 1",
		consumeErr,
		failedAt,
		taskType,
		taskKey,
	)
	if err != nil {
		return 0, 0, err
	}
	if found {
		consumeAttempts++
	}
	return fromStatus, consumeAttempts, nil
}

func getOutboxConsumeState(ctx context.Context, conn sqlx.SqlConn, taskType, taskKey string) (int64, int64, bool, error) {
	var row struct {
		TaskStatus      int64 `db:"task_status"`
		ConsumeAttempts int64 `db:"consume_attempts"`
	}
	err := conn.QueryRowCtx(
		ctx,
		&row,
		"select `task_status`, `consume_attempts` from `d_delay_task_outbox` where `task_type` = ? and `task_key` = ? and `status` = 1 limit 1",
		taskType,
		taskKey,
	)
	switch {
	case err == nil:
		return row.TaskStatus, row.ConsumeAttempts, true, nil
	case errors.Is(err, sqlx.ErrNotFound), errors.Is(err, sql.ErrNoRows):
		return 0, 0, false, nil
	default:
		return 0, 0, false, err
	}
}

func (a *orderRPCPrimeAdapter) PrimeRushRuntime(ctx context.Context, in *orderrpc.PrimeRushRuntimeReq) (*orderrpc.BoolResp, error) {
	return a.client.PrimeRushRuntime(ctx, in)
}

func (a *programRPCPrimeAdapter) PrimeSeatLedger(ctx context.Context, in *programrpc.PrimeSeatLedgerReq) (*programrpc.BoolResp, error) {
	return a.client.PrimeSeatLedger(ctx, in)
}
