package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"damai-go/pkg/xredis"
)

const (
	defaultOrderLimitLedgerPrefix = "damai-go:order:purchase-limit"
	defaultLedgerTTLSeconds       = 4 * 60 * 60
)

type userPoolEntry struct {
	JWT string `json:"jwt"`
}

func main() {
	userPoolFile := strings.TrimSpace(os.Getenv("USER_POOL_FILE"))
	if userPoolFile == "" {
		exitWithError("USER_POOL_FILE is required")
	}

	userIDs, err := loadUserPoolUsers(userPoolFile)
	if err != nil {
		exitWithError(err.Error())
	}

	programID := envInt64("PROGRAM_ID", 10001)
	redisHost := envString("REDIS_HOST", "127.0.0.1")
	redisPort := envInt("REDIS_PORT", 6379)
	ledgerPrefix := envString("ORDER_LIMIT_LEDGER_PREFIX", defaultOrderLimitLedgerPrefix)
	ledgerTTLSeconds := envInt("ORDER_LIMIT_LEDGER_TTL_SECONDS", defaultLedgerTTLSeconds)

	redisClient := xredis.MustNew(xredis.Config{
		Host: fmt.Sprintf("%s:%d", redisHost, redisPort),
		Type: "node",
	})

	if err := seedOrderLimitLedgers(context.Background(), redisClient, ledgerPrefix, userIDs, programID, ledgerTTLSeconds); err != nil {
		exitWithError(err.Error())
	}

	fmt.Printf(
		"prewarmed order limit ledgers: users=%d programId=%d redis=%s:%d ttl=%ds\n",
		len(userIDs),
		programID,
		redisHost,
		redisPort,
		ledgerTTLSeconds,
	)
}

func loadUserPoolUsers(path string) ([]int64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []userPoolEntry
	if err := json.Unmarshal(content, &entries); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("user pool is empty: %s", path)
	}

	userIDs := make([]int64, 0, len(entries))
	seen := make(map[int64]struct{}, len(entries))
	for index, entry := range entries {
		userID, err := decodeJWTUserID(entry.JWT)
		if err != nil {
			return nil, fmt.Errorf("decode jwt user id at index %d: %w", index, err)
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}

func decodeJWTUserID(token string) (int64, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid jwt format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, err
	}

	var claims struct {
		UserID int64 `json:"userId"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, err
	}
	if claims.UserID <= 0 {
		return 0, fmt.Errorf("invalid userId in jwt")
	}

	return claims.UserID, nil
}

func seedOrderLimitLedgers(ctx context.Context, redisClient *xredis.Client, prefix string, userIDs []int64, programID int64, ttlSeconds int) error {
	if redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}

	sortedUserIDs := append([]int64(nil), userIDs...)
	sort.Slice(sortedUserIDs, func(i, j int) bool {
		return sortedUserIDs[i] < sortedUserIDs[j]
	})

	for _, userID := range sortedUserIDs {
		ledgerKey := fmt.Sprintf("%s:ledger:%d:%d", prefix, userID, programID)
		loadingKey := fmt.Sprintf("%s:loading:%d:%d", prefix, userID, programID)

		if _, err := redisClient.DelCtx(ctx, loadingKey); err != nil {
			return err
		}
		if err := redisClient.HsetCtx(ctx, ledgerKey, "active_count", "0"); err != nil {
			return err
		}
		if ttlSeconds > 0 {
			if err := redisClient.ExpireCtx(ctx, ledgerKey, ttlSeconds); err != nil {
				return err
			}
		}
	}

	return nil
}

func envString(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		exitWithError(fmt.Sprintf("invalid %s: %v", name, err))
	}

	return parsed
}

func envInt64(name string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		exitWithError(fmt.Sprintf("invalid %s: %v", name, err))
	}

	return parsed
}

func exitWithError(message string) {
	fmt.Fprintf(os.Stderr, "[prewarm-order-limit-ledgers] ERROR: %s\n", message)
	os.Exit(1)
}
