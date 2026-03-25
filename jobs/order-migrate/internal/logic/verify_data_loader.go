package logic

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	verifyOrderScanBatchSize        int64 = 500
	verifyOrderNumberQueryBatchSize       = 200
)

type verifySlotData struct {
	orders  []*orderRow
	tickets []*orderTicketRow
	indexes []*userOrderIndexRow
}

type verifyShardTarget struct {
	conn        sqlx.SqlConn
	orderTable  string
	ticketTable string
	indexTable  string
	slots       []int
}

func loadLegacyVerifySlotData(ctx context.Context, conn sqlx.SqlConn, logicSlots []int, afterID, batchSize int64) (map[int]*verifySlotData, int64, bool, error) {
	datasets := newVerifySlotDataMap(logicSlots)
	if batchSize <= 0 {
		batchSize = verifyOrderScanBatchSize
	}

	var (
		cursor       = afterID
		matchedCount int64
		completed    bool
	)
	for {
		rows, err := listOrdersByTableAfter(ctx, conn, "d_order", cursor, batchSize)
		if err != nil {
			return nil, 0, false, err
		}
		if len(rows) == 0 {
			completed = true
			break
		}

		for _, row := range rows {
			cursor = row.Id
			logicSlot, err := logicSlotForOrder(row)
			if err != nil {
				return nil, 0, false, err
			}
			dataset, ok := datasets[logicSlot]
			if !ok {
				continue
			}
			dataset.orders = append(dataset.orders, row)
			matchedCount++
		}

		if int64(len(rows)) < batchSize {
			completed = true
			break
		}
		if matchedCount >= batchSize {
			break
		}
	}
	if err := populateVerifyChildRows(ctx, conn, "d_order_ticket_user", "d_user_order_index", datasets); err != nil {
		return nil, 0, false, err
	}
	return datasets, cursor, completed, nil
}

func loadShardVerifySlotData(ctx context.Context, shardConns map[string]sqlx.SqlConn, routes []sharding.Route, afterID, scanEnd int64) (map[int]*verifySlotData, error) {
	logicSlots := make([]int, 0, len(routes))
	for _, route := range routes {
		logicSlots = append(logicSlots, route.LogicSlot)
	}
	datasets := newVerifySlotDataMap(logicSlots)
	if scanEnd <= afterID {
		return datasets, nil
	}

	targets, err := buildVerifyShardTargets(shardConns, routes)
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		subset := selectVerifySlotData(datasets, target.slots)
		if err := collectVerifyOrdersByTableRange(ctx, target.conn, target.orderTable, afterID, scanEnd, subset); err != nil {
			return nil, err
		}
		if err := populateVerifyChildRows(ctx, target.conn, target.ticketTable, target.indexTable, subset); err != nil {
			return nil, err
		}
	}

	return datasets, nil
}

func buildVerifyShardTargets(shardConns map[string]sqlx.SqlConn, routes []sharding.Route) ([]verifyShardTarget, error) {
	targetByKey := make(map[string]*verifyShardTarget)
	for _, route := range routes {
		conn, ok := shardConns[route.DBKey]
		if !ok {
			return nil, fmt.Errorf("shard db key not configured: %s", route.DBKey)
		}

		key := route.DBKey + "/" + route.TableSuffix
		target, ok := targetByKey[key]
		if !ok {
			target = &verifyShardTarget{
				conn:        conn,
				orderTable:  "d_order_" + route.TableSuffix,
				ticketTable: "d_order_ticket_user_" + route.TableSuffix,
				indexTable:  "d_user_order_index_" + route.TableSuffix,
			}
			targetByKey[key] = target
		}
		target.slots = append(target.slots, route.LogicSlot)
	}

	keys := make([]string, 0, len(targetByKey))
	for key := range targetByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	targets := make([]verifyShardTarget, 0, len(keys))
	for _, key := range keys {
		targets = append(targets, *targetByKey[key])
	}
	return targets, nil
}

func newVerifySlotDataMap(logicSlots []int) map[int]*verifySlotData {
	datasets := make(map[int]*verifySlotData, len(logicSlots))
	for _, logicSlot := range logicSlots {
		if _, ok := datasets[logicSlot]; ok {
			continue
		}
		datasets[logicSlot] = &verifySlotData{}
	}
	return datasets
}

func selectVerifySlotData(datasets map[int]*verifySlotData, slots []int) map[int]*verifySlotData {
	selected := make(map[int]*verifySlotData, len(slots))
	for _, logicSlot := range slots {
		if dataset, ok := datasets[logicSlot]; ok {
			selected[logicSlot] = dataset
		}
	}
	return selected
}

func collectVerifyOrdersByTableRange(ctx context.Context, conn sqlx.SqlConn, table string, afterID, scanEnd int64, datasets map[int]*verifySlotData) error {
	rows, err := listOrdersByTableRange(ctx, conn, table, afterID, scanEnd)
	if err != nil {
		return err
	}
	for _, row := range rows {
		logicSlot, err := logicSlotForOrder(row)
		if err != nil {
			return err
		}
		dataset, ok := datasets[logicSlot]
		if !ok {
			continue
		}
		dataset.orders = append(dataset.orders, row)
	}
	return nil
}

func populateVerifyChildRows(ctx context.Context, conn sqlx.SqlConn, ticketTable, indexTable string, datasets map[int]*verifySlotData) error {
	orderNumbers, slotByOrderNumber := collectVerifyOrderNumbers(datasets)
	if len(orderNumbers) == 0 {
		return nil
	}

	ticketRows, err := listTicketsByOrderNumbers(ctx, conn, ticketTable, orderNumbers)
	if err != nil {
		return err
	}
	for _, row := range ticketRows {
		dataset, ok := datasets[slotByOrderNumber[row.OrderNumber]]
		if !ok {
			continue
		}
		dataset.tickets = append(dataset.tickets, row)
	}

	indexRows, err := listUserOrderIndexesByOrderNumbers(ctx, conn, indexTable, orderNumbers)
	if err != nil {
		return err
	}
	for _, row := range indexRows {
		dataset, ok := datasets[slotByOrderNumber[row.OrderNumber]]
		if !ok {
			continue
		}
		dataset.indexes = append(dataset.indexes, row)
	}

	return nil
}

func collectVerifyOrderNumbers(datasets map[int]*verifySlotData) ([]int64, map[int64]int) {
	orderNumberSet := make(map[int64]int)
	orderNumbers := make([]int64, 0)
	for logicSlot, dataset := range datasets {
		if dataset == nil {
			continue
		}
		for _, order := range dataset.orders {
			if order == nil {
				continue
			}
			if _, ok := orderNumberSet[order.OrderNumber]; ok {
				continue
			}
			orderNumberSet[order.OrderNumber] = logicSlot
			orderNumbers = append(orderNumbers, order.OrderNumber)
		}
	}
	sort.Slice(orderNumbers, func(i, j int) bool { return orderNumbers[i] < orderNumbers[j] })
	return orderNumbers, orderNumberSet
}

func listOrdersByTableAfter(ctx context.Context, conn sqlx.SqlConn, table string, afterID, limit int64) ([]*orderRow, error) {
	query := fmt.Sprintf("select * from %s where `status` = 1 and `id` > ? order by `id` asc limit ?", table)
	var rows []*orderRow
	err := conn.QueryRowsCtx(ctx, &rows, query, afterID, limit)
	switch {
	case err == nil:
		return rows, nil
	case err == sqlx.ErrNotFound:
		return []*orderRow{}, nil
	default:
		return nil, err
	}
}

func listOrdersByTableRange(ctx context.Context, conn sqlx.SqlConn, table string, afterID, scanEnd int64) ([]*orderRow, error) {
	query := fmt.Sprintf("select * from %s where `status` = 1 and `id` > ? and `id` <= ? order by `id` asc", table)
	var rows []*orderRow
	err := conn.QueryRowsCtx(ctx, &rows, query, afterID, scanEnd)
	switch {
	case err == nil:
		return rows, nil
	case err == sqlx.ErrNotFound:
		return []*orderRow{}, nil
	default:
		return nil, err
	}
}

func listTicketsByOrderNumbers(ctx context.Context, conn sqlx.SqlConn, table string, orderNumbers []int64) ([]*orderTicketRow, error) {
	rows := make([]*orderTicketRow, 0)
	for _, chunk := range chunkOrderNumbers(orderNumbers, verifyOrderNumberQueryBatchSize) {
		query, args := buildOrderNumberInQuery(
			fmt.Sprintf("select * from %s where `status` = 1 and `order_number` in (%%s) order by `order_number` asc, `id` asc", table),
			chunk,
		)
		var chunkRows []*orderTicketRow
		err := conn.QueryRowsCtx(ctx, &chunkRows, query, args...)
		switch {
		case err == nil:
			rows = append(rows, chunkRows...)
		case err == sqlx.ErrNotFound:
			continue
		default:
			return nil, err
		}
	}
	return rows, nil
}

func listUserOrderIndexesByOrderNumbers(ctx context.Context, conn sqlx.SqlConn, table string, orderNumbers []int64) ([]*userOrderIndexRow, error) {
	rows := make([]*userOrderIndexRow, 0)
	for _, chunk := range chunkOrderNumbers(orderNumbers, verifyOrderNumberQueryBatchSize) {
		query, args := buildOrderNumberInQuery(
			fmt.Sprintf("select * from %s where `status` = 1 and `order_number` in (%%s) order by `order_number` asc, `id` asc", table),
			chunk,
		)
		var chunkRows []*userOrderIndexRow
		err := conn.QueryRowsCtx(ctx, &chunkRows, query, args...)
		switch {
		case err == nil:
			rows = append(rows, chunkRows...)
		case err == sqlx.ErrNotFound:
			continue
		default:
			return nil, err
		}
	}
	return rows, nil
}

func chunkOrderNumbers(orderNumbers []int64, chunkSize int) [][]int64 {
	if chunkSize <= 0 || len(orderNumbers) == 0 {
		return nil
	}

	chunks := make([][]int64, 0, (len(orderNumbers)+chunkSize-1)/chunkSize)
	for start := 0; start < len(orderNumbers); start += chunkSize {
		end := start + chunkSize
		if end > len(orderNumbers) {
			end = len(orderNumbers)
		}
		chunks = append(chunks, orderNumbers[start:end])
	}
	return chunks
}

func buildOrderNumberInQuery(template string, orderNumbers []int64) (string, []any) {
	placeholders := make([]string, 0, len(orderNumbers))
	args := make([]any, 0, len(orderNumbers))
	for _, orderNumber := range orderNumbers {
		placeholders = append(placeholders, "?")
		args = append(args, orderNumber)
	}
	return fmt.Sprintf(template, strings.Join(placeholders, ",")), args
}
