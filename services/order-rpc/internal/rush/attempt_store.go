package rush

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"livepass/pkg/xerr"
	"livepass/pkg/xredis"

	goredis "github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	defaultAttemptInFlightTTL = 30 * time.Second
	defaultAttemptFinalTTL    = 30 * time.Minute
)

type AttemptStoreConfig struct {
	Prefix        string
	InFlightTTL   time.Duration
	FinalStateTTL time.Duration
}

type AdmitAttemptRequest struct {
	OrderNumber      int64
	UserID           int64
	ProgramID        int64
	ShowTimeID       int64
	TicketCategoryID int64
	ViewerIDs        []int64
	TicketCount      int64
	SaleWindowEndAt  time.Time
	ShowEndAt        time.Time
	Now              time.Time
}

type AdmitAttemptResult struct {
	OrderNumber int64
	Decision    int64
	RejectCode  int64
}

type AttemptStore struct {
	redis                *xredis.Client
	prefix               string
	inFlightTTLSeconds   int
	finalStateTTLSeconds int
}

func NewAttemptStore(redis *xredis.Client, cfg AttemptStoreConfig) *AttemptStore {
	if redis == nil {
		return nil
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = defaultAttemptPrefix
	}

	inFlightTTLSeconds := durationToSeconds(cfg.InFlightTTL, defaultAttemptInFlightTTL)
	finalStateTTLSeconds := durationToSeconds(cfg.FinalStateTTL, defaultAttemptFinalTTL)

	return &AttemptStore{
		redis:                redis,
		prefix:               prefix,
		inFlightTTLSeconds:   inFlightTTLSeconds,
		finalStateTTLSeconds: finalStateTTLSeconds,
	}
}

func (s *AttemptStore) SetQuotaAvailable(ctx context.Context, showTimeID, ticketCategoryID, available int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if showTimeID <= 0 || ticketCategoryID <= 0 || available < 0 {
		return xerr.ErrInvalidParam
	}

	return s.redis.HsetCtx(
		ctx,
		quotaAvailableKey(s.prefix, showTimeID),
		strconv.FormatInt(ticketCategoryID, 10),
		strconv.FormatInt(available, 10),
	)
}

func (s *AttemptStore) SetQuotaAvailableIfAbsent(ctx context.Context, showTimeID, ticketCategoryID, available int64) (bool, error) {
	if s == nil || s.redis == nil {
		return false, xerr.ErrInternal
	}
	if showTimeID <= 0 || ticketCategoryID <= 0 || available < 0 {
		return false, xerr.ErrInvalidParam
	}

	return s.redis.HsetnxCtx(
		ctx,
		quotaAvailableKey(s.prefix, showTimeID),
		strconv.FormatInt(ticketCategoryID, 10),
		strconv.FormatInt(available, 10),
	)
}

func (s *AttemptStore) GetQuotaAvailable(ctx context.Context, showTimeID, ticketCategoryID int64) (available int64, ok bool, err error) {
	if s == nil || s.redis == nil {
		return 0, false, xerr.ErrInternal
	}
	if showTimeID <= 0 || ticketCategoryID <= 0 {
		return 0, false, xerr.ErrInvalidParam
	}

	raw, err := s.redis.HgetCtx(
		ctx,
		quotaAvailableKey(s.prefix, showTimeID),
		strconv.FormatInt(ticketCategoryID, 10),
	)
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if raw == "" {
		return 0, false, nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false, err
	}

	return value, true, nil
}

func (s *AttemptStore) Admit(ctx context.Context, req AdmitAttemptRequest) (*AdmitAttemptResult, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrInternal
	}
	if req.ShowTimeID <= 0 {
		req.ShowTimeID = req.ProgramID
	}
	if req.OrderNumber <= 0 || req.UserID <= 0 || req.ProgramID <= 0 || req.ShowTimeID <= 0 || req.TicketCategoryID <= 0 {
		return nil, xerr.ErrInvalidParam
	}

	viewerIDs := normalizedInt64s(req.ViewerIDs)
	if len(viewerIDs) == 0 {
		return nil, xerr.ErrInvalidParam
	}
	if req.TicketCount <= 0 {
		req.TicketCount = int64(len(viewerIDs))
	}
	if req.TicketCount <= 0 || req.TicketCount != int64(len(viewerIDs)) {
		return nil, xerr.ErrInvalidParam
	}

	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	saleWindowEndAt := req.SaleWindowEndAt
	if saleWindowEndAt.IsZero() {
		saleWindowEndAt = now.Add(time.Duration(s.inFlightTTLSeconds) * time.Second)
	}
	showEndAt := req.ShowEndAt
	if showEndAt.IsZero() {
		showEndAt = saleWindowEndAt
	}
	attemptTTLSeconds := computeAttemptRecordTTLSeconds(now, saleWindowEndAt, s.inFlightTTLSeconds)
	projectionTTLSeconds := computeActiveProjectionTTLSeconds(now, showEndAt)

	keys := []string{
		attemptRecordKey(s.prefix, req.ShowTimeID, req.OrderNumber),
		userActiveKey(s.prefix, req.ShowTimeID),
		userInflightKey(s.prefix, req.ShowTimeID),
		viewerActiveKey(s.prefix, req.ShowTimeID),
		viewerInflightKey(s.prefix, req.ShowTimeID),
		quotaAvailableKey(s.prefix, req.ShowTimeID),
	}

	result, err := s.redis.EvalCtx(
		ctx,
		admitAttemptScript,
		keys,
		req.OrderNumber,
		req.UserID,
		req.ProgramID,
		req.ShowTimeID,
		req.TicketCategoryID,
		req.TicketCount,
		saleWindowEndAt.UnixMilli(),
		showEndAt.UnixMilli(),
		now.UnixMilli(),
		s.inFlightTTLSeconds,
		attemptTTLSeconds,
		projectionTTLSeconds,
		formatInt64CSV(viewerIDs),
	)
	if err != nil {
		return nil, err
	}

	values, err := parseEvalArray(result)
	if err != nil {
		return nil, err
	}
	if len(values) < 3 {
		return nil, fmt.Errorf("unexpected admit result length: %d", len(values))
	}

	decision, err := parseEvalInt64(values[0])
	if err != nil {
		return nil, err
	}
	orderNumber, err := parseEvalInt64AllowBlank(values[1])
	if err != nil {
		return nil, err
	}
	rejectCode, err := parseEvalInt64(values[2])
	if err != nil {
		return nil, err
	}

	return &AdmitAttemptResult{
		OrderNumber: orderNumber,
		Decision:    decision,
		RejectCode:  rejectCode,
	}, nil
}

func (s *AttemptStore) Get(ctx context.Context, orderNumber int64) (*AttemptRecord, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrInternal
	}
	if orderNumber <= 0 {
		return nil, xerr.ErrInvalidParam
	}

	attemptKey, err := s.resolveAttemptRecordKey(ctx, orderNumber)
	if err != nil {
		return nil, err
	}

	fields, err := s.redis.HgetallCtx(ctx, attemptKey)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, xerr.ErrOrderNotFound
	}

	record, err := mapAttemptRecord(fields)
	if err != nil {
		return nil, err
	}
	if record.OrderNumber == 0 {
		record.OrderNumber = orderNumber
	}

	return record, nil
}

func (s *AttemptStore) GetByShowTimeAndOrderNumber(ctx context.Context, showTimeID, orderNumber int64) (*AttemptRecord, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrInternal
	}
	if showTimeID <= 0 || orderNumber <= 0 {
		return nil, xerr.ErrInvalidParam
	}

	fields, err := s.redis.HgetallCtx(ctx, attemptRecordKey(s.prefix, showTimeID, orderNumber))
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, xerr.ErrOrderNotFound
	}

	record, err := mapAttemptRecord(fields)
	if err != nil {
		return nil, err
	}
	if record.OrderNumber == 0 {
		record.OrderNumber = orderNumber
	}
	if record.ShowTimeID == 0 {
		record.ShowTimeID = showTimeID
	}

	return record, nil
}

func (s *AttemptStore) ClearUserInflightByShowTime(ctx context.Context, showTimeID int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if showTimeID <= 0 {
		return xerr.ErrInvalidParam
	}

	_, err := s.redis.DelCtx(ctx, userInflightKey(s.prefix, showTimeID))
	return err
}

func (s *AttemptStore) ClearViewerInflightByShowTime(ctx context.Context, showTimeID int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if showTimeID <= 0 {
		return xerr.ErrInvalidParam
	}

	_, err := s.redis.DelCtx(ctx, viewerInflightKey(s.prefix, showTimeID))
	return err
}

func (s *AttemptStore) ClearQuotaByShowTime(ctx context.Context, showTimeID int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if showTimeID <= 0 {
		return xerr.ErrInvalidParam
	}

	_, err := s.redis.DelCtx(ctx, quotaAvailableKey(s.prefix, showTimeID))
	return err
}

func (s *AttemptStore) ReplaceUserActiveByShowTime(ctx context.Context, showTimeID int64, rows map[int64]int64, ttlSeconds int) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if showTimeID <= 0 || ttlSeconds < 0 {
		return xerr.ErrInvalidParam
	}

	key := userActiveKey(s.prefix, showTimeID)
	if _, err := s.redis.DelCtx(ctx, key); err != nil {
		return err
	}
	fields := make(map[string]string, len(rows))
	for userID, orderNumber := range rows {
		if userID <= 0 || orderNumber <= 0 {
			return xerr.ErrInvalidParam
		}
		fields[strconv.FormatInt(userID, 10)] = strconv.FormatInt(orderNumber, 10)
	}
	if len(fields) > 0 {
		if err := s.redis.HmsetCtx(ctx, key, fields); err != nil {
			return err
		}
	}
	if ttlSeconds > 0 {
		if err := s.redis.ExpireCtx(ctx, key, ttlSeconds); err != nil {
			return err
		}
	}

	return nil
}

func (s *AttemptStore) ReplaceViewerActiveByShowTime(ctx context.Context, showTimeID int64, rows map[int64]int64, ttlSeconds int) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if showTimeID <= 0 || ttlSeconds < 0 {
		return xerr.ErrInvalidParam
	}

	key := viewerActiveKey(s.prefix, showTimeID)
	if _, err := s.redis.DelCtx(ctx, key); err != nil {
		return err
	}
	fields := make(map[string]string, len(rows))
	for viewerID, orderNumber := range rows {
		if viewerID <= 0 || orderNumber <= 0 {
			return xerr.ErrInvalidParam
		}
		fields[strconv.FormatInt(viewerID, 10)] = strconv.FormatInt(orderNumber, 10)
	}
	if len(fields) > 0 {
		if err := s.redis.HmsetCtx(ctx, key, fields); err != nil {
			return err
		}
	}
	if ttlSeconds > 0 {
		if err := s.redis.ExpireCtx(ctx, key, ttlSeconds); err != nil {
			return err
		}
	}

	return nil
}

func (s *AttemptStore) PrepareAttemptForConsume(ctx context.Context, showTimeID, orderNumber int64, now time.Time) (*AttemptRecord, bool, error) {
	if s == nil || s.redis == nil {
		return nil, false, xerr.ErrInternal
	}
	if showTimeID <= 0 || orderNumber <= 0 {
		return nil, false, xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	result, err := s.redis.EvalCtx(
		ctx,
		prepareAttemptForConsumeScript,
		[]string{attemptRecordKey(s.prefix, showTimeID, orderNumber)},
		now.UnixMilli(),
		s.inFlightTTLSeconds,
	)
	if err != nil {
		return nil, false, err
	}

	values, err := parseEvalArray(result)
	if err != nil {
		return nil, false, err
	}
	if len(values) == 1 {
		statusCode, parseErr := parseEvalInt64(values[0])
		if parseErr != nil {
			return nil, false, parseErr
		}
		if statusCode <= 0 {
			return nil, false, xerr.ErrOrderNotFound
		}
	}
	if len(values) < 17 {
		return nil, false, fmt.Errorf("unexpected prepare attempt result length: %d", len(values))
	}

	shouldProcessCode, err := parseEvalInt64(values[0])
	if err != nil {
		return nil, false, err
	}
	record, err := mapPreparedAttemptRecord(values)
	if err != nil {
		return nil, false, err
	}
	if record.OrderNumber == 0 {
		record.OrderNumber = orderNumber
	}
	if record.ShowTimeID == 0 {
		record.ShowTimeID = showTimeID
	}

	return record, shouldProcessCode == 1, nil
}

func (s *AttemptStore) FailBeforeProcessing(ctx context.Context, record *AttemptRecord, reason string, now time.Time) (AttemptTransitionOutcome, error) {
	if s == nil || s.redis == nil {
		return AttemptStateMissing, xerr.ErrInternal
	}
	if record == nil || record.OrderNumber <= 0 || record.UserID <= 0 || record.ProgramID <= 0 || record.ShowTimeID <= 0 || record.TicketCategoryID <= 0 {
		return AttemptStateMissing, xerr.ErrInvalidParam
	}
	if reason == "" {
		return AttemptStateMissing, xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	keys := []string{
		attemptRecordKey(s.prefix, record.ShowTimeID, record.OrderNumber),
		userInflightKey(s.prefix, record.ShowTimeID),
		viewerInflightKey(s.prefix, record.ShowTimeID),
		quotaAvailableKey(s.prefix, record.ShowTimeID),
	}

	result, err := s.redis.EvalCtx(
		ctx,
		failBeforeProcessingScript,
		keys,
		reason,
		record.UserID,
		formatInt64CSV(normalizedInt64s(record.ViewerIDs)),
		record.TicketCategoryID,
		record.TicketCount,
		now.UnixMilli(),
		s.finalStateTTLSeconds,
		computeActiveProjectionTTLSeconds(now, record.ShowEndAt),
		s.inFlightTTLSeconds,
	)
	if err != nil {
		return AttemptStateMissing, err
	}

	return parseAttemptTransitionOutcome(result)
}

func (s *AttemptStore) RefreshProcessingLease(ctx context.Context, showTimeID, orderNumber int64, now time.Time) (bool, error) {
	if s == nil || s.redis == nil {
		return false, xerr.ErrInternal
	}
	if showTimeID <= 0 || orderNumber <= 0 {
		return false, xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	result, err := s.redis.EvalCtx(
		ctx,
		refreshProcessingLeaseScript,
		[]string{attemptRecordKey(s.prefix, showTimeID, orderNumber)},
		now.UnixMilli(),
		s.inFlightTTLSeconds,
	)
	if err != nil {
		return false, err
	}

	statusCode, err := parseEvalInt64(result)
	if err != nil {
		return false, err
	}
	return statusCode == 1, nil
}

func (s *AttemptStore) FinalizeSuccess(ctx context.Context, record *AttemptRecord, now time.Time) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if record == nil || record.OrderNumber <= 0 || record.UserID <= 0 || record.ProgramID <= 0 || record.ShowTimeID <= 0 {
		return xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	activeTTLSeconds := computeActiveProjectionTTLSeconds(now, record.ShowEndAt)
	viewerIDs := normalizedInt64s(record.ViewerIDs)
	keys := []string{
		attemptRecordKey(s.prefix, record.ShowTimeID, record.OrderNumber),
		userActiveKey(s.prefix, record.ShowTimeID),
		userInflightKey(s.prefix, record.ShowTimeID),
		viewerActiveKey(s.prefix, record.ShowTimeID),
		viewerInflightKey(s.prefix, record.ShowTimeID),
	}

	result, err := s.redis.EvalCtx(
		ctx,
		finalizeSuccessScript,
		keys,
		record.UserID,
		formatInt64CSV(viewerIDs),
		now.UnixMilli(),
		activeTTLSeconds,
		s.finalStateTTLSeconds,
		record.OrderNumber,
	)
	if err != nil {
		return err
	}

	outcome, err := parseAttemptTransitionOutcome(result)
	if err != nil {
		return err
	}
	if outcome == AttemptStateMissing {
		return xerr.ErrOrderNotFound
	}

	return nil
}

func (s *AttemptStore) FinalizeFailure(ctx context.Context, record *AttemptRecord, reason string, now time.Time) (AttemptTransitionOutcome, error) {
	if s == nil || s.redis == nil {
		return AttemptStateMissing, xerr.ErrInternal
	}
	if record == nil || record.OrderNumber <= 0 || record.UserID <= 0 || record.ProgramID <= 0 || record.ShowTimeID <= 0 || record.TicketCategoryID <= 0 {
		return AttemptStateMissing, xerr.ErrInvalidParam
	}
	if reason == "" {
		return AttemptStateMissing, xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	viewerIDs := normalizedInt64s(record.ViewerIDs)
	keys := []string{
		attemptRecordKey(s.prefix, record.ShowTimeID, record.OrderNumber),
		userActiveKey(s.prefix, record.ShowTimeID),
		userInflightKey(s.prefix, record.ShowTimeID),
		viewerActiveKey(s.prefix, record.ShowTimeID),
		viewerInflightKey(s.prefix, record.ShowTimeID),
		quotaAvailableKey(s.prefix, record.ShowTimeID),
	}

	result, err := s.redis.EvalCtx(
		ctx,
		finalizeFailureScript,
		keys,
		reason,
		record.UserID,
		formatInt64CSV(viewerIDs),
		record.TicketCategoryID,
		record.TicketCount,
		now.UnixMilli(),
		s.finalStateTTLSeconds,
		computeActiveProjectionTTLSeconds(now, record.ShowEndAt),
		s.inFlightTTLSeconds,
	)
	if err != nil {
		return AttemptStateMissing, err
	}

	return parseAttemptTransitionOutcome(result)
}

func (s *AttemptStore) FinalizeClosedOrder(ctx context.Context, record *AttemptRecord, now time.Time) (AttemptTransitionOutcome, error) {
	if s == nil || s.redis == nil {
		return AttemptStateMissing, xerr.ErrInternal
	}
	if record == nil || record.OrderNumber <= 0 || record.UserID <= 0 || record.ProgramID <= 0 || record.ShowTimeID <= 0 || record.TicketCategoryID <= 0 {
		return AttemptStateMissing, xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	viewerIDs := normalizedInt64s(record.ViewerIDs)
	keys := []string{
		attemptRecordKey(s.prefix, record.ShowTimeID, record.OrderNumber),
		userActiveKey(s.prefix, record.ShowTimeID),
		userInflightKey(s.prefix, record.ShowTimeID),
		viewerActiveKey(s.prefix, record.ShowTimeID),
		viewerInflightKey(s.prefix, record.ShowTimeID),
		quotaAvailableKey(s.prefix, record.ShowTimeID),
	}

	result, err := s.redis.EvalCtx(
		ctx,
		finalizeClosedOrderScript,
		keys,
		record.UserID,
		formatInt64CSV(viewerIDs),
		now.UnixMilli(),
		record.TicketCategoryID,
		record.TicketCount,
		s.finalStateTTLSeconds,
		computeActiveProjectionTTLSeconds(now, record.ShowEndAt),
		s.inFlightTTLSeconds,
	)
	if err != nil {
		return AttemptStateMissing, err
	}

	return parseAttemptTransitionOutcome(result)
}

func (s *AttemptStore) ScanOrderNumbers(ctx context.Context, limit int64) ([]int64, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrInternal
	}
	if limit <= 0 {
		limit = 100
	}

	pattern := fmt.Sprintf("%s:attempt:*", s.prefix)
	var (
		cursor uint64
		keys   []string
	)
	for {
		batch, nextCursor, err := s.redis.ScanCtx(ctx, cursor, pattern, limit)
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		if int64(len(keys)) >= limit || nextCursor == 0 {
			break
		}
		cursor = nextCursor
	}
	if int64(len(keys)) > limit {
		keys = keys[:limit]
	}

	orderNumbers := make([]int64, 0, len(keys))
	seen := make(map[int64]struct{}, len(keys))
	for _, key := range keys {
		idx := strings.LastIndexByte(key, ':')
		if idx < 0 || idx == len(key)-1 {
			continue
		}
		orderNumber, err := strconv.ParseInt(key[idx+1:], 10, 64)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[orderNumber]; ok {
			continue
		}
		seen[orderNumber] = struct{}{}
		orderNumbers = append(orderNumbers, orderNumber)
	}

	sort.Slice(orderNumbers, func(i, j int) bool {
		return orderNumbers[i] < orderNumbers[j]
	})

	return orderNumbers, nil
}

func mapAttemptRecord(fields map[string]string) (*AttemptRecord, error) {
	record := &AttemptRecord{
		State:      fields[attemptFieldState],
		ReasonCode: fields[attemptFieldReasonCode],
	}

	var err error
	record.OrderNumber, err = parseFieldInt64(fields, attemptFieldOrderNumber)
	if err != nil {
		return nil, err
	}
	record.UserID, err = parseFieldInt64(fields, attemptFieldUserID)
	if err != nil {
		return nil, err
	}
	record.ProgramID, err = parseFieldInt64(fields, attemptFieldProgramID)
	if err != nil {
		return nil, err
	}
	record.ShowTimeID, err = parseFieldInt64(fields, attemptFieldShowTimeID)
	if err != nil {
		return nil, err
	}
	if record.ShowTimeID <= 0 {
		record.ShowTimeID = record.ProgramID
	}
	record.TicketCategoryID, err = parseFieldInt64(fields, attemptFieldTicketCategoryID)
	if err != nil {
		return nil, err
	}
	record.TicketCount, err = parseFieldInt64(fields, attemptFieldTicketCount)
	if err != nil {
		return nil, err
	}
	record.ViewerIDs, err = parseInt64CSV(fields[attemptFieldViewerIDs])
	if err != nil {
		return nil, err
	}
	record.AcceptedAt, err = parseFieldTime(fields, attemptFieldAcceptedAt)
	if err != nil {
		return nil, err
	}
	record.FinishedAt, err = parseFieldTime(fields, attemptFieldFinishedAt)
	if err != nil {
		return nil, err
	}
	record.SaleWindowEndAt, err = parseFieldTime(fields, attemptFieldSaleWindowEndAt)
	if err != nil {
		return nil, err
	}
	record.ShowEndAt, err = parseFieldTime(fields, attemptFieldShowEndAt)
	if err != nil {
		return nil, err
	}
	record.ProcessingStartedAt, err = parseFieldTime(fields, attemptFieldProcessingStartAt)
	if err != nil {
		return nil, err
	}
	record.CreatedAt, err = parseFieldTime(fields, attemptFieldCreatedAt)
	if err != nil {
		return nil, err
	}
	record.LastTransitionAt, err = parseFieldTime(fields, attemptFieldTransitionAt)
	if err != nil {
		return nil, err
	}

	return record, nil
}

func mapPreparedAttemptRecord(values []any) (*AttemptRecord, error) {
	state, err := parseEvalString(values[1])
	if err != nil {
		return nil, err
	}
	orderNumber, err := parseEvalInt64AllowBlank(values[2])
	if err != nil {
		return nil, err
	}
	userID, err := parseEvalInt64AllowBlank(values[3])
	if err != nil {
		return nil, err
	}
	programID, err := parseEvalInt64AllowBlank(values[4])
	if err != nil {
		return nil, err
	}
	showTimeID, err := parseEvalInt64AllowBlank(values[5])
	if err != nil {
		return nil, err
	}
	ticketCategoryID, err := parseEvalInt64AllowBlank(values[6])
	if err != nil {
		return nil, err
	}
	viewerIDsRaw, err := parseEvalString(values[7])
	if err != nil {
		return nil, err
	}
	viewerIDs, err := parseInt64CSV(viewerIDsRaw)
	if err != nil {
		return nil, err
	}
	ticketCount, err := parseEvalInt64AllowBlank(values[8])
	if err != nil {
		return nil, err
	}
	saleWindowEndAt, err := parsePreparedTime(values[9])
	if err != nil {
		return nil, err
	}
	showEndAt, err := parsePreparedTime(values[10])
	if err != nil {
		return nil, err
	}
	reasonCode, err := parseEvalString(values[11])
	if err != nil {
		return nil, err
	}
	acceptedAt, err := parsePreparedTime(values[12])
	if err != nil {
		return nil, err
	}
	finishedAt, err := parsePreparedTime(values[13])
	if err != nil {
		return nil, err
	}
	processingStartedAt, err := parsePreparedTime(values[14])
	if err != nil {
		return nil, err
	}
	createdAt, err := parsePreparedTime(values[15])
	if err != nil {
		return nil, err
	}
	lastTransitionAt, err := parsePreparedTime(values[16])
	if err != nil {
		return nil, err
	}

	return &AttemptRecord{
		OrderNumber:         orderNumber,
		UserID:              userID,
		ProgramID:           programID,
		ShowTimeID:          showTimeID,
		TicketCategoryID:    ticketCategoryID,
		ViewerIDs:           viewerIDs,
		TicketCount:         ticketCount,
		SaleWindowEndAt:     saleWindowEndAt,
		ShowEndAt:           showEndAt,
		State:               state,
		ReasonCode:          reasonCode,
		AcceptedAt:          acceptedAt,
		FinishedAt:          finishedAt,
		ProcessingStartedAt: processingStartedAt,
		CreatedAt:           createdAt,
		LastTransitionAt:    lastTransitionAt,
	}, nil
}

func parsePreparedTime(value any) (time.Time, error) {
	raw, err := parseEvalString(value)
	if err != nil {
		return time.Time{}, err
	}
	if raw == "" {
		return time.Time{}, nil
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	if ms <= 0 {
		return time.Time{}, nil
	}
	return time.UnixMilli(ms), nil
}

func (s *AttemptStore) resolveAttemptRecordKey(ctx context.Context, orderNumber int64) (string, error) {
	pattern := fmt.Sprintf("%s:attempt:*:%d", s.prefix, orderNumber)
	var cursor uint64
	for {
		batch, nextCursor, err := s.redis.ScanCtx(ctx, cursor, pattern, 16)
		if err != nil {
			return "", err
		}
		if len(batch) > 0 {
			sort.Strings(batch)
			return batch[0], nil
		}
		if nextCursor == 0 {
			break
		}
		cursor = nextCursor
	}

	return "", xerr.ErrOrderNotFound
}

func (s *AttemptStore) deleteKeysByPattern(ctx context.Context, pattern string) error {
	if pattern == "" {
		return xerr.ErrInvalidParam
	}

	var cursor uint64
	for {
		keys, nextCursor, err := s.redis.ScanCtx(ctx, cursor, pattern, 128)
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if _, err := s.redis.DelCtx(ctx, keys...); err != nil {
				return err
			}
		}
		if nextCursor == 0 {
			return nil
		}
		cursor = nextCursor
	}
}

func computeAttemptRecordTTLSeconds(now, saleWindowEndAt time.Time, fallback int) int {
	if fallback <= 0 {
		fallback = int(defaultAttemptFinalTTL.Seconds())
	}
	if now.IsZero() || saleWindowEndAt.IsZero() {
		return fallback
	}
	return maxInt(fallback, int(saleWindowEndAt.Sub(now).Seconds())+2*60*60)
}

func computeActiveProjectionTTLSeconds(now, showEndAt time.Time) int {
	const retention = 7 * 24 * 60 * 60
	if now.IsZero() || showEndAt.IsZero() {
		return retention
	}
	return maxInt(retention, int(showEndAt.Sub(now).Seconds())+retention)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parseFieldInt64(fields map[string]string, key string) (int64, error) {
	raw, ok := fields[key]
	if !ok || raw == "" {
		return 0, nil
	}

	return strconv.ParseInt(raw, 10, 64)
}

func parseFieldTime(fields map[string]string, key string) (time.Time, error) {
	raw, ok := fields[key]
	if !ok || raw == "" {
		return time.Time{}, nil
	}

	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	if ms <= 0 {
		return time.Time{}, nil
	}

	return time.UnixMilli(ms), nil
}

func parseEvalInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("unsupported eval result type %T", value)
	}
}

func parseEvalInt64AllowBlank(value any) (int64, error) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return 0, nil
		}
	case []byte:
		if len(v) == 0 {
			return 0, nil
		}
	}

	return parseEvalInt64(value)
}

func parseEvalArray(value any) ([]any, error) {
	switch v := value.(type) {
	case []any:
		return v, nil
	default:
		return nil, fmt.Errorf("unexpected eval array type %T", value)
	}
}

func parseEvalString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("unsupported eval string type %T", value)
	}
}

func parseAttemptTransitionOutcome(value any) (AttemptTransitionOutcome, error) {
	raw, err := parseEvalString(value)
	if err != nil {
		return AttemptStateMissing, err
	}
	outcome := AttemptTransitionOutcome(raw)
	switch outcome {
	case AttemptTransitioned, AttemptAlreadyFailed, AttemptAlreadySucceeded, AttemptLostOwnership, AttemptStateMissing:
		return outcome, nil
	default:
		return AttemptStateMissing, fmt.Errorf("unknown attempt transition outcome: %s", raw)
	}
}

func durationToSeconds(value, defaultValue time.Duration) int {
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

func normalizedInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}

	unique := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := unique[value]; ok {
			continue
		}
		unique[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	return result
}
