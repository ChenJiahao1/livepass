package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"damai-go/jobs/order-migrate/internal/svc"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type orderRow struct {
	Id                      int64        `db:"id"`
	OrderNumber             int64        `db:"order_number"`
	ProgramId               int64        `db:"program_id"`
	ProgramTitle            string       `db:"program_title"`
	ProgramItemPicture      string       `db:"program_item_picture"`
	ProgramPlace            string       `db:"program_place"`
	ProgramShowTime         time.Time    `db:"program_show_time"`
	ProgramPermitChooseSeat int64        `db:"program_permit_choose_seat"`
	UserId                  int64        `db:"user_id"`
	DistributionMode        string       `db:"distribution_mode"`
	TakeTicketMode          string       `db:"take_ticket_mode"`
	TicketCount             int64        `db:"ticket_count"`
	OrderPrice              int64        `db:"order_price"`
	OrderStatus             int64        `db:"order_status"`
	FreezeToken             string       `db:"freeze_token"`
	OrderExpireTime         time.Time    `db:"order_expire_time"`
	CreateOrderTime         time.Time    `db:"create_order_time"`
	CancelOrderTime         sql.NullTime `db:"cancel_order_time"`
	PayOrderTime            sql.NullTime `db:"pay_order_time"`
	CreateTime              time.Time    `db:"create_time"`
	EditTime                time.Time    `db:"edit_time"`
	Status                  int64        `db:"status"`
}

type orderTicketRow struct {
	Id                 int64     `db:"id"`
	OrderNumber        int64     `db:"order_number"`
	UserId             int64     `db:"user_id"`
	TicketUserId       int64     `db:"ticket_user_id"`
	TicketUserName     string    `db:"ticket_user_name"`
	TicketUserIdNumber string    `db:"ticket_user_id_number"`
	TicketCategoryId   int64     `db:"ticket_category_id"`
	TicketCategoryName string    `db:"ticket_category_name"`
	TicketPrice        int64     `db:"ticket_price"`
	SeatId             int64     `db:"seat_id"`
	SeatRow            int64     `db:"seat_row"`
	SeatCol            int64     `db:"seat_col"`
	SeatPrice          int64     `db:"seat_price"`
	OrderStatus        int64     `db:"order_status"`
	CreateOrderTime    time.Time `db:"create_order_time"`
	CreateTime         time.Time `db:"create_time"`
	EditTime           time.Time `db:"edit_time"`
	Status             int64     `db:"status"`
}

type userOrderIndexRow struct {
	Id              int64     `db:"id"`
	OrderNumber     int64     `db:"order_number"`
	UserId          int64     `db:"user_id"`
	ProgramId       int64     `db:"program_id"`
	OrderStatus     int64     `db:"order_status"`
	TicketCount     int64     `db:"ticket_count"`
	OrderPrice      int64     `db:"order_price"`
	CreateOrderTime time.Time `db:"create_order_time"`
	CreateTime      time.Time `db:"create_time"`
	EditTime        time.Time `db:"edit_time"`
	Status          int64     `db:"status"`
}

type backfillCheckpoint struct {
	LastOrderID int64 `json:"last_order_id"`
}

type verifyAggregate struct {
	Count        int64
	OrderPrice   int64
	StatusCounts map[int64]int64
}

func loadBackfillCheckpoint(path string) (backfillCheckpoint, error) {
	if path == "" {
		return backfillCheckpoint{}, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return backfillCheckpoint{}, nil
		}
		return backfillCheckpoint{}, err
	}

	var checkpoint backfillCheckpoint
	if err := json.Unmarshal(content, &checkpoint); err != nil {
		return backfillCheckpoint{}, err
	}
	return checkpoint, nil
}

func saveBackfillCheckpoint(path string, checkpoint backfillCheckpoint) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func listLegacyOrdersAfter(ctx context.Context, conn sqlx.SqlConn, afterID, limit int64) ([]*orderRow, error) {
	query := "select * from d_order where `status` = 1 and `id` > ? order by `id` asc limit ?"
	var rows []*orderRow
	err := conn.QueryRowsCtx(ctx, &rows, query, afterID, limit)
	switch {
	case err == nil:
		return rows, nil
	case errors.Is(err, sqlx.ErrNotFound):
		return []*orderRow{}, nil
	default:
		return nil, err
	}
}

func listOrderTicketsByNumber(ctx context.Context, conn sqlx.SqlConn, table string, orderNumber int64) ([]*orderTicketRow, error) {
	query := fmt.Sprintf("select * from %s where `status` = 1 and `order_number` = ? order by `id` asc", table)
	var rows []*orderTicketRow
	err := conn.QueryRowsCtx(ctx, &rows, query, orderNumber)
	switch {
	case err == nil:
		return rows, nil
	case errors.Is(err, sqlx.ErrNotFound):
		return []*orderTicketRow{}, nil
	default:
		return nil, err
	}
}

func listTicketsByTable(ctx context.Context, conn sqlx.SqlConn, table string) ([]*orderTicketRow, error) {
	query := fmt.Sprintf("select * from %s where `status` = 1 order by `order_number` asc, `id` asc", table)
	var rows []*orderTicketRow
	err := conn.QueryRowsCtx(ctx, &rows, query)
	switch {
	case err == nil:
		return rows, nil
	case errors.Is(err, sqlx.ErrNotFound):
		return []*orderTicketRow{}, nil
	default:
		return nil, err
	}
}

func listOrdersByTable(ctx context.Context, conn sqlx.SqlConn, table string) ([]*orderRow, error) {
	query := fmt.Sprintf("select * from %s where `status` = 1 order by `create_order_time` asc, `id` asc", table)
	var rows []*orderRow
	err := conn.QueryRowsCtx(ctx, &rows, query)
	switch {
	case err == nil:
		return rows, nil
	case errors.Is(err, sqlx.ErrNotFound):
		return []*orderRow{}, nil
	default:
		return nil, err
	}
}

func listUserOrderIndexesByTable(ctx context.Context, conn sqlx.SqlConn, table string) ([]*userOrderIndexRow, error) {
	query := fmt.Sprintf("select * from %s where `status` = 1 order by `order_number` asc, `id` asc", table)
	var rows []*userOrderIndexRow
	err := conn.QueryRowsCtx(ctx, &rows, query)
	switch {
	case err == nil:
		return rows, nil
	case errors.Is(err, sqlx.ErrNotFound):
		return []*userOrderIndexRow{}, nil
	default:
		return nil, err
	}
}

func logicSlotForOrder(order *orderRow) (int, error) {
	if order == nil {
		return 0, sharding.ErrInvalidOrderNumber
	}
	slot, err := sharding.LogicSlotByOrderNumber(order.OrderNumber)
	if err == nil {
		return slot, nil
	}
	if errors.Is(err, sharding.ErrLegacyOrderRequiresDirectoryLookup) {
		return sharding.LogicSlotByUserID(order.UserId), nil
	}
	return 0, err
}

func filterOrdersBySlot(orders []*orderRow, logicSlot int) []*orderRow {
	filtered := make([]*orderRow, 0, len(orders))
	for _, order := range orders {
		slot, err := logicSlotForOrder(order)
		if err != nil || slot != logicSlot {
			continue
		}
		filtered = append(filtered, order)
	}
	return filtered
}

func buildVerifyAggregate(orders []*orderRow) verifyAggregate {
	aggregate := verifyAggregate{
		StatusCounts: make(map[int64]int64),
	}
	for _, order := range orders {
		if order == nil {
			continue
		}
		aggregate.Count++
		aggregate.OrderPrice += order.OrderPrice
		aggregate.StatusCounts[order.OrderStatus]++
	}
	return aggregate
}

func compareAggregates(logicSlot int, legacy, shard verifyAggregate) error {
	if legacy.Count != shard.Count {
		return fmt.Errorf("count mismatch for slot %d: legacy=%d shard=%d", logicSlot, legacy.Count, shard.Count)
	}
	if legacy.OrderPrice != shard.OrderPrice {
		return fmt.Errorf("sum mismatch for slot %d: legacy=%d shard=%d", logicSlot, legacy.OrderPrice, shard.OrderPrice)
	}
	if len(legacy.StatusCounts) != len(shard.StatusCounts) {
		return fmt.Errorf("status distribution mismatch for slot %d", logicSlot)
	}
	for status, count := range legacy.StatusCounts {
		if shard.StatusCounts[status] != count {
			return fmt.Errorf("status distribution mismatch for slot %d status=%d legacy=%d shard=%d", logicSlot, status, count, shard.StatusCounts[status])
		}
	}
	return nil
}

func compareOrderSamples(sampleSize int64, legacyOrders, shardOrders []*orderRow) error {
	if sampleSize <= 0 {
		return nil
	}
	legacyByNumber := make(map[int64]*orderRow, len(legacyOrders))
	for _, order := range legacyOrders {
		if order != nil {
			legacyByNumber[order.OrderNumber] = order
		}
	}
	shardByNumber := make(map[int64]*orderRow, len(shardOrders))
	for _, order := range shardOrders {
		if order != nil {
			shardByNumber[order.OrderNumber] = order
		}
	}

	orderNumbers := make([]int64, 0, len(legacyByNumber))
	for orderNumber := range legacyByNumber {
		orderNumbers = append(orderNumbers, orderNumber)
	}
	sort.Slice(orderNumbers, func(i, j int) bool { return orderNumbers[i] < orderNumbers[j] })
	if int64(len(orderNumbers)) > sampleSize {
		orderNumbers = orderNumbers[:sampleSize]
	}

	for _, orderNumber := range orderNumbers {
		legacyOrder := legacyByNumber[orderNumber]
		shardOrder := shardByNumber[orderNumber]
		if shardOrder == nil {
			return fmt.Errorf("sample detail missing order %d", orderNumber)
		}
		if legacyOrder.OrderStatus != shardOrder.OrderStatus || legacyOrder.OrderPrice != shardOrder.OrderPrice || legacyOrder.UserId != shardOrder.UserId {
			return fmt.Errorf("sample detail mismatch for order %d", orderNumber)
		}
	}

	return nil
}

func compareUserListSamples(sampleSize int64, legacyOrders, shardOrders []*orderRow) error {
	if sampleSize <= 0 {
		return nil
	}

	legacyByUser := map[int64]int64{}
	for _, order := range legacyOrders {
		if order != nil {
			legacyByUser[order.UserId]++
		}
	}
	shardByUser := map[int64]int64{}
	for _, order := range shardOrders {
		if order != nil {
			shardByUser[order.UserId]++
		}
	}

	userIDs := make([]int64, 0, len(legacyByUser))
	for userID := range legacyByUser {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	if int64(len(userIDs)) > sampleSize {
		userIDs = userIDs[:sampleSize]
	}

	for _, userID := range userIDs {
		if shardByUser[userID] != legacyByUser[userID] {
			return fmt.Errorf("sample list mismatch for user %d", userID)
		}
	}

	return nil
}

func collectOrderNumbers(orders ...[]*orderRow) map[int64]struct{} {
	orderNumbers := make(map[int64]struct{})
	for _, rows := range orders {
		for _, order := range rows {
			if order == nil {
				continue
			}
			orderNumbers[order.OrderNumber] = struct{}{}
		}
	}
	return orderNumbers
}

func filterTicketsByOrderNumbers(rows []*orderTicketRow, orderNumbers map[int64]struct{}) []*orderTicketRow {
	if len(orderNumbers) == 0 {
		return []*orderTicketRow{}
	}

	filtered := make([]*orderTicketRow, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		if _, ok := orderNumbers[row.OrderNumber]; !ok {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func filterUserOrderIndexesByOrderNumbers(rows []*userOrderIndexRow, orderNumbers map[int64]struct{}) []*userOrderIndexRow {
	if len(orderNumbers) == 0 {
		return []*userOrderIndexRow{}
	}

	filtered := make([]*userOrderIndexRow, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		if _, ok := orderNumbers[row.OrderNumber]; !ok {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func compareTicketSnapshots(legacyRows, shardRows []*orderTicketRow) error {
	if len(legacyRows) != len(shardRows) {
		return fmt.Errorf("ticket snapshot mismatch: row count legacy=%d shard=%d", len(legacyRows), len(shardRows))
	}

	for i := range legacyRows {
		legacyRow := legacyRows[i]
		shardRow := shardRows[i]
		if !ticketSnapshotEqual(legacyRow, shardRow) {
			return fmt.Errorf(
				"ticket snapshot mismatch: order=%d legacy(ticket_user=%d seat=%d status=%d) shard(ticket_user=%d seat=%d status=%d)",
				snapshotOrderNumber(legacyRow, shardRow),
				snapshotTicketUserID(legacyRow),
				snapshotSeatID(legacyRow),
				snapshotTicketStatus(legacyRow),
				snapshotTicketUserID(shardRow),
				snapshotSeatID(shardRow),
				snapshotTicketStatus(shardRow),
			)
		}
	}

	return nil
}

func compareUserOrderIndexSnapshots(legacyRows, shardRows []*userOrderIndexRow) error {
	if len(legacyRows) != len(shardRows) {
		return fmt.Errorf("user order index mismatch: row count legacy=%d shard=%d", len(legacyRows), len(shardRows))
	}

	for i := range legacyRows {
		legacyRow := legacyRows[i]
		shardRow := shardRows[i]
		if !userOrderIndexEqual(legacyRow, shardRow) {
			return fmt.Errorf(
				"user order index mismatch: order=%d legacy(status=%d price=%d tickets=%d) shard(status=%d price=%d tickets=%d)",
				snapshotUserIndexOrderNumber(legacyRow, shardRow),
				snapshotUserIndexStatus(legacyRow),
				snapshotUserIndexPrice(legacyRow),
				snapshotUserIndexTicketCount(legacyRow),
				snapshotUserIndexStatus(shardRow),
				snapshotUserIndexPrice(shardRow),
				snapshotUserIndexTicketCount(shardRow),
			)
		}
	}

	return nil
}

func ticketSnapshotEqual(legacyRow, shardRow *orderTicketRow) bool {
	if legacyRow == nil || shardRow == nil {
		return legacyRow == shardRow
	}
	return legacyRow.OrderNumber == shardRow.OrderNumber &&
		legacyRow.UserId == shardRow.UserId &&
		legacyRow.TicketUserId == shardRow.TicketUserId &&
		legacyRow.TicketCategoryId == shardRow.TicketCategoryId &&
		legacyRow.TicketPrice == shardRow.TicketPrice &&
		legacyRow.SeatId == shardRow.SeatId &&
		legacyRow.SeatRow == shardRow.SeatRow &&
		legacyRow.SeatCol == shardRow.SeatCol &&
		legacyRow.SeatPrice == shardRow.SeatPrice &&
		legacyRow.OrderStatus == shardRow.OrderStatus
}

func userOrderIndexEqual(legacyRow, shardRow *userOrderIndexRow) bool {
	if legacyRow == nil || shardRow == nil {
		return legacyRow == shardRow
	}
	return legacyRow.OrderNumber == shardRow.OrderNumber &&
		legacyRow.UserId == shardRow.UserId &&
		legacyRow.ProgramId == shardRow.ProgramId &&
		legacyRow.OrderStatus == shardRow.OrderStatus &&
		legacyRow.TicketCount == shardRow.TicketCount &&
		legacyRow.OrderPrice == shardRow.OrderPrice
}

func snapshotOrderNumber(legacyRow, shardRow *orderTicketRow) int64 {
	if legacyRow != nil {
		return legacyRow.OrderNumber
	}
	if shardRow != nil {
		return shardRow.OrderNumber
	}
	return 0
}

func snapshotTicketUserID(row *orderTicketRow) int64 {
	if row == nil {
		return 0
	}
	return row.TicketUserId
}

func snapshotSeatID(row *orderTicketRow) int64 {
	if row == nil {
		return 0
	}
	return row.SeatId
}

func snapshotTicketStatus(row *orderTicketRow) int64 {
	if row == nil {
		return 0
	}
	return row.OrderStatus
}

func snapshotUserIndexOrderNumber(legacyRow, shardRow *userOrderIndexRow) int64 {
	if legacyRow != nil {
		return legacyRow.OrderNumber
	}
	if shardRow != nil {
		return shardRow.OrderNumber
	}
	return 0
}

func snapshotUserIndexStatus(row *userOrderIndexRow) int64 {
	if row == nil {
		return 0
	}
	return row.OrderStatus
}

func snapshotUserIndexPrice(row *userOrderIndexRow) int64 {
	if row == nil {
		return 0
	}
	return row.OrderPrice
}

func snapshotUserIndexTicketCount(row *userOrderIndexRow) int64 {
	if row == nil {
		return 0
	}
	return row.TicketCount
}

func upsertOrderBundle(ctx context.Context, svcCtx *svc.ServiceContext, route sharding.Route, order *orderRow, tickets []*orderTicketRow) error {
	if svcCtx == nil {
		return fmt.Errorf("service context is nil")
	}
	shardConn, ok := svcCtx.ShardSqlConns[route.DBKey]
	if !ok {
		return fmt.Errorf("shard db key not configured: %s", route.DBKey)
	}

	rawDB, err := shardConn.RawDB()
	if err != nil {
		return err
	}
	tx, err := rawDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if err := upsertOrderTx(ctx, tx, "d_order_"+route.TableSuffix, order); err != nil {
		return err
	}
	for _, ticket := range tickets {
		if err := upsertTicketTx(ctx, tx, "d_order_ticket_user_"+route.TableSuffix, ticket); err != nil {
			return err
		}
	}
	if err := upsertUserOrderIndexTx(ctx, tx, "d_user_order_index_"+route.TableSuffix, order); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	tx = nil

	if _, err := sharding.LogicSlotByOrderNumber(order.OrderNumber); errors.Is(err, sharding.ErrLegacyOrderRequiresDirectoryLookup) {
		return upsertLegacyRoute(ctx, svcCtx.LegacySqlConn, order, route.LogicSlot, route.Version)
	}

	return nil
}

func upsertOrderTx(ctx context.Context, tx *sql.Tx, table string, order *orderRow) error {
	query := fmt.Sprintf(
		`INSERT INTO %s (
			id, order_number, program_id, program_title, program_item_picture, program_place, program_show_time,
			program_permit_choose_seat, user_id, distribution_mode, take_ticket_mode, ticket_count, order_price,
			order_status, freeze_token, order_expire_time, create_order_time, cancel_order_time, pay_order_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			program_id = VALUES(program_id),
			program_title = VALUES(program_title),
			program_item_picture = VALUES(program_item_picture),
			program_place = VALUES(program_place),
			program_show_time = VALUES(program_show_time),
			program_permit_choose_seat = VALUES(program_permit_choose_seat),
			user_id = VALUES(user_id),
			distribution_mode = VALUES(distribution_mode),
			take_ticket_mode = VALUES(take_ticket_mode),
			ticket_count = VALUES(ticket_count),
			order_price = VALUES(order_price),
			order_status = VALUES(order_status),
			freeze_token = VALUES(freeze_token),
			order_expire_time = VALUES(order_expire_time),
			create_order_time = VALUES(create_order_time),
			cancel_order_time = VALUES(cancel_order_time),
			pay_order_time = VALUES(pay_order_time),
			edit_time = VALUES(edit_time),
			status = VALUES(status)`,
		table,
	)
	_, err := tx.ExecContext(
		ctx,
		query,
		order.Id,
		order.OrderNumber,
		order.ProgramId,
		order.ProgramTitle,
		order.ProgramItemPicture,
		order.ProgramPlace,
		order.ProgramShowTime,
		order.ProgramPermitChooseSeat,
		order.UserId,
		order.DistributionMode,
		order.TakeTicketMode,
		order.TicketCount,
		order.OrderPrice,
		order.OrderStatus,
		order.FreezeToken,
		order.OrderExpireTime,
		order.CreateOrderTime,
		order.CancelOrderTime,
		order.PayOrderTime,
		order.CreateTime,
		order.EditTime,
		order.Status,
	)
	return err
}

func upsertTicketTx(ctx context.Context, tx *sql.Tx, table string, ticket *orderTicketRow) error {
	query := fmt.Sprintf(
		`INSERT INTO %s (
			id, order_number, user_id, ticket_user_id, ticket_user_name, ticket_user_id_number,
			ticket_category_id, ticket_category_name, ticket_price, seat_id, seat_row, seat_col,
			seat_price, order_status, create_order_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			user_id = VALUES(user_id),
			ticket_user_id = VALUES(ticket_user_id),
			ticket_user_name = VALUES(ticket_user_name),
			ticket_user_id_number = VALUES(ticket_user_id_number),
			ticket_category_id = VALUES(ticket_category_id),
			ticket_category_name = VALUES(ticket_category_name),
			ticket_price = VALUES(ticket_price),
			seat_id = VALUES(seat_id),
			seat_row = VALUES(seat_row),
			seat_col = VALUES(seat_col),
			seat_price = VALUES(seat_price),
			order_status = VALUES(order_status),
			edit_time = VALUES(edit_time),
			status = VALUES(status)`,
		table,
	)
	_, err := tx.ExecContext(
		ctx,
		query,
		ticket.Id,
		ticket.OrderNumber,
		ticket.UserId,
		ticket.TicketUserId,
		ticket.TicketUserName,
		ticket.TicketUserIdNumber,
		ticket.TicketCategoryId,
		ticket.TicketCategoryName,
		ticket.TicketPrice,
		ticket.SeatId,
		ticket.SeatRow,
		ticket.SeatCol,
		ticket.SeatPrice,
		ticket.OrderStatus,
		ticket.CreateOrderTime,
		ticket.CreateTime,
		ticket.EditTime,
		ticket.Status,
	)
	return err
}

func upsertUserOrderIndexTx(ctx context.Context, tx *sql.Tx, table string, order *orderRow) error {
	query := fmt.Sprintf(
		`INSERT INTO %s (
			id, order_number, user_id, program_id, order_status, ticket_count, order_price, create_order_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			user_id = VALUES(user_id),
			program_id = VALUES(program_id),
			order_status = VALUES(order_status),
			ticket_count = VALUES(ticket_count),
			order_price = VALUES(order_price),
			create_order_time = VALUES(create_order_time),
			edit_time = VALUES(edit_time),
			status = VALUES(status)`,
		table,
	)
	_, err := tx.ExecContext(
		ctx,
		query,
		order.Id,
		order.OrderNumber,
		order.UserId,
		order.ProgramId,
		order.OrderStatus,
		order.TicketCount,
		order.OrderPrice,
		order.CreateOrderTime,
		order.CreateTime,
		order.EditTime,
		order.Status,
	)
	return err
}

func upsertLegacyRoute(ctx context.Context, conn sqlx.SqlConn, order *orderRow, logicSlot int, version string) error {
	query := `INSERT INTO d_order_route_legacy (
		order_number, user_id, logic_slot, route_version, status, create_time, edit_time
	) VALUES (?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		user_id = VALUES(user_id),
		logic_slot = VALUES(logic_slot),
		route_version = VALUES(route_version),
		edit_time = VALUES(edit_time),
		status = VALUES(status)`
	_, err := conn.ExecCtx(ctx, query, order.OrderNumber, order.UserId, logicSlot, version, order.Status, order.CreateTime, order.EditTime)
	return err
}
