package limitcache

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xredis"
)

const (
	defaultPurchaseLimitLedgerTTL       = 4 * time.Hour
	defaultPurchaseLimitLoadingCooldown = 3 * time.Second
)

type Config struct {
	Prefix          string
	LedgerTTL       time.Duration
	LoadingCooldown time.Duration
}

type PurchaseLimitSnapshot struct {
	Ready        bool
	Loading      bool
	ActiveCount  int64
	Reservations map[int64]int64
}

type PurchaseLimitStore struct {
	redis            *xredis.Client
	prefix           string
	ledgerTTLSeconds int
	loader           *purchaseLimitLoader
}

func NewPurchaseLimitStore(redis *xredis.Client, orderModel activeTicketCounter, cfg Config) *PurchaseLimitStore {
	if redis == nil {
		return nil
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = defaultPurchaseLimitPrefix
	}

	ledgerTTLSeconds := durationSeconds(cfg.LedgerTTL, defaultPurchaseLimitLedgerTTL)
	loadingCooldownSeconds := durationSeconds(cfg.LoadingCooldown, defaultPurchaseLimitLoadingCooldown)

	return &PurchaseLimitStore{
		redis:            redis,
		prefix:           prefix,
		ledgerTTLSeconds: ledgerTTLSeconds,
		loader:           newPurchaseLimitLoader(redis, orderModel, prefix, ledgerTTLSeconds, loadingCooldownSeconds),
	}
}

func (s *PurchaseLimitStore) Reserve(ctx context.Context, userID, programID, orderNumber, ticketCount, limit int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrOrderLimitLedgerNotReady
	}
	if limit <= 0 || ticketCount <= 0 {
		return nil
	}

	result, err := s.redis.EvalCtx(ctx, reservePurchaseLimitScript, []string{ledgerKey(s.prefix, userID, programID)}, orderNumber, ticketCount, limit, s.ledgerTTLSeconds)
	if err != nil {
		return err
	}

	code, err := parseEvalInt64(result)
	if err != nil {
		return err
	}

	switch code {
	case 1:
		return nil
	case 0:
		return xerr.ErrOrderPurchaseLimitExceeded
	case -1:
		if s.loader != nil {
			s.loader.Schedule(userID, programID)
		}
		return xerr.ErrOrderLimitLedgerNotReady
	default:
		return fmt.Errorf("unexpected reserve purchase limit result: %d", code)
	}
}

func (s *PurchaseLimitStore) Release(ctx context.Context, userID, programID, orderNumber int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrOrderLimitLedgerNotReady
	}

	result, err := s.redis.EvalCtx(ctx, releasePurchaseLimitScript, []string{ledgerKey(s.prefix, userID, programID)}, orderNumber, s.ledgerTTLSeconds)
	if err != nil {
		return err
	}

	code, err := parseEvalInt64(result)
	if err != nil {
		return err
	}

	switch code {
	case 1:
		return nil
	case -1:
		if s.loader != nil {
			s.loader.Schedule(userID, programID)
		}
		return xerr.ErrOrderLimitLedgerNotReady
	default:
		return fmt.Errorf("unexpected release purchase limit result: %d", code)
	}
}

func (s *PurchaseLimitStore) Seed(ctx context.Context, userID, programID, activeCount int64, reservations map[int64]int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrOrderLimitLedgerNotReady
	}

	key := ledgerKey(s.prefix, userID, programID)
	if _, err := s.redis.Del(key, loadingKey(s.prefix, userID, programID)); err != nil {
		return err
	}
	if err := s.redis.HsetCtx(ctx, key, purchaseLimitCountField, strconv.FormatInt(activeCount, 10)); err != nil {
		return err
	}
	for orderNumber, ticketCount := range reservations {
		if err := s.redis.HsetCtx(ctx, key, reservationField(orderNumber), strconv.FormatInt(ticketCount, 10)); err != nil {
			return err
		}
	}
	if s.ledgerTTLSeconds > 0 {
		return s.redis.ExpireCtx(ctx, key, s.ledgerTTLSeconds)
	}

	return nil
}

func (s *PurchaseLimitStore) Clear(ctx context.Context, userID, programID int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrOrderLimitLedgerNotReady
	}

	_, err := s.redis.DelCtx(ctx, ledgerKey(s.prefix, userID, programID), loadingKey(s.prefix, userID, programID))
	return err
}

func (s *PurchaseLimitStore) Snapshot(ctx context.Context, userID, programID int64) (*PurchaseLimitSnapshot, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrOrderLimitLedgerNotReady
	}

	key := ledgerKey(s.prefix, userID, programID)
	ready, err := s.redis.ExistsCtx(ctx, key)
	if err != nil {
		return nil, err
	}
	loading, err := s.redis.ExistsCtx(ctx, loadingKey(s.prefix, userID, programID))
	if err != nil {
		return nil, err
	}

	snapshot := &PurchaseLimitSnapshot{
		Ready:        ready,
		Loading:      loading,
		Reservations: make(map[int64]int64),
	}
	if !ready {
		return snapshot, nil
	}

	fields, err := s.redis.HgetallCtx(ctx, key)
	if err != nil {
		return nil, err
	}
	for field, value := range fields {
		if field == purchaseLimitCountField {
			snapshot.ActiveCount, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			continue
		}
		if !strings.HasPrefix(field, purchaseLimitReservationField) {
			continue
		}

		orderNumber, err := strconv.ParseInt(strings.TrimPrefix(field, purchaseLimitReservationField), 10, 64)
		if err != nil {
			return nil, err
		}
		ticketCount, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}
		snapshot.Reservations[orderNumber] = ticketCount
	}

	return snapshot, nil
}

func durationSeconds(value, defaultValue time.Duration) int {
	if value <= 0 {
		value = defaultValue
	}

	seconds := int(value / time.Second)
	if value%time.Second != 0 {
		seconds++
	}
	if seconds <= 0 {
		return 1
	}

	return seconds
}

func parseEvalInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported eval result type %T", value)
	}
}
