package logic

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	red "github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"damai-go/pkg/xid"
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/user-rpc/internal/config"
	"damai-go/services/user-rpc/internal/model"
	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"
)

const (
	testMySQLDataSource = "root:123456@tcp(127.0.0.1:3306)/damai_user?parseTime=true"
	testRedisAddr       = "127.0.0.1:6379"
	testChannelCode     = "0001"
	testChannelSecret   = "local-user-secret-0001"
)

func TestRegisterInsertsUser(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)

	l := NewRegisterLogic(context.Background(), svcCtx)

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
	if user.Password.String != md5Hex("123456") {
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
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000001",
		Password: "123456",
	})

	l := NewRegisterLogic(context.Background(), svcCtx)
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
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)

	l := NewRegisterLogic(context.Background(), svcCtx)
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
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)

	l := NewExistLogic(context.Background(), svcCtx)
	resp, err := l.Exist(&pb.ExistReq{Mobile: "13800000009"})
	if err != nil {
		t.Fatalf("Exist returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
}

func TestExistRejectsDuplicateMobile(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000010",
		Password: "123456",
	})

	l := NewExistLogic(context.Background(), svcCtx)
	_, err := l.Exist(&pb.ExistReq{Mobile: "13800000010"})
	if err == nil {
		t.Fatalf("expected already exists error")
	}
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected already exists code, got %s", status.Code(err))
	}
}

func newTestServiceContext(t *testing.T) *svc.ServiceContext {
	t.Helper()
	return svc.NewServiceContext(config.Config{
		MySQL: xmysql.Config{
			DataSource: testMySQLDataSource,
		},
		StoreRedis: xredis.Config{
			Host: testRedisAddr,
			Type: "node",
		},
		UserAuth: config.UserAuthConfig{
			TokenExpire:    time.Hour,
			LoginFailLimit: 2,
			ChannelMap: map[string]string{
				testChannelCode: testChannelSecret,
			},
		},
	})
}

func resetUserDomainState(t *testing.T) {
	t.Helper()

	db, err := sql.Open("mysql", testMySQLDataSource)
	if err != nil {
		t.Fatalf("sql.Open error: %v", err)
	}
	defer db.Close()

	for _, ddlPath := range []string{
		"sql/user/d_ticket_user.sql",
		"sql/user/d_user_email.sql",
		"sql/user/d_user_mobile.sql",
		"sql/user/d_user.sql",
	} {
		execSQLFile(t, db, ddlPath)
	}

	rdb := red.NewClient(&red.Options{Addr: testRedisAddr})
	defer rdb.Close()
	if err := rdb.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("FlushDB error: %v", err)
	}
}

type userSeed struct {
	Name        string
	Mobile      string
	Email       string
	Password    string
	EmailStatus int64
	RelName     string
	IdNumber    string
	Address     string
}

func mustSeedUser(t *testing.T, svcCtx *svc.ServiceContext, seed userSeed) *model.DUser {
	t.Helper()

	now := time.Now()
	user := &model.DUser{
		Id:       xid.New(),
		Name:     nullString(seed.Name),
		Mobile:   seed.Mobile,
		Gender:   1,
		Password: sql.NullString{String: md5Hex(seed.Password), Valid: seed.Password != ""},
		Email:    nullString(seed.Email),
		EmailStatus: func() int64 {
			if seed.Email != "" && seed.EmailStatus == 0 {
				return 1
			}
			return seed.EmailStatus
		}(),
		RelName:                 nullString(seed.RelName),
		IdNumber:                nullString(seed.IdNumber),
		Address:                 nullString(seed.Address),
		RelAuthenticationStatus: boolToInt64(seed.RelName != "" && seed.IdNumber != ""),
		EditTime:                sql.NullTime{Time: now, Valid: true},
		Status:                  1,
	}
	if _, err := svcCtx.DUserModel.Insert(context.Background(), user); err != nil {
		t.Fatalf("insert d_user error: %v", err)
	}

	mobile := &model.DUserMobile{
		Id:       xid.New(),
		UserId:   user.Id,
		Mobile:   seed.Mobile,
		EditTime: sql.NullTime{Time: now, Valid: true},
		Status:   1,
	}
	if _, err := svcCtx.DUserMobileModel.Insert(context.Background(), mobile); err != nil {
		t.Fatalf("insert d_user_mobile error: %v", err)
	}

	if seed.Email != "" {
		email := &model.DUserEmail{
			Id:          xid.New(),
			UserId:      user.Id,
			Email:       seed.Email,
			EmailStatus: user.EmailStatus,
			EditTime:    sql.NullTime{Time: now, Valid: true},
			Status:      1,
		}
		if _, err := svcCtx.DUserEmailModel.Insert(context.Background(), email); err != nil {
			t.Fatalf("insert d_user_email error: %v", err)
		}
	}

	return user
}

func mustSeedTicketUser(t *testing.T, svcCtx *svc.ServiceContext, userID int64, relName string, idType int64, idNumber string) *model.DTicketUser {
	t.Helper()

	ticketUser := &model.DTicketUser{
		Id:       xid.New(),
		UserId:   userID,
		RelName:  relName,
		IdType:   idType,
		IdNumber: idNumber,
		EditTime: sql.NullTime{Time: time.Now(), Valid: true},
		Status:   1,
	}
	if _, err := svcCtx.DTicketUserModel.Insert(context.Background(), ticketUser); err != nil {
		t.Fatalf("insert d_ticket_user error: %v", err)
	}

	return ticketUser
}

func execSQLFile(t *testing.T, db *sql.DB, relativePath string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(projectRoot(t), relativePath))
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", relativePath, err)
	}

	for _, stmt := range strings.Split(string(content), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %s error: %v\nstatement: %s", relativePath, err, stmt)
		}
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func boolToInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
