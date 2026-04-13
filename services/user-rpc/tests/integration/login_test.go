package integration_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"damai-go/pkg/xjwt"
	"damai-go/services/user-rpc/internal/logic"
	"damai-go/services/user-rpc/pb"
	"damai-go/services/user-rpc/tests/testkit"
)

func TestLoginByMobileReturnsToken(t *testing.T) {
	svcCtx := testkit.NewServiceContext(t)
	testkit.ResetDomainState(t)
	user := testkit.MustSeedUser(t, svcCtx, testkit.UserSeed{
		Name:     "mobile-user",
		Mobile:   "13800000010",
		Password: "123456",
	})

	l := logic.NewLoginLogic(context.Background(), svcCtx)
	resp, err := l.Login(&pb.LoginReq{
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

	claims, err := xjwt.ParseToken(resp.Token, testkit.TestAccessSecret)
	if err != nil {
		t.Fatalf("ParseToken returned error: %v", err)
	}
	if claims.UserID != user.Id {
		t.Fatalf("unexpected token user id: %d", claims.UserID)
	}

	tokenInRedis, err := testkit.NewRedisClient(t).Get(context.Background(), testkit.LoginStateKey(user.Id)).Result()
	if err != nil {
		t.Fatalf("redis Get returned error: %v", err)
	}
	if tokenInRedis != resp.Token {
		t.Fatalf("unexpected token in redis: %s", tokenInRedis)
	}
}

func TestLoginByEmailReturnsToken(t *testing.T) {
	svcCtx := testkit.NewServiceContext(t)
	testkit.ResetDomainState(t)
	user := testkit.MustSeedUser(t, svcCtx, testkit.UserSeed{
		Name:        "email-user",
		Mobile:      "13800000011",
		Email:       "user11@example.com",
		EmailStatus: 1,
		Password:    "123456",
	})

	l := logic.NewLoginLogic(context.Background(), svcCtx)
	resp, err := l.Login(&pb.LoginReq{
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
	svcCtx := testkit.NewServiceContext(t)
	testkit.ResetDomainState(t)
	testkit.MustSeedUser(t, svcCtx, testkit.UserSeed{
		Mobile:   "13800000012",
		Password: "123456",
	})

	l := logic.NewLoginLogic(context.Background(), svcCtx)
	_, err := l.Login(&pb.LoginReq{
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
