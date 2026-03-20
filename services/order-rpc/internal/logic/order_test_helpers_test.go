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

	"damai-go/pkg/xmysql"
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc"
)

const testOrderMySQLDataSource = "root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true"

const (
	testOrderStatusUnpaid    int64 = 1
	testOrderStatusCancelled int64 = 2
)

type orderFixture struct {
	ID                      int64
	OrderNumber             int64
	ProgramID               int64
	ProgramTitle            string
	ProgramItemPicture      string
	ProgramPlace            string
	ProgramShowTime         string
	ProgramPermitChooseSeat int64
	UserID                  int64
	DistributionMode        string
	TakeTicketMode          string
	TicketCount             int64
	OrderPrice              int64
	OrderStatus             int64
	FreezeToken             string
	OrderExpireTime         string
	CreateOrderTime         string
	CancelOrderTime         string
}

type orderTicketUserFixture struct {
	ID                 int64
	OrderNumber        int64
	UserID             int64
	TicketUserID       int64
	TicketUserName     string
	TicketUserIDNumber string
	TicketCategoryID   int64
	TicketCategoryName string
	TicketPrice        int64
	SeatID             int64
	SeatRow            int64
	SeatCol            int64
	SeatPrice          int64
	OrderStatus        int64
	CreateOrderTime    string
}

type fakeOrderProgramRPC struct {
	getProgramPreorderResp    *programrpc.ProgramPreorderInfo
	getProgramPreorderErr     error
	lastGetProgramPreorderReq *programrpc.GetProgramDetailReq

	autoAssignAndFreezeSeatsResp    *programrpc.AutoAssignAndFreezeSeatsResp
	autoAssignAndFreezeSeatsErr     error
	lastAutoAssignAndFreezeSeatsReq *programrpc.AutoAssignAndFreezeSeatsReq

	releaseSeatFreezeResp    *programrpc.ReleaseSeatFreezeResp
	releaseSeatFreezeErr     error
	lastReleaseSeatFreezeReq *programrpc.ReleaseSeatFreezeReq
	releaseSeatFreezeCalls   int
}

type fakeOrderUserRPC struct {
	getUserAndTicketUserListResp    *userrpc.GetUserAndTicketUserListResp
	getUserAndTicketUserListErr     error
	lastGetUserAndTicketUserListReq *userrpc.GetUserAndTicketUserListReq
}

func newOrderTestServiceContext(t *testing.T) (*svc.ServiceContext, *fakeOrderProgramRPC, *fakeOrderUserRPC) {
	t.Helper()

	cfg := config.Config{
		MySQL: xmysql.Config{
			DataSource: testOrderMySQLDataSource,
		},
		Order: config.OrderConfig{
			CloseAfter: 15 * time.Minute,
		},
	}

	programRPC := &fakeOrderProgramRPC{
		releaseSeatFreezeResp: &programrpc.ReleaseSeatFreezeResp{Success: true},
	}
	userRPC := &fakeOrderUserRPC{}
	conn := sqlx.NewMysql(cfg.MySQL.DataSource)
	svcCtx := &svc.ServiceContext{
		Config:                cfg,
		SqlConn:               conn,
		DOrderModel:           model.NewDOrderModel(conn),
		DOrderTicketUserModel: model.NewDOrderTicketUserModel(conn),
		ProgramRpc:            programRPC,
		UserRpc:               userRPC,
	}

	return svcCtx, programRPC, userRPC
}

func resetOrderDomainState(t *testing.T) {
	t.Helper()

	db := openOrderTestDB(t, testOrderMySQLDataSource)
	defer db.Close()

	for _, relativePath := range []string{
		"sql/order/d_order.sql",
		"sql/order/d_order_ticket_user.sql",
	} {
		execOrderSQLFile(t, db, relativePath)
	}
}

func seedOrderFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...orderFixture) {
	t.Helper()

	db := openOrderTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withOrderFixtureDefaults(fixture)
		mustExecOrderSQL(
			t,
			db,
			`INSERT INTO d_order (
				id, order_number, program_id, program_title, program_item_picture, program_place, program_show_time,
				program_permit_choose_seat, user_id, distribution_mode, take_ticket_mode, ticket_count, order_price,
				order_status, freeze_token, order_expire_time, create_order_time, cancel_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.ProgramID,
			fixture.ProgramTitle,
			fixture.ProgramItemPicture,
			fixture.ProgramPlace,
			fixture.ProgramShowTime,
			fixture.ProgramPermitChooseSeat,
			fixture.UserID,
			fixture.DistributionMode,
			fixture.TakeTicketMode,
			fixture.TicketCount,
			fixture.OrderPrice,
			fixture.OrderStatus,
			fixture.FreezeToken,
			fixture.OrderExpireTime,
			fixture.CreateOrderTime,
			nullTimeIfEmpty(fixture.CancelOrderTime),
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			1,
		)
	}
}

func seedOrderTicketUserFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...orderTicketUserFixture) {
	t.Helper()

	db := openOrderTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withOrderTicketUserFixtureDefaults(fixture)
		mustExecOrderSQL(
			t,
			db,
			`INSERT INTO d_order_ticket_user (
				id, order_number, user_id, ticket_user_id, ticket_user_name, ticket_user_id_number,
				ticket_category_id, ticket_category_name, ticket_price, seat_id, seat_row, seat_col,
				seat_price, order_status, create_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.UserID,
			fixture.TicketUserID,
			fixture.TicketUserName,
			fixture.TicketUserIDNumber,
			fixture.TicketCategoryID,
			fixture.TicketCategoryName,
			fixture.TicketPrice,
			fixture.SeatID,
			fixture.SeatRow,
			fixture.SeatCol,
			fixture.SeatPrice,
			fixture.OrderStatus,
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			1,
		)
	}
}

func countRows(t *testing.T, dataSource, table string) int64 {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var count int64
	if err := db.QueryRow("SELECT COUNT(1) FROM " + table).Scan(&count); err != nil {
		t.Fatalf("QueryRow count error: %v", err)
	}

	return count
}

func findOrderStatus(t *testing.T, dataSource string, orderNumber int64) int64 {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var status int64
	if err := db.QueryRow("SELECT order_status FROM d_order WHERE order_number = ?", orderNumber).Scan(&status); err != nil {
		t.Fatalf("QueryRow order status error: %v", err)
	}

	return status
}

func findOrderTicketStatus(t *testing.T, dataSource string, orderNumber int64) int64 {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var status int64
	if err := db.QueryRow("SELECT order_status FROM d_order_ticket_user WHERE order_number = ? ORDER BY id ASC LIMIT 1", orderNumber).Scan(&status); err != nil {
		t.Fatalf("QueryRow order ticket status error: %v", err)
	}

	return status
}

func openOrderTestDB(t *testing.T, dataSource string) *sql.DB {
	t.Helper()

	db, err := sql.Open("mysql", dataSource)
	if err != nil {
		t.Fatalf("sql.Open error: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("db.Ping error: %v", err)
	}

	return db
}

func execOrderSQLFile(t *testing.T, db *sql.DB, relativePath string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(orderProjectRoot(t), relativePath))
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

func mustExecOrderSQL(t *testing.T, db *sql.DB, stmt string, args ...interface{}) {
	t.Helper()

	if _, err := db.Exec(stmt, args...); err != nil {
		t.Fatalf("db.Exec error: %v\nstatement: %s", err, stmt)
	}
}

func withOrderFixtureDefaults(fixture orderFixture) orderFixture {
	if fixture.ProgramTitle == "" {
		fixture.ProgramTitle = "订单测试演出"
	}
	if fixture.ProgramItemPicture == "" {
		fixture.ProgramItemPicture = "https://example.com/order-program.jpg"
	}
	if fixture.ProgramPlace == "" {
		fixture.ProgramPlace = "测试场馆"
	}
	if fixture.ProgramShowTime == "" {
		fixture.ProgramShowTime = "2026-12-31 19:30:00"
	}
	if fixture.ProgramPermitChooseSeat == 0 {
		fixture.ProgramPermitChooseSeat = 0
	}
	if fixture.DistributionMode == "" {
		fixture.DistributionMode = "express"
	}
	if fixture.TakeTicketMode == "" {
		fixture.TakeTicketMode = "paper"
	}
	if fixture.TicketCount == 0 {
		fixture.TicketCount = 1
	}
	if fixture.OrderPrice == 0 {
		fixture.OrderPrice = 299
	}
	if fixture.OrderStatus == 0 {
		fixture.OrderStatus = testOrderStatusUnpaid
	}
	if fixture.FreezeToken == "" {
		fixture.FreezeToken = "freeze-seed"
	}
	if fixture.OrderExpireTime == "" {
		fixture.OrderExpireTime = "2026-12-31 20:00:00"
	}
	if fixture.CreateOrderTime == "" {
		fixture.CreateOrderTime = "2026-01-01 00:00:00"
	}

	return fixture
}

func withOrderTicketUserFixtureDefaults(fixture orderTicketUserFixture) orderTicketUserFixture {
	if fixture.TicketUserName == "" {
		fixture.TicketUserName = "张三"
	}
	if fixture.TicketUserIDNumber == "" {
		fixture.TicketUserIDNumber = "110101199001011234"
	}
	if fixture.TicketCategoryName == "" {
		fixture.TicketCategoryName = "普通票"
	}
	if fixture.TicketPrice == 0 {
		fixture.TicketPrice = 299
	}
	if fixture.SeatPrice == 0 {
		fixture.SeatPrice = 299
	}
	if fixture.OrderStatus == 0 {
		fixture.OrderStatus = testOrderStatusUnpaid
	}
	if fixture.CreateOrderTime == "" {
		fixture.CreateOrderTime = "2026-01-01 00:00:00"
	}

	return fixture
}

func nullTimeIfEmpty(value string) interface{} {
	if value == "" {
		return nil
	}

	return value
}

func orderProjectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func buildTestProgramPreorder(programID, ticketCategoryID, perOrderLimit, perAccountLimit, ticketPrice int64) *programrpc.ProgramPreorderInfo {
	return &programrpc.ProgramPreorderInfo{
		Id:                           programID,
		ProgramGroupId:               programID + 1000,
		Title:                        "订单测试演出",
		Place:                        "测试场馆",
		ItemPicture:                  "https://example.com/order-program.jpg",
		ShowTime:                     "2026-12-31 19:30:00",
		ShowDayTime:                  "2026-12-31 00:00:00",
		ShowWeekTime:                 "周四",
		PerOrderLimitPurchaseCount:   perOrderLimit,
		PerAccountLimitPurchaseCount: perAccountLimit,
		PermitChooseSeat:             0,
		TicketCategoryVoList: []*programrpc.ProgramPreorderTicketCategoryInfo{
			{
				Id:           ticketCategoryID,
				Introduce:    "普通票",
				Price:        ticketPrice,
				TotalNumber:  100,
				RemainNumber: 100,
			},
		},
	}
}

func buildTestUserAndTicketUsers(userID int64, ticketUsers ...*userrpc.TicketUserInfo) *userrpc.GetUserAndTicketUserListResp {
	return &userrpc.GetUserAndTicketUserListResp{
		UserVo:           &userrpc.UserInfo{Id: userID, Mobile: "13800000000"},
		TicketUserVoList: ticketUsers,
	}
}

func (f *fakeOrderProgramRPC) ListProgramCategories(ctx context.Context, in *programrpc.Empty, opts ...grpc.CallOption) (*programrpc.ProgramCategoryListResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) ListHomePrograms(ctx context.Context, in *programrpc.ListHomeProgramsReq, opts ...grpc.CallOption) (*programrpc.ProgramHomeListResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) PagePrograms(ctx context.Context, in *programrpc.PageProgramsReq, opts ...grpc.CallOption) (*programrpc.ProgramPageResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) GetProgramDetail(ctx context.Context, in *programrpc.GetProgramDetailReq, opts ...grpc.CallOption) (*programrpc.ProgramDetailInfo, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) GetProgramPreorder(ctx context.Context, in *programrpc.GetProgramDetailReq, opts ...grpc.CallOption) (*programrpc.ProgramPreorderInfo, error) {
	f.lastGetProgramPreorderReq = in
	return f.getProgramPreorderResp, f.getProgramPreorderErr
}

func (f *fakeOrderProgramRPC) ListTicketCategoriesByProgram(ctx context.Context, in *programrpc.ListTicketCategoriesByProgramReq, opts ...grpc.CallOption) (*programrpc.TicketCategoryDetailListResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) AutoAssignAndFreezeSeats(ctx context.Context, in *programrpc.AutoAssignAndFreezeSeatsReq, opts ...grpc.CallOption) (*programrpc.AutoAssignAndFreezeSeatsResp, error) {
	f.lastAutoAssignAndFreezeSeatsReq = in
	return f.autoAssignAndFreezeSeatsResp, f.autoAssignAndFreezeSeatsErr
}

func (f *fakeOrderProgramRPC) ReleaseSeatFreeze(ctx context.Context, in *programrpc.ReleaseSeatFreezeReq, opts ...grpc.CallOption) (*programrpc.ReleaseSeatFreezeResp, error) {
	f.lastReleaseSeatFreezeReq = in
	f.releaseSeatFreezeCalls++
	return f.releaseSeatFreezeResp, f.releaseSeatFreezeErr
}

func (f *fakeOrderUserRPC) Register(ctx context.Context, in *userrpc.RegisterReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) Exist(ctx context.Context, in *userrpc.ExistReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) Login(ctx context.Context, in *userrpc.LoginReq, opts ...grpc.CallOption) (*userrpc.LoginResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) GetUserById(ctx context.Context, in *userrpc.GetUserByIdReq, opts ...grpc.CallOption) (*userrpc.UserInfo, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) GetUserByMobile(ctx context.Context, in *userrpc.GetUserByMobileReq, opts ...grpc.CallOption) (*userrpc.UserInfo, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) Logout(ctx context.Context, in *userrpc.LogoutReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) UpdateUser(ctx context.Context, in *userrpc.UpdateUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) UpdatePassword(ctx context.Context, in *userrpc.UpdatePasswordReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) UpdateEmail(ctx context.Context, in *userrpc.UpdateEmailReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) UpdateMobile(ctx context.Context, in *userrpc.UpdateMobileReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) Authentication(ctx context.Context, in *userrpc.AuthenticationReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) ListTicketUsers(ctx context.Context, in *userrpc.ListTicketUsersReq, opts ...grpc.CallOption) (*userrpc.ListTicketUsersResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) AddTicketUser(ctx context.Context, in *userrpc.AddTicketUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) DeleteTicketUser(ctx context.Context, in *userrpc.DeleteTicketUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) GetUserAndTicketUserList(ctx context.Context, in *userrpc.GetUserAndTicketUserListReq, opts ...grpc.CallOption) (*userrpc.GetUserAndTicketUserListResp, error) {
	f.lastGetUserAndTicketUserListReq = in
	return f.getUserAndTicketUserListResp, f.getUserAndTicketUserListErr
}

var _ programrpc.ProgramRpc = (*fakeOrderProgramRPC)(nil)
var _ userrpc.UserRpc = (*fakeOrderUserRPC)(nil)
var _ = pb.BoolResp{}
