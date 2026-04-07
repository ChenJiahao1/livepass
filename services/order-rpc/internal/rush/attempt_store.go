package rush

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xredis"
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
	Generation       string
	SaleWindowEndAt  time.Time
	TokenFingerprint string
	CommitCutoffAt   time.Time
	UserDeadlineAt   time.Time
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
	attemptTTLSeconds    int
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
	attemptTTLSeconds := inFlightTTLSeconds
	if finalStateTTLSeconds > attemptTTLSeconds {
		attemptTTLSeconds = finalStateTTLSeconds
	}

	return &AttemptStore{
		redis:                redis,
		prefix:               prefix,
		inFlightTTLSeconds:   inFlightTTLSeconds,
		finalStateTTLSeconds: finalStateTTLSeconds,
		attemptTTLSeconds:    attemptTTLSeconds,
	}
}

func (s *AttemptStore) SetQuotaAvailable(ctx context.Context, showTimeID, ticketCategoryID, available int64) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if showTimeID <= 0 || ticketCategoryID <= 0 || available < 0 {
		return xerr.ErrInvalidParam
	}

	return s.redis.SetCtx(ctx, quotaAvailableKey(s.prefix, showTimeID, BuildRushGeneration(showTimeID), ticketCategoryID), strconv.FormatInt(available, 10))
}

func (s *AttemptStore) GetQuotaAvailable(ctx context.Context, showTimeID, ticketCategoryID int64) (available int64, ok bool, err error) {
	if s == nil || s.redis == nil {
		return 0, false, xerr.ErrInternal
	}
	if showTimeID <= 0 || ticketCategoryID <= 0 {
		return 0, false, xerr.ErrInvalidParam
	}

	raw, err := s.redis.GetCtx(ctx, quotaAvailableKey(s.prefix, showTimeID, BuildRushGeneration(showTimeID), ticketCategoryID))
	if err != nil {
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

	tokenFingerprint := req.TokenFingerprint
	generation := normalizeRushGeneration(req.ShowTimeID, req.Generation)
	if tokenFingerprint == "" {
		tokenFingerprint = BuildTokenFingerprint(
			req.UserID,
			req.ShowTimeID,
			req.TicketCategoryID,
			viewerIDs,
			"",
			"",
			generation,
		)
	}

	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	commitCutoffAt := req.CommitCutoffAt
	if commitCutoffAt.IsZero() {
		commitCutoffAt = now
	}
	userDeadlineAt := req.UserDeadlineAt
	if userDeadlineAt.IsZero() {
		userDeadlineAt = now.Add(time.Duration(s.inFlightTTLSeconds) * time.Second)
	}
	saleWindowEndAt := req.SaleWindowEndAt
	if saleWindowEndAt.IsZero() {
		saleWindowEndAt = userDeadlineAt
	}
	showEndAt := req.ShowEndAt
	if showEndAt.IsZero() {
		showEndAt = saleWindowEndAt
	}
	attemptTTLSeconds := computeAttemptRecordTTLSeconds(now, saleWindowEndAt, s.attemptTTLSeconds)

	keys := []string{
		attemptRecordKey(s.prefix, req.ShowTimeID, generation, req.OrderNumber),
		userActiveKey(s.prefix, req.ShowTimeID, generation, req.UserID),
		userInflightKey(s.prefix, req.ShowTimeID, generation, req.UserID),
		quotaAvailableKey(s.prefix, req.ShowTimeID, generation, req.TicketCategoryID),
		orderProgressIndexKey(s.prefix, req.ShowTimeID, generation),
		userFingerprintIndexKey(s.prefix, req.ShowTimeID, generation, req.UserID),
	}
	for _, viewerID := range viewerIDs {
		keys = append(keys, viewerActiveKey(s.prefix, req.ShowTimeID, generation, viewerID))
	}
	for _, viewerID := range viewerIDs {
		keys = append(keys, viewerInflightKey(s.prefix, req.ShowTimeID, generation, viewerID))
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
		generation,
		tokenFingerprint,
		saleWindowEndAt.UnixMilli(),
		commitCutoffAt.UnixMilli(),
		userDeadlineAt.UnixMilli(),
		showEndAt.UnixMilli(),
		now.UnixMilli(),
		s.inFlightTTLSeconds,
		attemptTTLSeconds,
		formatInt64CSV(viewerIDs),
		len(viewerIDs),
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

func (s *AttemptStore) MarkQueued(ctx context.Context, orderNumber int64, now time.Time) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if orderNumber <= 0 {
		return xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	attemptKey, err := s.resolveAttemptRecordKey(ctx, orderNumber)
	if err != nil {
		return err
	}

	result, err := s.redis.EvalCtx(
		ctx,
		markAttemptQueuedScript,
		[]string{attemptKey},
		now.UnixMilli(),
	)
	if err != nil {
		return err
	}

	statusCode, err := parseEvalInt64(result)
	if err != nil {
		return err
	}
	if statusCode < 0 {
		return xerr.ErrOrderNotFound
	}

	return nil
}

func (s *AttemptStore) MarkVerifying(ctx context.Context, orderNumber int64, now, nextProbeAt time.Time) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if orderNumber <= 0 {
		return xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}
	if nextProbeAt.IsZero() {
		nextProbeAt = now
	}

	attemptKey, err := s.resolveAttemptRecordKey(ctx, orderNumber)
	if err != nil {
		return err
	}

	result, err := s.redis.EvalCtx(
		ctx,
		markAttemptVerifyingScript,
		[]string{attemptKey},
		now.UnixMilli(),
		nextProbeAt.UnixMilli(),
	)
	if err != nil {
		return err
	}

	statusCode, err := parseEvalInt64(result)
	if err != nil {
		return err
	}
	if statusCode < 0 {
		return xerr.ErrOrderNotFound
	}

	return nil
}

func (s *AttemptStore) ClaimProcessing(ctx context.Context, orderNumber int64, now time.Time) (claimed bool, epoch int64, err error) {
	if s == nil || s.redis == nil {
		return false, 0, xerr.ErrInternal
	}
	if orderNumber <= 0 {
		return false, 0, xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	attemptKey, err := s.resolveAttemptRecordKey(ctx, orderNumber)
	if err != nil {
		return false, 0, err
	}

	result, err := s.redis.EvalCtx(
		ctx,
		claimProcessingScript,
		[]string{attemptKey},
		now.UnixMilli(),
	)
	if err != nil {
		return false, 0, err
	}

	values, err := parseEvalArray(result)
	if err != nil {
		return false, 0, err
	}
	if len(values) < 2 {
		return false, 0, fmt.Errorf("unexpected claim processing result length: %d", len(values))
	}

	statusCode, err := parseEvalInt64(values[0])
	if err != nil {
		return false, 0, err
	}
	epoch, err = parseEvalInt64(values[1])
	if err != nil {
		return false, 0, err
	}
	if statusCode < 0 {
		return false, 0, xerr.ErrOrderNotFound
	}

	return statusCode == 1, epoch, nil
}

func (s *AttemptStore) CommitProjection(ctx context.Context, record *AttemptRecord, seatIDs []int64, now time.Time) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if record == nil || record.OrderNumber <= 0 || record.UserID <= 0 || record.ProgramID <= 0 || record.ShowTimeID <= 0 {
		return xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}
	generation := normalizeRushGeneration(record.ShowTimeID, record.Generation)
	activeTTLSeconds := computeActiveProjectionTTLSeconds(now, record.ShowEndAt)

	keys := []string{
		attemptRecordKey(s.prefix, record.ShowTimeID, generation, record.OrderNumber),
		userActiveKey(s.prefix, record.ShowTimeID, generation, record.UserID),
		userInflightKey(s.prefix, record.ShowTimeID, generation, record.UserID),
		orderProgressIndexKey(s.prefix, record.ShowTimeID, generation),
		seatOccupiedKey(s.prefix, record.ShowTimeID, generation, record.OrderNumber),
	}
	for _, viewerID := range normalizedInt64s(record.ViewerIDs) {
		keys = append(keys, viewerActiveKey(s.prefix, record.ShowTimeID, generation, viewerID))
	}
	for _, viewerID := range normalizedInt64s(record.ViewerIDs) {
		keys = append(keys, viewerInflightKey(s.prefix, record.ShowTimeID, generation, viewerID))
	}

	result, err := s.redis.EvalCtx(
		ctx,
		commitAttemptProjectionScript,
		keys,
		now.UnixMilli(),
		activeTTLSeconds,
		s.finalStateTTLSeconds,
		formatInt64CSV(seatIDs),
		len(record.ViewerIDs),
		record.OrderNumber,
	)
	if err != nil {
		return err
	}

	statusCode, err := parseEvalInt64(result)
	if err != nil {
		return err
	}
	if statusCode < 0 {
		return xerr.ErrOrderNotFound
	}

	return nil
}

func (s *AttemptStore) Release(ctx context.Context, record *AttemptRecord, reason string, now time.Time) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if record == nil || record.OrderNumber <= 0 || record.UserID <= 0 || record.ProgramID <= 0 || record.ShowTimeID <= 0 || record.TicketCategoryID <= 0 {
		return xerr.ErrInvalidParam
	}
	if reason == "" {
		return xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	generation := normalizeRushGeneration(record.ShowTimeID, record.Generation)
	keys := []string{
		attemptRecordKey(s.prefix, record.ShowTimeID, generation, record.OrderNumber),
		userActiveKey(s.prefix, record.ShowTimeID, generation, record.UserID),
		userInflightKey(s.prefix, record.ShowTimeID, generation, record.UserID),
		quotaAvailableKey(s.prefix, record.ShowTimeID, generation, record.TicketCategoryID),
		orderProgressIndexKey(s.prefix, record.ShowTimeID, generation),
		seatOccupiedKey(s.prefix, record.ShowTimeID, generation, record.OrderNumber),
		userFingerprintIndexKey(s.prefix, record.ShowTimeID, generation, record.UserID),
	}
	for _, viewerID := range normalizedInt64s(record.ViewerIDs) {
		keys = append(keys, viewerActiveKey(s.prefix, record.ShowTimeID, generation, viewerID))
	}
	for _, viewerID := range normalizedInt64s(record.ViewerIDs) {
		keys = append(keys, viewerInflightKey(s.prefix, record.ShowTimeID, generation, viewerID))
	}

	result, err := s.redis.EvalCtx(
		ctx,
		releaseAttemptScript,
		keys,
		reason,
		record.TicketCount,
		now.UnixMilli(),
		s.finalStateTTLSeconds,
		record.TokenFingerprint,
		len(record.ViewerIDs),
	)
	if err != nil {
		return err
	}

	statusCode, err := parseEvalInt64(result)
	if err != nil {
		return err
	}
	if statusCode < 0 {
		return xerr.ErrOrderNotFound
	}

	return nil
}

func (s *AttemptStore) ReleaseClosedOrderProjection(ctx context.Context, record *AttemptRecord, now time.Time) error {
	if s == nil || s.redis == nil {
		return xerr.ErrInternal
	}
	if record == nil || record.OrderNumber <= 0 || record.UserID <= 0 || record.ProgramID <= 0 || record.ShowTimeID <= 0 || record.TicketCategoryID <= 0 {
		return xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	generation := normalizeRushGeneration(record.ShowTimeID, record.Generation)
	keys := []string{
		attemptRecordKey(s.prefix, record.ShowTimeID, generation, record.OrderNumber),
		userActiveKey(s.prefix, record.ShowTimeID, generation, record.UserID),
		userInflightKey(s.prefix, record.ShowTimeID, generation, record.UserID),
		quotaAvailableKey(s.prefix, record.ShowTimeID, generation, record.TicketCategoryID),
		orderProgressIndexKey(s.prefix, record.ShowTimeID, generation),
		seatOccupiedKey(s.prefix, record.ShowTimeID, generation, record.OrderNumber),
		userFingerprintIndexKey(s.prefix, record.ShowTimeID, generation, record.UserID),
	}
	for _, viewerID := range normalizedInt64s(record.ViewerIDs) {
		keys = append(keys, viewerActiveKey(s.prefix, record.ShowTimeID, generation, viewerID))
	}
	for _, viewerID := range normalizedInt64s(record.ViewerIDs) {
		keys = append(keys, viewerInflightKey(s.prefix, record.ShowTimeID, generation, viewerID))
	}

	result, err := s.redis.EvalCtx(
		ctx,
		releaseClosedOrderProjectionScript,
		keys,
		now.UnixMilli(),
		record.TicketCount,
		s.finalStateTTLSeconds,
		record.TokenFingerprint,
		len(record.ViewerIDs),
	)
	if err != nil {
		return err
	}

	statusCode, err := parseEvalInt64(result)
	if err != nil {
		return err
	}
	if statusCode < 0 {
		return xerr.ErrOrderNotFound
	}

	return nil
}

func (s *AttemptStore) ScanOrderNumbers(ctx context.Context, limit int64) ([]int64, error) {
	if s == nil || s.redis == nil {
		return nil, xerr.ErrInternal
	}
	if limit <= 0 {
		limit = 100
	}

	pattern := fmt.Sprintf("%s:*:attempt:*", s.prefix)
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
	record.ProcessingEpoch, err = parseFieldInt64(fields, attemptFieldProcessingEpoch)
	if err != nil {
		return nil, err
	}
	record.DBProbeAttempts, err = parseFieldInt64(fields, attemptFieldDBProbeAttempts)
	if err != nil {
		return nil, err
	}
	record.Generation = fields[attemptFieldGeneration]
	if record.Generation == "" {
		record.Generation = BuildRushGeneration(record.ShowTimeID)
	}
	record.TokenFingerprint = fields[attemptFieldTokenFingerprint]
	record.ViewerIDs, err = parseInt64CSV(fields[attemptFieldViewerIDs])
	if err != nil {
		return nil, err
	}
	record.CommitCutoffAt, err = parseFieldTime(fields, attemptFieldCommitCutoffAt)
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
	record.UserDeadlineAt, err = parseFieldTime(fields, attemptFieldUserDeadlineAt)
	if err != nil {
		return nil, err
	}
	record.ProcessingStartedAt, err = parseFieldTime(fields, attemptFieldProcessingStartAt)
	if err != nil {
		return nil, err
	}
	record.VerifyStartedAt, err = parseFieldTime(fields, attemptFieldVerifyStartedAt)
	if err != nil {
		return nil, err
	}
	record.LastDBProbeAt, err = parseFieldTime(fields, attemptFieldLastDBProbeAt)
	if err != nil {
		return nil, err
	}
	record.NextDBProbeAt, err = parseFieldTime(fields, attemptFieldNextDBProbeAt)
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

func (s *AttemptStore) resolveAttemptRecordKey(ctx context.Context, orderNumber int64) (string, error) {
	pattern := fmt.Sprintf("%s:*:attempt:%d", s.prefix, orderNumber)
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
