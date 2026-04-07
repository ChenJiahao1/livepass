package seatcache

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xredis"
	"damai-go/services/program-rpc/internal/model"
)

const (
	defaultSeatLedgerStockTTL        = 4 * time.Hour
	defaultSeatLedgerSeatTTL         = 4 * time.Hour
	defaultSeatLedgerLoadingCooldown = 3 * time.Second

	seatStatusAvailable int64 = 1
	seatStatusFrozen    int64 = 2
	seatStatusSold      int64 = 3
)

type (
	redisClient = *xredis.Client

	Config struct {
		Prefix          string
		StockTTL        time.Duration
		SeatTTL         time.Duration
		LoadingCooldown time.Duration
	}

	Seat struct {
		SeatID           int64
		TicketCategoryID int64
		RowCode          int64
		ColCode          int64
		Price            int64
	}

	SeatLedgerSnapshot struct {
		Ready          bool
		Loading        bool
		AvailableCount int64
		AvailableSeats []Seat
		SoldSeats      []Seat
		FrozenSeats    map[string][]Seat
	}

	SeatStockStore struct {
		redis           redisClient
		prefix          string
		stockTTLSeconds int
		seatTTLSeconds  int
		loader          *seatStockLoader
	}
)

func NewSeatStockStore(redis redisClient, seatModel seatLedgerSource, cfg Config) *SeatStockStore {
	if redis == nil {
		return nil
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = defaultSeatLedgerPrefix
	}

	stockTTLSeconds := durationSeconds(cfg.StockTTL, defaultSeatLedgerStockTTL)
	seatTTLSeconds := durationSeconds(cfg.SeatTTL, defaultSeatLedgerSeatTTL)
	loadingCooldownSeconds := durationSeconds(cfg.LoadingCooldown, defaultSeatLedgerLoadingCooldown)

	return &SeatStockStore{
		redis:           redis,
		prefix:          prefix,
		stockTTLSeconds: stockTTLSeconds,
		seatTTLSeconds:  seatTTLSeconds,
		loader:          newSeatStockLoader(redis, seatModel, prefix, stockTTLSeconds, seatTTLSeconds, loadingCooldownSeconds),
	}
}

func (s *SeatStockStore) FreezeAutoAssignedSeats(ctx context.Context, showTimeID, ticketCategoryID int64, freezeToken string, count int64) ([]Seat, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrProgramSeatLedgerNotReady
	}

	result, err := s.redis.EvalCtx(
		ctx,
		freezeAutoAssignedSeatsScript,
		[]string{
			stockKey(s.prefix, showTimeID, ticketCategoryID),
			availableSeatsKey(s.prefix, showTimeID, ticketCategoryID),
			frozenSeatsKey(s.prefix, showTimeID, ticketCategoryID, freezeToken),
		},
		count,
		s.stockTTLSeconds,
		s.seatTTLSeconds,
	)
	if err != nil {
		return nil, err
	}

	values, err := parseEvalStrings(result)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("unexpected empty freeze seat result")
	}

	code, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return nil, err
	}

	switch code {
	case 1:
		seats := make([]Seat, 0, len(values)-1)
		for _, value := range values[1:] {
			seat, err := decodeSeatMember(value)
			if err != nil {
				return nil, err
			}
			seats = append(seats, seat)
		}
		return seats, nil
	case 0:
		return nil, xerr.ErrSeatInventoryInsufficient
	case -1:
		if s.loader != nil {
			s.loader.Schedule(showTimeID, ticketCategoryID)
		}
		return nil, xerr.ErrProgramSeatLedgerNotReady
	default:
		return nil, fmt.Errorf("unexpected freeze seat result: %d", code)
	}
}

func (s *SeatStockStore) ReleaseFrozenSeats(ctx context.Context, showTimeID, ticketCategoryID int64, freezeToken string, ownerOrderNumber, ownerEpoch int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrProgramSeatLedgerNotReady
	}

	result, err := s.redis.EvalCtx(
		ctx,
		releaseFrozenSeatsScript,
		[]string{
			stockKey(s.prefix, showTimeID, ticketCategoryID),
			availableSeatsKey(s.prefix, showTimeID, ticketCategoryID),
			frozenSeatsKey(s.prefix, showTimeID, ticketCategoryID, freezeToken),
			freezeMetaKey(s.prefix, freezeToken),
		},
		s.stockTTLSeconds,
		s.seatTTLSeconds,
		ownerOrderNumber,
		ownerEpoch,
	)
	if err != nil {
		return err
	}

	code, err := parseEvalInt64(result)
	if err != nil {
		return err
	}
	if code == -1 {
		if s.loader != nil {
			s.loader.Schedule(showTimeID, ticketCategoryID)
		}
		return xerr.ErrProgramSeatLedgerNotReady
	}
	if code == -2 {
		return xerr.ErrSeatFreezeStatusInvalid
	}
	if code != 1 {
		return fmt.Errorf("unexpected release frozen seat result: %d", code)
	}

	return nil
}

func (s *SeatStockStore) ConfirmFrozenSeats(ctx context.Context, showTimeID, ticketCategoryID int64, freezeToken string, ownerOrderNumber, ownerEpoch int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrProgramSeatLedgerNotReady
	}

	result, err := s.redis.EvalCtx(
		ctx,
		confirmFrozenSeatsScript,
		[]string{
			stockKey(s.prefix, showTimeID, ticketCategoryID),
			soldSeatsKey(s.prefix, showTimeID, ticketCategoryID),
			frozenSeatsKey(s.prefix, showTimeID, ticketCategoryID, freezeToken),
			freezeMetaKey(s.prefix, freezeToken),
		},
		s.stockTTLSeconds,
		s.seatTTLSeconds,
		ownerOrderNumber,
		ownerEpoch,
	)
	if err != nil {
		return err
	}

	code, err := parseEvalInt64(result)
	if err != nil {
		return err
	}
	if code == -1 {
		if s.loader != nil {
			s.loader.Schedule(showTimeID, ticketCategoryID)
		}
		return xerr.ErrProgramSeatLedgerNotReady
	}
	if code == -2 {
		return xerr.ErrSeatFreezeStatusInvalid
	}
	if code != 1 {
		return fmt.Errorf("unexpected confirm frozen seat result: %d", code)
	}

	return nil
}

func (s *SeatStockStore) PrimeFromDB(ctx context.Context, showTimeID, ticketCategoryID int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrProgramSeatLedgerNotReady
	}
	if s.loader == nil {
		return xerr.ErrProgramSeatLedgerNotReady
	}

	return s.loader.LoadSync(ctx, showTimeID, ticketCategoryID)
}

func (s *SeatStockStore) Clear(ctx context.Context, showTimeID, ticketCategoryID int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrProgramSeatLedgerNotReady
	}

	keys := []string{
		stockKey(s.prefix, showTimeID, ticketCategoryID),
		availableSeatsKey(s.prefix, showTimeID, ticketCategoryID),
		soldSeatsKey(s.prefix, showTimeID, ticketCategoryID),
		loadingKey(s.prefix, showTimeID, ticketCategoryID),
	}
	frozenKeys, err := s.redis.KeysCtx(ctx, frozenSeatsPattern(s.prefix, showTimeID, ticketCategoryID))
	if err != nil {
		return err
	}
	keys = append(keys, frozenKeys...)

	_, err = s.redis.DelCtx(ctx, keys...)
	return err
}

func (s *SeatStockStore) Snapshot(ctx context.Context, showTimeID, ticketCategoryID int64) (*SeatLedgerSnapshot, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrProgramSeatLedgerNotReady
	}

	ready, err := s.redis.ExistsCtx(ctx, stockKey(s.prefix, showTimeID, ticketCategoryID))
	if err != nil {
		return nil, err
	}
	loading, err := s.redis.ExistsCtx(ctx, loadingKey(s.prefix, showTimeID, ticketCategoryID))
	if err != nil {
		return nil, err
	}

	snapshot := &SeatLedgerSnapshot{
		Ready:       ready,
		Loading:     loading,
		FrozenSeats: make(map[string][]Seat),
	}
	if !ready {
		return snapshot, nil
	}

	availableCountRaw, err := s.redis.HgetCtx(ctx, stockKey(s.prefix, showTimeID, ticketCategoryID), seatStockAvailableCountField)
	if err != nil {
		return nil, err
	}
	snapshot.AvailableCount, err = strconv.ParseInt(availableCountRaw, 10, 64)
	if err != nil {
		return nil, err
	}

	snapshot.AvailableSeats, err = s.listSeats(ctx, availableSeatsKey(s.prefix, showTimeID, ticketCategoryID))
	if err != nil {
		return nil, err
	}
	snapshot.SoldSeats, err = s.listSeats(ctx, soldSeatsKey(s.prefix, showTimeID, ticketCategoryID))
	if err != nil {
		return nil, err
	}
	frozenKeys, err := s.redis.KeysCtx(ctx, frozenSeatsPattern(s.prefix, showTimeID, ticketCategoryID))
	if err != nil {
		return nil, err
	}
	for _, key := range frozenKeys {
		freezeToken := strings.TrimPrefix(key, fmt.Sprintf("%s:frozen:%s:%d:", s.prefix, seatLedgerScopeTag(showTimeID), ticketCategoryID))
		snapshot.FrozenSeats[freezeToken], err = s.listSeats(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	return snapshot, nil
}

func (s *SeatStockStore) listSeats(ctx context.Context, redisKey string) ([]Seat, error) {
	values, err := s.redis.ZrangeCtx(ctx, redisKey, 0, -1)
	if err != nil {
		if strings.Contains(err.Error(), "nil") {
			return []Seat{}, nil
		}
		return nil, err
	}

	resp := make([]Seat, 0, len(values))
	for _, value := range values {
		seat, err := decodeSeatMember(value)
		if err != nil {
			return nil, err
		}
		resp = append(resp, seat)
	}

	return resp, nil
}

func newSeatFromModel(seat *model.DSeat) Seat {
	return Seat{
		SeatID:           seat.Id,
		TicketCategoryID: seat.TicketCategoryId,
		RowCode:          seat.RowCode,
		ColCode:          seat.ColCode,
		Price:            int64(seat.Price),
	}
}

func encodeSeatMember(seat Seat) string {
	return fmt.Sprintf("%d|%d|%d|%d|%d", seat.SeatID, seat.TicketCategoryID, seat.RowCode, seat.ColCode, seat.Price)
}

func decodeSeatMember(member string) (Seat, error) {
	parts := strings.Split(member, "|")
	if len(parts) != 5 {
		return Seat{}, fmt.Errorf("invalid seat member: %s", member)
	}

	seatID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Seat{}, err
	}
	ticketCategoryID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return Seat{}, err
	}
	rowCode, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return Seat{}, err
	}
	colCode, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return Seat{}, err
	}
	price, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return Seat{}, err
	}

	return Seat{
		SeatID:           seatID,
		TicketCategoryID: ticketCategoryID,
		RowCode:          rowCode,
		ColCode:          colCode,
		Price:            price,
	}, nil
}

func seatSortScore(rowCode, colCode int64) int64 {
	return rowCode*1000000 + colCode
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
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("unsupported eval result type %T", value)
	}
}

func parseEvalStrings(value any) ([]string, error) {
	switch v := value.(type) {
	case []any:
		resp := make([]string, 0, len(v))
		for _, item := range v {
			str, err := parseEvalString(item)
			if err != nil {
				return nil, err
			}
			resp = append(resp, str)
		}
		return resp, nil
	default:
		return nil, fmt.Errorf("unsupported eval strings type %T", value)
	}
}

func parseEvalString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case int:
		return strconv.Itoa(v), nil
	default:
		return "", fmt.Errorf("unsupported eval string type %T", value)
	}
}
