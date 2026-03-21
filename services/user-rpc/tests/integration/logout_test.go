package integration_test

import (
	"context"
	"testing"

	"damai-go/pkg/xjwt"
	"damai-go/services/user-rpc/internal/logic"
	"damai-go/services/user-rpc/pb"
	"damai-go/services/user-rpc/tests/testkit"
)

func TestLogoutDeletesLoginState(t *testing.T) {
	svcCtx := testkit.NewServiceContext(t)
	testkit.ResetDomainState(t)
	user := testkit.MustSeedUser(t, svcCtx, testkit.UserSeed{
		Mobile:   "13800000020",
		Password: "123456",
	})

	token, err := xjwt.CreateToken(user.Id, testkit.TestChannelSecret, svcCtx.Config.UserAuth.TokenExpire)
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}

	rdb := testkit.NewRedisClient(t)
	defer rdb.Close()
	if err := rdb.Set(context.Background(), testkit.LoginStateKey(user.Id), token, 0).Err(); err != nil {
		t.Fatalf("redis Set returned error: %v", err)
	}

	l := logic.NewLogoutLogic(context.Background(), svcCtx)
	resp, err := l.Logout(&pb.LogoutReq{
		Code:  testkit.TestChannelCode,
		Token: token,
	})
	if err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}

	count, err := rdb.Exists(context.Background(), testkit.LoginStateKey(user.Id)).Result()
	if err != nil {
		t.Fatalf("redis Exists returned error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected login state deleted")
	}
}
