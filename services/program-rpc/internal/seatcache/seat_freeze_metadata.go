package seatcache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"damai-go/pkg/xerr"
)

const (
	SeatFreezeStatusFrozen    int64 = 1
	SeatFreezeStatusReleased  int64 = 2
	SeatFreezeStatusExpired   int64 = 3
	SeatFreezeStatusConfirmed int64 = 4
)

type SeatFreezeMetadata struct {
	FreezeToken      string `json:"freezeToken"`
	RequestNo        string `json:"requestNo"`
	ProgramID        int64  `json:"programId"`
	TicketCategoryID int64  `json:"ticketCategoryId"`
	SeatCount        int64  `json:"seatCount"`
	FreezeStatus     int64  `json:"freezeStatus"`
	ExpireAt         int64  `json:"expireAt"`
	ReleaseReason    string `json:"releaseReason,omitempty"`
	ReleaseAt        int64  `json:"releaseAt,omitempty"`
	UpdatedAt        int64  `json:"updatedAt"`
}

func (m *SeatFreezeMetadata) ExpireTime() time.Time {
	if m == nil || m.ExpireAt <= 0 {
		return time.Time{}
	}

	return time.Unix(m.ExpireAt, 0)
}

func (s *SeatStockStore) SaveFreezeMetadata(ctx context.Context, meta *SeatFreezeMetadata) error {
	if s == nil || s.redis == nil {
		return xerr.ErrProgramSeatLedgerNotReady
	}
	if meta == nil || meta.FreezeToken == "" || meta.RequestNo == "" || meta.ProgramID <= 0 || meta.TicketCategoryID <= 0 {
		return xerr.ErrInvalidParam
	}

	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	metaKey := freezeMetaKey(s.prefix, meta.FreezeToken)
	requestKey := freezeRequestKey(s.prefix, meta.RequestNo)
	indexKey := freezeIndexKey(s.prefix, meta.ProgramID, meta.TicketCategoryID)
	ttlSeconds := s.freezeMetadataTTLSeconds()

	if err := s.redis.SetexCtx(ctx, metaKey, string(payload), ttlSeconds); err != nil {
		return err
	}
	if err := s.redis.SetexCtx(ctx, requestKey, meta.FreezeToken, ttlSeconds); err != nil {
		_, _ = s.redis.DelCtx(ctx, metaKey)
		return err
	}
	if _, err := s.redis.ZaddCtx(ctx, indexKey, meta.ExpireAt, meta.FreezeToken); err != nil {
		_, _ = s.redis.DelCtx(ctx, metaKey, requestKey)
		return err
	}
	if ttlSeconds > 0 {
		if err := s.redis.ExpireCtx(ctx, indexKey, ttlSeconds); err != nil {
			_, _ = s.redis.DelCtx(ctx, metaKey, requestKey)
			_, _ = s.redis.ZremCtx(ctx, indexKey, meta.FreezeToken)
			return err
		}
	}

	return nil
}

func (s *SeatStockStore) GetFreezeMetadataByToken(ctx context.Context, freezeToken string) (*SeatFreezeMetadata, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrProgramSeatLedgerNotReady
	}
	if freezeToken == "" {
		return nil, nil
	}

	payload, err := s.redis.GetCtx(ctx, freezeMetaKey(s.prefix, freezeToken))
	if err != nil {
		return nil, err
	}
	if payload == "" {
		return nil, nil
	}

	var meta SeatFreezeMetadata
	if err := json.Unmarshal([]byte(payload), &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

func (s *SeatStockStore) GetFreezeMetadataByRequestNo(ctx context.Context, requestNo string) (*SeatFreezeMetadata, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrProgramSeatLedgerNotReady
	}
	if requestNo == "" {
		return nil, nil
	}

	freezeToken, err := s.redis.GetCtx(ctx, freezeRequestKey(s.prefix, requestNo))
	if err != nil {
		return nil, err
	}
	if freezeToken == "" {
		return nil, nil
	}

	return s.GetFreezeMetadataByToken(ctx, freezeToken)
}

func (s *SeatStockStore) MarkFreezeReleased(ctx context.Context, freezeToken, releaseReason string, now time.Time) (*SeatFreezeMetadata, error) {
	meta, err := s.GetFreezeMetadataByToken(ctx, freezeToken)
	if err != nil || meta == nil {
		return meta, err
	}

	meta.FreezeStatus = SeatFreezeStatusReleased
	meta.ReleaseReason = releaseReason
	meta.ReleaseAt = now.Unix()
	meta.UpdatedAt = now.Unix()
	if err := s.persistFreezeMetadata(ctx, meta); err != nil {
		return nil, err
	}
	if err := s.removeFreezeFromIndex(ctx, meta); err != nil {
		return nil, err
	}

	return meta, nil
}

func (s *SeatStockStore) MarkFreezeExpired(ctx context.Context, freezeToken string, now time.Time) (*SeatFreezeMetadata, error) {
	meta, err := s.GetFreezeMetadataByToken(ctx, freezeToken)
	if err != nil || meta == nil {
		return meta, err
	}

	meta.FreezeStatus = SeatFreezeStatusExpired
	meta.ReleaseAt = now.Unix()
	meta.UpdatedAt = now.Unix()
	if err := s.persistFreezeMetadata(ctx, meta); err != nil {
		return nil, err
	}
	if err := s.removeFreezeFromIndex(ctx, meta); err != nil {
		return nil, err
	}

	return meta, nil
}

func (s *SeatStockStore) MarkFreezeConfirmed(ctx context.Context, freezeToken string, now time.Time) (*SeatFreezeMetadata, error) {
	meta, err := s.GetFreezeMetadataByToken(ctx, freezeToken)
	if err != nil || meta == nil {
		return meta, err
	}

	meta.FreezeStatus = SeatFreezeStatusConfirmed
	meta.UpdatedAt = now.Unix()
	if err := s.persistFreezeMetadata(ctx, meta); err != nil {
		return nil, err
	}
	if err := s.removeFreezeFromIndex(ctx, meta); err != nil {
		return nil, err
	}

	return meta, nil
}

func (s *SeatStockStore) ListExpiredFreezeTokens(ctx context.Context, programID, ticketCategoryID int64, now time.Time) ([]string, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrProgramSeatLedgerNotReady
	}

	pairs, err := s.redis.ZrangebyscoreWithScoresCtx(ctx, freezeIndexKey(s.prefix, programID, ticketCategoryID), 0, now.Unix())
	if err != nil {
		return nil, err
	}

	resp := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		if pair.Key == "" {
			continue
		}
		resp = append(resp, pair.Key)
	}

	return resp, nil
}

func (s *SeatStockStore) FrozenSeats(ctx context.Context, programID, ticketCategoryID int64, freezeToken string) ([]Seat, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrProgramSeatLedgerNotReady
	}

	return s.listSeats(ctx, frozenSeatsKey(s.prefix, programID, ticketCategoryID, freezeToken))
}

func (s *SeatStockStore) freezeMetadataTTLSeconds() int {
	if s == nil {
		return 0
	}
	if s.seatTTLSeconds > 0 {
		return s.seatTTLSeconds
	}
	if s.stockTTLSeconds > 0 {
		return s.stockTTLSeconds
	}

	return int(defaultSeatLedgerSeatTTL / time.Second)
}

func (s *SeatStockStore) persistFreezeMetadata(ctx context.Context, meta *SeatFreezeMetadata) error {
	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	ttlSeconds := s.freezeMetadataTTLSeconds()
	if err := s.redis.SetexCtx(ctx, freezeMetaKey(s.prefix, meta.FreezeToken), string(payload), ttlSeconds); err != nil {
		return err
	}
	if err := s.redis.SetexCtx(ctx, freezeRequestKey(s.prefix, meta.RequestNo), meta.FreezeToken, ttlSeconds); err != nil {
		return err
	}

	return nil
}

func (s *SeatStockStore) removeFreezeFromIndex(ctx context.Context, meta *SeatFreezeMetadata) error {
	if meta == nil {
		return nil
	}

	_, err := s.redis.ZremCtx(ctx, freezeIndexKey(s.prefix, meta.ProgramID, meta.TicketCategoryID), meta.FreezeToken)
	return err
}

func freezeMetaKey(prefix, freezeToken string) string {
	return fmt.Sprintf("%s:freeze:meta:%s", prefix, freezeToken)
}

func freezeRequestKey(prefix, requestNo string) string {
	return fmt.Sprintf("%s:freeze:req:%s", prefix, requestNo)
}

func freezeIndexKey(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:freeze:index:%d:%d", prefix, programID, ticketCategoryID)
}
