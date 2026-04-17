package logic

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"livepass/pkg/xerr"
	"livepass/services/user-rpc/internal/model"
	"livepass/services/user-rpc/internal/svc"
	"livepass/services/user-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func buildUserInfo(user *model.DUser) *pb.UserInfo {
	return &pb.UserInfo{
		Id:                      user.Id,
		Name:                    nullStringValue(user.Name),
		RelName:                 nullStringValue(user.RelName),
		Gender:                  user.Gender,
		Mobile:                  user.Mobile,
		EmailStatus:             user.EmailStatus,
		Email:                   nullStringValue(user.Email),
		RelAuthenticationStatus: user.RelAuthenticationStatus,
		IdNumber:                nullStringValue(user.IdNumber),
		Address:                 nullStringValue(user.Address),
	}
}

func buildTicketUserInfo(ticketUser *model.DTicketUser) *pb.TicketUserInfo {
	return &pb.TicketUserInfo{
		Id:       ticketUser.Id,
		UserId:   ticketUser.UserId,
		RelName:  ticketUser.RelName,
		IdType:   ticketUser.IdType,
		IdNumber: ticketUser.IdNumber,
	}
}

func accessSecret(svcCtx *svc.ServiceContext) (string, error) {
	secret := strings.TrimSpace(svcCtx.Config.UserAuth.AccessSecret)
	if secret == "" {
		return "", status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return secret, nil
}

func loginStateKey(userID int64) string {
	return fmt.Sprintf("user:login:token:%d", userID)
}

func mobileFailKey(mobile string) string {
	return "user:login:fail:mobile:" + mobile
}

func emailFailKey(email string) string {
	return "user:login:fail:email:" + email
}

func durationSeconds(d time.Duration) int {
	seconds := int(d / time.Second)
	if seconds <= 0 {
		return 1
	}
	return seconds
}

func parseFailCount(value string) int64 {
	count, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return count
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func normalizeOptionalEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}

	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || parsed.Address != trimmed {
		return "", status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return trimmed, nil
}

func emailStatusForAddress(email string) int64 {
	if email == "" {
		return 0
	}
	return 1
}

func nullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
