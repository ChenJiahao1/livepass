package svc

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"damai-go/jobs/rush-inventory-preheat/internal/config"
	"damai-go/pkg/xerr"
	"damai-go/pkg/xmysql"
	orderrpc "damai-go/services/order-rpc/orderrpc"
	programrpc "damai-go/services/program-rpc/programrpc"

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
	MarkInventoryPreheatedByProgram(ctx context.Context, programID int64, expectedOpenTime time.Time, updatedAt time.Time) (bool, error)
}

type OrderPreheatRPC interface {
	PrimeRushRuntime(ctx context.Context, in *orderrpc.PrimeRushRuntimeReq) (*orderrpc.BoolResp, error)
}

type ProgramPreheatRPC interface {
	PrimeSeatLedger(ctx context.Context, in *programrpc.PrimeSeatLedgerReq) (*programrpc.BoolResp, error)
}

type WorkerServiceContext struct {
	Config        config.Config
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

func NewWorkerServiceContext(c config.Config) *WorkerServiceContext {
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

func (s *mysqlShowTimeStore) MarkInventoryPreheatedByProgram(ctx context.Context, programID int64, expectedOpenTime time.Time, updatedAt time.Time) (bool, error) {
	result, err := s.conn.ExecCtx(
		ctx,
		"update `d_program` set `inventory_preheat_status` = 2, `edit_time` = ? where `id` = ? and `status` = 1 and `rush_sale_open_time` = ?",
		updatedAt,
		programID,
		expectedOpenTime,
	)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}

func (a *orderRPCPrimeAdapter) PrimeRushRuntime(ctx context.Context, in *orderrpc.PrimeRushRuntimeReq) (*orderrpc.BoolResp, error) {
	return a.client.PrimeRushRuntime(ctx, in)
}

func (a *programRPCPrimeAdapter) PrimeSeatLedger(ctx context.Context, in *programrpc.PrimeSeatLedgerReq) (*programrpc.BoolResp, error) {
	return a.client.PrimeSeatLedger(ctx, in)
}
