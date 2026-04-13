package xmiddleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"damai-go/pkg/xerr"
)

const (
	UserIDHeader           = "X-User-Id"
	GatewayTimestampHeader = "X-Gateway-Timestamp"
	GatewaySignatureHeader = "X-Gateway-Signature"

	defaultGatewayClockSkew = 5 * time.Minute
)

func AttachGatewayIdentityHeaders(header http.Header, userID int64, secret string) error {
	if header == nil || userID <= 0 {
		return xerr.ErrUnauthorized
	}

	secret = strings.TrimSpace(secret)
	if secret == "" {
		return xerr.ErrUnauthorized
	}

	timestamp := time.Now().Unix()
	header.Set(UserIDHeader, strconv.FormatInt(userID, 10))
	header.Set(GatewayTimestampHeader, strconv.FormatInt(timestamp, 10))
	header.Set(GatewaySignatureHeader, signGatewayIdentity(userID, timestamp, secret))
	return nil
}

func AuthenticateGatewayIdentity(r *http.Request, secret string, maxClockSkew time.Duration) (int64, error) {
	if r == nil {
		return 0, xerr.ErrUnauthorized
	}

	secret = strings.TrimSpace(secret)
	if secret == "" {
		return 0, xerr.ErrUnauthorized
	}

	userID, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get(UserIDHeader)), 10, 64)
	if err != nil || userID <= 0 {
		return 0, xerr.ErrUnauthorized
	}

	timestamp, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get(GatewayTimestampHeader)), 10, 64)
	if err != nil {
		return 0, xerr.ErrUnauthorized
	}

	signature := strings.TrimSpace(r.Header.Get(GatewaySignatureHeader))
	if signature == "" {
		return 0, xerr.ErrUnauthorized
	}

	clockSkew := maxClockSkew
	if clockSkew <= 0 {
		clockSkew = defaultGatewayClockSkew
	}

	issuedAt := time.Unix(timestamp, 0)
	now := time.Now()
	if issuedAt.Before(now.Add(-clockSkew)) || issuedAt.After(now.Add(clockSkew)) {
		return 0, xerr.ErrUnauthorized
	}

	expected := signGatewayIdentity(userID, timestamp, secret)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return 0, xerr.ErrUnauthorized
	}

	return userID, nil
}

func ClearGatewayIdentityHeaders(header http.Header) {
	if header == nil {
		return
	}

	header.Del(UserIDHeader)
	header.Del(GatewayTimestampHeader)
	header.Del(GatewaySignatureHeader)
}

func ClearExternalAuthHeaders(header http.Header) {
	if header == nil {
		return
	}

	header.Del("Authorization")
}

func signGatewayIdentity(userID int64, timestamp int64, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(userID, 10) + ":" + strconv.FormatInt(timestamp, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}
