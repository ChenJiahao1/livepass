package logic

import (
	"context"
	"testing"

	red "github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"damai-go/pkg/xjwt"
	"damai-go/services/user-rpc/pb"
)

func TestLoginByMobileReturnsToken(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Name:     "mobile-user",
		Mobile:   "13800000010",
		Password: "123456",
	})

	l := NewLoginLogic(context.Background(), svcCtx)
	resp, err := l.Login(&pb.LoginReq{
		Code:     testChannelCode,
		Mobile:   "13800000010",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if resp.UserId != user.Id {
		t.Fatalf("unexpected user id: %d", resp.UserId)
	}
	if resp.Token == "" {
		t.Fatalf("expected non-empty token")
	}
	claims, err := xjwt.ParseToken(resp.Token, testChannelSecret)
	if err != nil {
		t.Fatalf("ParseToken returned error: %v", err)
	}
	if claims.UserID != user.Id {
		t.Fatalf("unexpected token user id: %d", claims.UserID)
	}

	tokenInRedis, err := newRedisClient(t).Get(context.Background(), loginStateKey(user.Id)).Result()
	if err != nil {
		t.Fatalf("redis Get returned error: %v", err)
	}
	if tokenInRedis != resp.Token {
		t.Fatalf("unexpected token in redis: %s", tokenInRedis)
	}
}

func TestLoginByEmailReturnsToken(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Name:        "email-user",
		Mobile:      "13800000011",
		Email:       "user11@example.com",
		EmailStatus: 1,
		Password:    "123456",
	})

	l := NewLoginLogic(context.Background(), svcCtx)
	resp, err := l.Login(&pb.LoginReq{
		Code:     testChannelCode,
		Email:    "user11@example.com",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if resp.UserId != user.Id {
		t.Fatalf("unexpected user id: %d", resp.UserId)
	}
	if resp.Token == "" {
		t.Fatalf("expected non-empty token")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000012",
		Password: "123456",
	})

	l := NewLoginLogic(context.Background(), svcCtx)
	_, err := l.Login(&pb.LoginReq{
		Code:     testChannelCode,
		Mobile:   "13800000012",
		Password: "654321",
	})
	if err == nil {
		t.Fatalf("expected unauthenticated error")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected unauthenticated code, got %s", status.Code(err))
	}
}

func newRedisClient(t *testing.T) *red.Client {
	t.Helper()
	return red.NewClient(&red.Options{Addr: testRedisAddr})
}
