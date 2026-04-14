package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DProgramModel = (*customDProgramModel)(nil)

type (
	// DProgramModel is an interface to be customized, add more methods here,
	// and implement the added methods in customDProgramModel.
	DProgramModel interface {
		dProgramModel
		withSession(session sqlx.Session) DProgramModel
		InsertWithSession(ctx context.Context, session sqlx.Session, data *DProgram) (sql.Result, error)
		FindOneForUpdate(ctx context.Context, session sqlx.Session, id int64) (*DProgram, error)
		FindHomeList(ctx context.Context, q *ProgramHomeListQuery) ([]*DProgram, error)
		CountPageList(ctx context.Context, q *ProgramPageListQuery) (int64, error)
		FindPageList(ctx context.Context, q *ProgramPageListQuery) ([]*DProgram, error)
	}

	customDProgramModel struct {
		*defaultDProgramModel
	}

	ProgramHomeListQuery struct {
		AreaId                   int64
		ParentProgramCategoryIds []int64
		Limit                    int64
	}

	ProgramPageListQuery struct {
		PageNumber              int64
		PageSize                int64
		AreaId                  int64
		ParentProgramCategoryId int64
		ProgramCategoryId       int64
		TimeType                int64
		StartDateTime           string
		EndDateTime             string
		Type                    int64
	}
)

// NewDProgramModel returns a model for the database table.
func NewDProgramModel(conn sqlx.SqlConn) DProgramModel {
	return &customDProgramModel{
		defaultDProgramModel: newDProgramModel(conn),
	}
}

// NewCachedDProgramModel returns a cached model for the database table.
func NewCachedDProgramModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) DProgramModel {
	return &customDProgramModel{
		defaultDProgramModel: newCachedDProgramModel(conn, c, opts...),
	}
}

func (m *customDProgramModel) withSession(session sqlx.Session) DProgramModel {
	return NewDProgramModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customDProgramModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *DProgram) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`id`, `program_group_id`, `prime`, `area_id`, `program_category_id`, `parent_program_category_id`, `title`, `actor`, `place`, `item_picture`, `pre_sell`, `pre_sell_instruction`, `important_notice`, `detail`, `per_order_limit_purchase_count`, `per_account_limit_purchase_count`, `refund_ticket_rule`, `delivery_instruction`, `entry_rule`, `child_purchase`, `invoice_specification`, `real_ticket_purchase_rule`, `abnormal_order_description`, `kind_reminder`, `performance_duration`, `entry_time`, `min_performance_count`, `main_actor`, `min_performance_duration`, `prohibited_item`, `deposit_specification`, `total_count`, `permit_refund`, `refund_explain`, `refund_rule_json`, `rel_name_ticket_entrance`, `rel_name_ticket_entrance_explain`, `permit_choose_seat`, `choose_seat_explain`, `electronic_delivery_ticket`, `electronic_delivery_ticket_explain`, `electronic_invoice`, `electronic_invoice_explain`, `high_heat`, `program_status`, `issue_time`, `rush_sale_open_time`, `rush_sale_end_time`, `inventory_preheat_status`, `create_time`, `edit_time`, `status`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return m.withSession(session).(*customDProgramModel).conn.ExecCtx(
		ctx,
		query,
		data.Id,
		data.ProgramGroupId,
		data.Prime,
		data.AreaId,
		data.ProgramCategoryId,
		data.ParentProgramCategoryId,
		data.Title,
		data.Actor,
		data.Place,
		data.ItemPicture,
		data.PreSell,
		data.PreSellInstruction,
		data.ImportantNotice,
		data.Detail,
		data.PerOrderLimitPurchaseCount,
		data.PerAccountLimitPurchaseCount,
		data.RefundTicketRule,
		data.DeliveryInstruction,
		data.EntryRule,
		data.ChildPurchase,
		data.InvoiceSpecification,
		data.RealTicketPurchaseRule,
		data.AbnormalOrderDescription,
		data.KindReminder,
		data.PerformanceDuration,
		data.EntryTime,
		data.MinPerformanceCount,
		data.MainActor,
		data.MinPerformanceDuration,
		data.ProhibitedItem,
		data.DepositSpecification,
		data.TotalCount,
		data.PermitRefund,
		data.RefundExplain,
		data.RefundRuleJson,
		data.RelNameTicketEntrance,
		data.RelNameTicketEntranceExplain,
		data.PermitChooseSeat,
		data.ChooseSeatExplain,
		data.ElectronicDeliveryTicket,
		data.ElectronicDeliveryTicketExplain,
		data.ElectronicInvoice,
		data.ElectronicInvoiceExplain,
		data.HighHeat,
		data.ProgramStatus,
		data.IssueTime,
		data.RushSaleOpenTime,
		data.RushSaleEndTime,
		data.InventoryPreheatStatus,
		data.CreateTime,
		data.EditTime,
		data.Status,
	)
}

func (m *customDProgramModel) FindOneForUpdate(ctx context.Context, session sqlx.Session, id int64) (*DProgram, error) {
	query := fmt.Sprintf(
		"select %s from %s where `status` = 1 and `id` = ? limit 1 for update",
		dProgramRows,
		m.table,
	)

	var resp DProgram
	err := m.withSession(session).(*customDProgramModel).conn.QueryRowCtx(ctx, &resp, query, id)
	switch err {
	case nil:
		return &resp, nil
	case sql.ErrNoRows, sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customDProgramModel) FindHomeList(ctx context.Context, q *ProgramHomeListQuery) ([]*DProgram, error) {
	if q == nil {
		q = &ProgramHomeListQuery{}
	}

	conditions, args := buildProgramBaseConditions(q.AreaId)
	if len(q.ParentProgramCategoryIds) > 0 {
		inClause, inArgs := buildInt64InClause(q.ParentProgramCategoryIds)
		conditions = append(conditions, fmt.Sprintf("p.`parent_program_category_id` in (%s)", inClause))
		args = append(args, inArgs...)
	}

	query := fmt.Sprintf(
		"select %s from %s p where %s order by p.`high_heat` desc, p.`id` desc",
		dProgramRows,
		m.table,
		strings.Join(conditions, " and "),
	)

	if q.Limit > 0 {
		query += " limit ?"
		args = append(args, q.Limit)
	}

	var resp []*DProgram
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return resp, nil
}

func (m *customDProgramModel) CountPageList(ctx context.Context, q *ProgramPageListQuery) (int64, error) {
	if q == nil {
		q = &ProgramPageListQuery{}
	}

	conditions, args := buildProgramPageConditions(q)
	query := fmt.Sprintf(
		"select count(1) from %s p where %s",
		m.table,
		strings.Join(conditions, " and "),
	)

	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return 0, nil
		}
		return 0, err
	}

	return total, nil
}

func (m *customDProgramModel) FindPageList(ctx context.Context, q *ProgramPageListQuery) ([]*DProgram, error) {
	if q == nil {
		q = &ProgramPageListQuery{}
	}

	conditions, args := buildProgramPageConditions(q)
	orderBy := buildProgramPageOrderBy(q.Type)

	pageNumber := q.PageNumber
	if pageNumber <= 0 {
		pageNumber = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (pageNumber - 1) * pageSize

	query := fmt.Sprintf(
		"select %s from %s p where %s order by %s limit ?, ?",
		dProgramRows,
		m.table,
		strings.Join(conditions, " and "),
		orderBy,
	)
	args = append(args, offset, pageSize)

	var resp []*DProgram
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return resp, nil
}

func buildProgramBaseConditions(areaId int64) ([]string, []interface{}) {
	conditions := []string{
		"p.`status` = 1",
		"p.`program_status` = 1",
	}
	args := make([]interface{}, 0)

	if areaId > 0 {
		conditions = append(conditions, "p.`area_id` = ?")
		args = append(args, areaId)
	} else {
		conditions = append(conditions, "p.`prime` = 1")
	}

	return conditions, args
}

func buildProgramPageConditions(q *ProgramPageListQuery) ([]string, []interface{}) {
	conditions, args := buildProgramBaseConditions(q.AreaId)

	if q.ParentProgramCategoryId > 0 {
		conditions = append(conditions, "p.`parent_program_category_id` = ?")
		args = append(args, q.ParentProgramCategoryId)
	}
	if q.ProgramCategoryId > 0 {
		conditions = append(conditions, "p.`program_category_id` = ?")
		args = append(args, q.ProgramCategoryId)
	}

	if q.TimeType > 0 && q.StartDateTime != "" && q.EndDateTime != "" {
		conditions = append(conditions,
			"exists (select 1 from `d_program_show_time` st where st.`program_id` = p.`id` and st.`status` = 1 and st.`show_day_time` >= ? and st.`show_day_time` <= ?)",
		)
		args = append(args, q.StartDateTime, q.EndDateTime)
	}

	return conditions, args
}

func buildProgramPageOrderBy(tp int64) string {
	switch tp {
	case 2:
		return "p.`high_heat` desc, p.`id` desc"
	case 3:
		return "(select min(st.`show_time`) from `d_program_show_time` st where st.`program_id` = p.`id` and st.`status` = 1) asc, p.`id` asc"
	case 4:
		return "p.`issue_time` asc, p.`id` asc"
	default:
		return "p.`id` desc"
	}
}
