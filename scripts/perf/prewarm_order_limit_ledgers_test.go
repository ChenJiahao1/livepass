package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeTestJWT(t *testing.T, userID int64, exp int64) string {
	t.Helper()

	header := map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	}
	payload := map[string]any{
		"userId": userID,
		"exp":    exp,
		"iat":    exp - 3600,
	}

	encode := func(value any) string {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal jwt part: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}

	return encode(header) + "." + encode(payload) + ".signature"
}

func TestLoadUserPoolUsersDeduplicatesUserIDs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	poolFile := filepath.Join(tmpDir, "user_pool.json")
	expiresAt := time.Now().Add(time.Hour).Unix()
	content := []map[string]any{
		{
			"jwt":           makeTestJWT(t, 101, expiresAt),
			"ticketUserIds": []string{"1001", "1002", "1003"},
		},
		{
			"jwt":           makeTestJWT(t, 202, expiresAt),
			"ticketUserIds": []string{"2001", "2002", "2003"},
		},
		{
			"jwt":           makeTestJWT(t, 101, expiresAt),
			"ticketUserIds": []string{"3001", "3002", "3003"},
		},
	}

	raw, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("marshal pool: %v", err)
	}
	if err := os.WriteFile(poolFile, raw, 0o644); err != nil {
		t.Fatalf("write pool: %v", err)
	}

	users, err := loadUserPoolUsers(poolFile)
	if err != nil {
		t.Fatalf("loadUserPoolUsers returned error: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("expected 2 unique users, got %d", len(users))
	}
	if users[0] != 101 || users[1] != 202 {
		t.Fatalf("unexpected user ids: %+v", users)
	}
}
