package rush

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const purchaseTokenVersion = "v1"

var (
	ErrInvalidPurchaseToken = errors.New("invalid purchase token")
	ErrExpiredPurchaseToken = errors.New("expired purchase token")
)

type PurchaseTokenClaims struct {
	OrderNumber      int64   `json:"orderNumber"`
	UserID           int64   `json:"userId"`
	ProgramID        int64   `json:"programId"`
	ShowTimeID       int64   `json:"showTimeId"`
	TicketCategoryID int64   `json:"ticketCategoryId"`
	TicketUserIDs    []int64 `json:"ticketUserIds"`
	TicketCount      int64   `json:"ticketCount"`
	SaleWindowEndAt  int64   `json:"saleWindowEndAt"`
	ShowEndAt        int64   `json:"showEndAt"`
	DistributionMode string  `json:"distributionMode,omitempty"`
	TakeTicketMode   string  `json:"takeTicketMode,omitempty"`
	ExpireAt         int64   `json:"expireAt"`
	TokenFingerprint string  `json:"tokenFingerprint"`
}

type PurchaseTokenCodec struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewPurchaseTokenCodec(secret string, ttl time.Duration) (*PurchaseTokenCodec, error) {
	if strings.TrimSpace(secret) == "" || ttl <= 0 {
		return nil, ErrInvalidPurchaseToken
	}

	return &PurchaseTokenCodec{
		secret: []byte(secret),
		ttl:    ttl,
		now:    time.Now,
	}, nil
}

func MustNewPurchaseTokenCodec(secret string, ttl time.Duration) *PurchaseTokenCodec {
	codec, err := NewPurchaseTokenCodec(secret, ttl)
	if err != nil {
		panic(err)
	}

	return codec
}

func (c *PurchaseTokenCodec) Issue(claims PurchaseTokenClaims) (string, error) {
	if c == nil {
		return "", ErrInvalidPurchaseToken
	}

	normalized, err := c.normalizeClaims(claims)
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}

	signature := c.sign(payload)
	return fmt.Sprintf(
		"%s.%s.%s",
		purchaseTokenVersion,
		base64.RawURLEncoding.EncodeToString(payload),
		base64.RawURLEncoding.EncodeToString(signature),
	), nil
}

func (c *PurchaseTokenCodec) Verify(token string) (*PurchaseTokenClaims, error) {
	if c == nil || token == "" {
		return nil, ErrInvalidPurchaseToken
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != purchaseTokenVersion {
		return nil, ErrInvalidPurchaseToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidPurchaseToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidPurchaseToken
	}
	if !hmac.Equal(signature, c.sign(payload)) {
		return nil, ErrInvalidPurchaseToken
	}

	var claims PurchaseTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidPurchaseToken
	}
	normalized, err := c.normalizeClaims(claims)
	if err != nil {
		return nil, ErrInvalidPurchaseToken
	}
	if normalized.ExpireAt <= c.now().UTC().Unix() {
		return nil, ErrExpiredPurchaseToken
	}

	return &normalized, nil
}

func (c *PurchaseTokenCodec) normalizeClaims(claims PurchaseTokenClaims) (PurchaseTokenClaims, error) {
	if claims.OrderNumber <= 0 || claims.UserID <= 0 || claims.ProgramID <= 0 || claims.TicketCategoryID <= 0 {
		return PurchaseTokenClaims{}, ErrInvalidPurchaseToken
	}
	if claims.ShowTimeID <= 0 {
		claims.ShowTimeID = claims.ProgramID
	}
	if len(claims.TicketUserIDs) == 0 {
		return PurchaseTokenClaims{}, ErrInvalidPurchaseToken
	}

	if claims.TicketCount == 0 {
		claims.TicketCount = int64(len(claims.TicketUserIDs))
	}
	if claims.TicketCount != int64(len(claims.TicketUserIDs)) {
		return PurchaseTokenClaims{}, ErrInvalidPurchaseToken
	}
	if claims.ExpireAt == 0 {
		claims.ExpireAt = c.now().UTC().Add(c.ttl).Unix()
	}
	if claims.SaleWindowEndAt == 0 {
		claims.SaleWindowEndAt = claims.ExpireAt
	}
	if claims.ShowEndAt == 0 {
		claims.ShowEndAt = claims.SaleWindowEndAt
	}
	if claims.TokenFingerprint == "" {
		claims.TokenFingerprint = BuildTokenFingerprint(
			claims.OrderNumber,
			claims.UserID,
			claims.ShowTimeID,
			claims.TicketCategoryID,
			claims.TicketUserIDs,
			claims.DistributionMode,
			claims.TakeTicketMode,
		)
	}

	return claims, nil
}

func (c *PurchaseTokenCodec) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, c.secret)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

func BuildTokenFingerprint(
	orderNumber, userID, showTimeID, ticketCategoryID int64,
	ticketUserIDs []int64,
	distributionMode, takeTicketMode string,
) string {
	sortedTicketUserIDs := append([]int64(nil), ticketUserIDs...)
	sort.Slice(sortedTicketUserIDs, func(i, j int) bool {
		return sortedTicketUserIDs[i] < sortedTicketUserIDs[j]
	})

	payload, _ := json.Marshal(struct {
		OrderNumber      int64   `json:"orderNumber"`
		UserID           int64   `json:"userId"`
		ShowTimeID       int64   `json:"showTimeId"`
		TicketCategoryID int64   `json:"ticketCategoryId"`
		TicketUserIDs    []int64 `json:"ticketUserIds"`
		DistributionMode string  `json:"distributionMode,omitempty"`
		TakeTicketMode   string  `json:"takeTicketMode,omitempty"`
	}{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    sortedTicketUserIDs,
		DistributionMode: distributionMode,
		TakeTicketMode:   takeTicketMode,
	})
	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:])
}
