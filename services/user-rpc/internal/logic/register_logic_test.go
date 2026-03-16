package logic

import (
	"context"
	"database/sql"
	"testing"

	"damai-go/pkg/xmysql"
	"damai-go/services/user-rpc/internal/config"
	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testMySQLDataSource = "root:123456@tcp(127.0.0.1:3306)/damai_user?parseTime=true"

func TestRegisterInsertsUser(t *testing.T) {
	resetDUserTable(t)

	svcCtx := newTestServiceContext()
	l := NewRegisterLogic(context.Background(), svcCtx)

	resp, err := l.Register(&pb.RegisterReq{
		Mobile:   "13800000000",
		Password: "123456",
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
	expectedHash := md5Hex("123456")
	if user.Password.String != expectedHash {
		t.Fatalf("unexpected password hash: got %s want %s", user.Password.String, expectedHash)
	}
	if user.Id == 0 {
		t.Fatalf("expected non-zero user id")
	}
}

func TestRegisterRejectsDuplicateMobile(t *testing.T) {
	resetDUserTable(t)

	svcCtx := newTestServiceContext()
	l := NewRegisterLogic(context.Background(), svcCtx)

	_, err := l.Register(&pb.RegisterReq{
		Mobile:   "13800000001",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("first Register returned error: %v", err)
	}

	_, err = l.Register(&pb.RegisterReq{
		Mobile:   "13800000001",
		Password: "abcdef",
	})
	if err == nil {
		t.Fatalf("expected duplicate mobile error")
	}
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected already exists code, got %s", status.Code(err))
	}
}

func TestRegisterRejectsInvalidArgument(t *testing.T) {
	resetDUserTable(t)

	svcCtx := newTestServiceContext()
	l := NewRegisterLogic(context.Background(), svcCtx)

	_, err := l.Register(&pb.RegisterReq{
		Mobile:   "",
		Password: "123456",
	})
	if err == nil {
		t.Fatalf("expected invalid argument error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument code, got %s", status.Code(err))
	}
}

func newTestServiceContext() *svc.ServiceContext {
	return svc.NewServiceContext(config.Config{
		MySQL: xmysql.Config{
			DataSource: testMySQLDataSource,
		},
	})
}

func resetDUserTable(t *testing.T) {
	t.Helper()

	db, err := sql.Open("mysql", testMySQLDataSource)
	if err != nil {
		t.Fatalf("sql.Open error: %v", err)
	}
	defer db.Close()

	dropDDL := `DROP TABLE IF EXISTS d_user;`
	createDDL := `CREATE TABLE d_user (
  id BIGINT NOT NULL COMMENT '主键id',
  name VARCHAR(256) DEFAULT NULL COMMENT '用户名字',
  rel_name VARCHAR(256) DEFAULT NULL COMMENT '用户真实名字',
  mobile VARCHAR(512) NOT NULL COMMENT '手机号',
  gender INT NOT NULL DEFAULT 1 COMMENT '1:男 2:女',
  password VARCHAR(512) DEFAULT NULL COMMENT '密码',
  email_status TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否邮箱认证 1:已验证 0:未验证',
  email VARCHAR(256) DEFAULT NULL COMMENT '邮箱地址',
  rel_authentication_status TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否实名认证 1:已验证 0:未验证',
  id_number VARCHAR(512) DEFAULT NULL COMMENT '身份证号码',
  address VARCHAR(256) DEFAULT NULL COMMENT '收货地址',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  edit_time DATETIME DEFAULT NULL COMMENT '编辑时间',
  status TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1:正常 0:删除',
  PRIMARY KEY (id),
  KEY idx_d_user_mobile (mobile)
);`
	if _, err := db.Exec(dropDDL); err != nil {
		t.Fatalf("drop d_user table error: %v", err)
	}
	if _, err := db.Exec(createDDL); err != nil {
		t.Fatalf("reset d_user table error: %v", err)
	}
}
