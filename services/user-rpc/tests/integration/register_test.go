package integration_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"damai-go/services/user-rpc/internal/logic"
	"damai-go/services/user-rpc/pb"
	"damai-go/services/user-rpc/tests/testkit"
)

func TestRegisterInsertsUser(t *testing.T) {
	svcCtx := testkit.NewServiceContext(t)
	testkit.ResetDomainState(t)

	l := logic.NewRegisterLogic(context.Background(), svcCtx)
	resp, err := l.Register(&pb.RegisterReq{
		Mobile:          "13800000000",
		Password:        "123456",
		ConfirmPassword: "123456",
		Mail:            "user@example.com",
		MailStatus:      1,
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}

	user, err := svcCtx.DUserModel.FindOneByMobile(context.Background(), "13800000000")
	if err != nil {
		t.Fatalf("FindOneByMobile returned error: %v", err)
	}
	if user.Mobile != "13800000000" {
		t.Fatalf("unexpected mobile: %s", user.Mobile)
	}
	if !user.Password.Valid {
		t.Fatalf("expected stored password")
	}
	if user.Password.String != testkit.MD5Hex("123456") {
		t.Fatalf("unexpected password hash: %s", user.Password.String)
	}
	if user.Id == 0 {
		t.Fatalf("expected non-zero user id")
	}

	mobileMapping, err := svcCtx.DUserMobileModel.FindOneByMobile(context.Background(), "13800000000")
	if err != nil {
		t.Fatalf("FindOneByMobile on mapping returned error: %v", err)
	}
	if mobileMapping.UserId != user.Id {
		t.Fatalf("unexpected mobile mapping user id: %d", mobileMapping.UserId)
	}

	emailMapping, err := svcCtx.DUserEmailModel.FindOneByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("FindOneByEmail returned error: %v", err)
	}
	if emailMapping.UserId != user.Id {
		t.Fatalf("unexpected email mapping user id: %d", emailMapping.UserId)
	}
}

func TestRegisterRejectsDuplicateMobile(t *testing.T) {
	svcCtx := testkit.NewServiceContext(t)
	testkit.ResetDomainState(t)
	testkit.MustSeedUser(t, svcCtx, testkit.UserSeed{
		Mobile:   "13800000001",
		Password: "123456",
	})

	l := logic.NewRegisterLogic(context.Background(), svcCtx)
	_, err := l.Register(&pb.RegisterReq{
		Mobile:          "13800000001",
		Password:        "abcdef",
		ConfirmPassword: "abcdef",
	})
	if err == nil {
		t.Fatalf("expected duplicate mobile error")
	}
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected already exists code, got %s", status.Code(err))
	}
}

func TestRegisterRejectsConfirmPasswordMismatch(t *testing.T) {
	svcCtx := testkit.NewServiceContext(t)
	testkit.ResetDomainState(t)

	l := logic.NewRegisterLogic(context.Background(), svcCtx)
	_, err := l.Register(&pb.RegisterReq{
		Mobile:          "13800000002",
		Password:        "123456",
		ConfirmPassword: "654321",
	})
	if err == nil {
		t.Fatalf("expected invalid argument error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument code, got %s", status.Code(err))
	}
}

func TestExistReturnsSuccessWhenMobileMissing(t *testing.T) {
	svcCtx := testkit.NewServiceContext(t)
	testkit.ResetDomainState(t)

	l := logic.NewExistLogic(context.Background(), svcCtx)
	resp, err := l.Exist(&pb.ExistReq{Mobile: "13800000009"})
	if err != nil {
		t.Fatalf("Exist returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
}

func TestExistRejectsDuplicateMobile(t *testing.T) {
	svcCtx := testkit.NewServiceContext(t)
	testkit.ResetDomainState(t)
	testkit.MustSeedUser(t, svcCtx, testkit.UserSeed{
		Mobile:   "13800000010",
		Password: "123456",
	})

	l := logic.NewExistLogic(context.Background(), svcCtx)
	_, err := l.Exist(&pb.ExistReq{Mobile: "13800000010"})
	if err == nil {
		t.Fatalf("expected already exists error")
	}
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected already exists code, got %s", status.Code(err))
	}
}
