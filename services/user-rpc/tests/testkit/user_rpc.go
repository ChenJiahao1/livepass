package testkit

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	red "github.com/redis/go-redis/v9"

	"damai-go/pkg/xid"
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/user-rpc/internal/config"
	"damai-go/services/user-rpc/internal/model"
	"damai-go/services/user-rpc/internal/svc"
)

const (
	TestRedisAddr     = "127.0.0.1:6379"
	TestChannelCode   = "0001"
	TestChannelSecret = "local-user-secret-0001"
)

var TestMySQLDataSource = xmysql.WithLocalTime("root:123456@tcp(127.0.0.1:3306)/damai_user?parseTime=true")

type UserSeed struct {
	Name        string
	Mobile      string
	Email       string
	Password    string
	EmailStatus int64
	RelName     string
	IdNumber    string
	Address     string
}

func NewServiceContext(t *testing.T) *svc.ServiceContext {
	t.Helper()

	return svc.NewServiceContext(config.Config{
		MySQL: xmysql.Config{
			DataSource: TestMySQLDataSource,
		},
		StoreRedis: xredis.Config{
			Host: TestRedisAddr,
			Type: "node",
		},
		UserAuth: config.UserAuthConfig{
			TokenExpire:    time.Hour,
			LoginFailLimit: 2,
			ChannelMap: map[string]string{
				TestChannelCode: TestChannelSecret,
			},
		},
	})
}

func ResetDomainState(t *testing.T) {
	t.Helper()

	db, err := sql.Open("mysql", xmysql.WithLocalTime(TestMySQLDataSource))
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

	rdb := NewRedisClient(t)
	defer rdb.Close()
	if err := rdb.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("FlushDB error: %v", err)
	}
}

func MustSeedUser(t *testing.T, svcCtx *svc.ServiceContext, seed UserSeed) *model.DUser {
	t.Helper()

	now := time.Now()
	user := &model.DUser{
		Id:       xid.New(),
		Name:     nullString(seed.Name),
		Mobile:   seed.Mobile,
		Gender:   1,
		Password: sql.NullString{String: MD5Hex(seed.Password), Valid: seed.Password != ""},
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

func MustSeedTicketUser(t *testing.T, svcCtx *svc.ServiceContext, userID int64, relName string, idType int64, idNumber string) *model.DTicketUser {
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

func NewRedisClient(t *testing.T) *red.Client {
	t.Helper()
	return red.NewClient(&red.Options{Addr: TestRedisAddr})
}

func LoginStateKey(userID int64) string {
	return "user:login:token:" + strconv.FormatInt(userID, 10)
}

func MD5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
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
