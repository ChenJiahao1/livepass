package logic

import (
	"context"
	"testing"

	"damai-go/pkg/xjwt"
	"damai-go/services/user-rpc/pb"
)

func TestLogoutDeletesLoginState(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000020",
		Password: "123456",
	})

	token, err := xjwt.CreateToken(user.Id, testChannelSecret, svcCtx.Config.UserAuth.TokenExpire)
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}

	rdb := newRedisClient(t)
	defer rdb.Close()
	if err := rdb.Set(context.Background(), loginStateKey(user.Id), token, 0).Err(); err != nil {
		t.Fatalf("redis Set returned error: %v", err)
	}

	l := NewLogoutLogic(context.Background(), svcCtx)
	resp, err := l.Logout(&pb.LogoutReq{
		Code:  testChannelCode,
		Token: token,
	})
	if err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}

	count, err := rdb.Exists(context.Background(), loginStateKey(user.Id)).Result()
	if err != nil {
		t.Fatalf("redis Exists returned error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected login state deleted")
	}
}
