package logic

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const rushContractPurchaseTokenVersion = "v1"

var rushContractNow = func() time.Time {
	return time.Now()
}

func encodeRushContractPurchaseToken(userID, orderNumber int64) (string, error) {
	if userID <= 0 || orderNumber <= 0 {
		return "", status.Error(codes.InvalidArgument, "invalid rush contract token args")
	}

	raw := fmt.Sprintf("%s:%d:%d", rushContractPurchaseTokenVersion, userID, orderNumber)
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

func decodeRushContractPurchaseToken(token string) (int64, int64, error) {
	if token == "" {
		return 0, 0, status.Error(codes.InvalidArgument, "purchaseToken is empty")
	}

	payload, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, 0, status.Error(codes.InvalidArgument, "invalid purchaseToken")
	}
	parts := strings.Split(string(payload), ":")
	if len(parts) != 3 || parts[0] != rushContractPurchaseTokenVersion {
		return 0, 0, status.Error(codes.InvalidArgument, "invalid purchaseToken")
	}

	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || userID <= 0 {
		return 0, 0, status.Error(codes.InvalidArgument, "invalid purchaseToken")
	}
	orderNumber, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || orderNumber <= 0 {
		return 0, 0, status.Error(codes.InvalidArgument, "invalid purchaseToken")
	}

	return userID, orderNumber, nil
}

func allocateRushContractOrderNumber(userID int64) int64 {
	return defaultOrderNumberGenerator.Next(userID, rushContractNow())
}
