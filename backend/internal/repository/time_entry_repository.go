package repository

import (
	"errors"
	"fmt"
	"time"

	"cylawcase/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TimeEntryStatus 工时列表筛选状态。
const (
	TimeEntryFilterAll      = ""
	TimeEntryFilterUnbilled = "unbilled"
	TimeEntryFilterBilled   = "billed"
)

// TimeEntryRepository 工时仓储。
type TimeEntryRepository struct {
	db *gorm.DB
}

// NewTimeEntryRepository 构造工时仓储。
func NewTimeEntryRepository(db *gorm.DB) *TimeEntryRepository {
	return &TimeEntryRepository{db: db}
}

// Transaction 在一个数据库事务中执行，保证生成收费单与挂载工时原子提交。
func (r *TimeEntryRepository) Transaction(fn func(tx *gorm.DB) error) error {
	if err := r.db.Transaction(fn); err != nil {
		return fmt.Errorf("time entry transaction: %w", err)
	}
	return nil
}

// Create 登记工时。
func (r *TimeEntryRepository) Create(t *model.TimeEntry) error {
	if err := r.db.Create(t).Error; err != nil {
		return fmt.Errorf("create time entry: %w", err)
	}
	return nil
}

// Update 更新工时。
func (r *TimeEntryRepository) Update(t *model.TimeEntry) error {
	if err := r.db.Save(t).Error; err != nil {
		return fmt.Errorf("update time entry: %w", err)
	}
	return nil
}

// Delete 删除工时。
func (r *TimeEntryRepository) Delete(id uint64) error {
	res := r.db.Delete(&model.TimeEntry{}, id)
	if res.Error != nil {
		return fmt.Errorf("delete time entry: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// FindByID 按 ID 查询工时。
func (r *TimeEntryRepository) FindByID(id uint64) (*model.TimeEntry, error) {
	var t model.TimeEntry
	if err := r.db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find time entry by id: %w", err)
	}
	return &t, nil
}

// ListByCase 查询某案件的工时，可按结算状态（unbilled/billed）过滤。
func (r *TimeEntryRepository) ListByCase(caseID uint64, statusFilter string) ([]model.TimeEntry, error) {
	var list []model.TimeEntry
	q := r.db.Where("case_id = ?", caseID)
	switch statusFilter {
	case TimeEntryFilterUnbilled:
		q = q.Where("billing_id IS NULL")
	case TimeEntryFilterBilled:
		q = q.Where("billing_id IS NOT NULL")
	}
	if err := q.Order("work_date DESC, id DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list time entries by case: %w", err)
	}
	return list, nil
}

// ListByBilling 查询某张收费单包含的工时（收费来源）。
func (r *TimeEntryRepository) ListByBilling(billingID uint64) ([]model.TimeEntry, error) {
	var list []model.TimeEntry
	if err := r.db.Where("billing_id = ?", billingID).
		Order("work_date ASC, id ASC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list time entries by billing: %w", err)
	}
	return list, nil
}

// ListUnbilledByCaseForUpdate 在事务内锁定某案件全部未收费工时行。
// 并发生成收费单时，后到事务会在此阻塞，待前者提交后重新读取，
// 已被挂载的工时不再满足 billing_id IS NULL，从而杜绝同一段工时重复入账。
func (r *TimeEntryRepository) ListUnbilledByCaseForUpdate(tx *gorm.DB, caseID uint64) ([]model.TimeEntry, error) {
	var list []model.TimeEntry
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("case_id = ? AND billing_id IS NULL", caseID).
		Order("work_date ASC, id ASC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list unbilled time entries for update: %w", err)
	}
	return list, nil
}

// AttachToBilling 在事务内把指定工时挂到收费单上；只挂载仍处于未收费状态的工时，
// 返回实际挂载行数，供上层检测并发竞争。
func (r *TimeEntryRepository) AttachToBilling(tx *gorm.DB, ids []uint64, billingID uint64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := tx.Model(&model.TimeEntry{}).
		Where("id IN ? AND billing_id IS NULL", ids).
		Update("billing_id", billingID)
	if res.Error != nil {
		return 0, fmt.Errorf("attach time entries to billing: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// ReleaseByBilling 在事务内把某张收费单下的工时重新置为未收费（收费单作废时调用）。
func (r *TimeEntryRepository) ReleaseByBilling(tx *gorm.DB, billingID uint64) (int64, error) {
	res := tx.Model(&model.TimeEntry{}).
		Where("billing_id = ?", billingID).
		Update("billing_id", nil)
	if res.Error != nil {
		return 0, fmt.Errorf("release time entries by billing: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// UnbilledSummary 某案件未收费工时总时长（分钟）与预计金额。
func (r *TimeEntryRepository) UnbilledSummary(caseID uint64) (int64, float64, error) {
	var row struct {
		Minutes int64   `gorm:"column:minutes"`
		Amount  float64 `gorm:"column:amount"`
	}
	if err := r.db.Model(&model.TimeEntry{}).
		Select("COALESCE(SUM(duration_min), 0) AS minutes, COALESCE(SUM(duration_min / 60.0 * hourly_rate), 0) AS amount").
		Where("case_id = ? AND billing_id IS NULL", caseID).
		Scan(&row).Error; err != nil {
		return 0, 0, fmt.Errorf("summary unbilled time entries: %w", err)
	}
	return row.Minutes, row.Amount, nil
}

// ListByLawyerAndRange 查询律师在某时间区间内的工时（预留统计用）。
func (r *TimeEntryRepository) ListByLawyerAndRange(lawyerID uint64, start, end time.Time) ([]model.TimeEntry, error) {
	var list []model.TimeEntry
	if err := r.db.Where("lawyer_id = ? AND work_date >= ? AND work_date <= ?", lawyerID, start, end).
		Order("work_date DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list time entries by lawyer: %w", err)
	}
	return list, nil
}
