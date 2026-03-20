package logic

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/user-rpc/internal/model"
	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

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

func channelSecret(svcCtx *svc.ServiceContext, code string) (string, error) {
	if code == "" {
		return "", status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	secret, ok := svcCtx.Config.UserAuth.ChannelMap[code]
	if !ok || secret == "" {
		return "", status.Error(codes.InvalidArgument, xerr.ErrChannelNotFound.Error())
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

func nullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
