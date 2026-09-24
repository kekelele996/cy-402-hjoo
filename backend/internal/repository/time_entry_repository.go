package repository

import (
	"errors"
	"fmt"

	"cylawcase/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TimeEntryRepository 工时记录仓储。
type TimeEntryRepository struct {
	db *gorm.DB
}

// NewTimeEntryRepository 构造工时记录仓储。
func NewTimeEntryRepository(db *gorm.DB) *TimeEntryRepository {
	return &TimeEntryRepository{db: db}
}

// Create 创建工时记录。
func (r *TimeEntryRepository) Create(e *model.TimeEntry) error {
	if err := r.db.Create(e).Error; err != nil {
		return fmt.Errorf("create time entry: %w", err)
	}
	return nil
}

// FindByID 按 ID 查询工时记录。
func (r *TimeEntryRepository) FindByID(id uint64) (*model.TimeEntry, error) {
	var e model.TimeEntry
	if err := r.db.First(&e, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find time entry by id: %w", err)
	}
	return &e, nil
}

// ListByCase 查询某案件的工时记录，unbilledOnly 为 true 时只看待结算工时。
func (r *TimeEntryRepository) ListByCase(caseID uint64, unbilledOnly bool) ([]model.TimeEntry, error) {
	var list []model.TimeEntry
	q := r.db.Where("case_id = ?", caseID)
	if unbilledOnly {
		q = q.Where("billing_id IS NULL")
	}
	if err := q.Order("work_date DESC, id DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list time entries by case: %w", err)
	}
	return list, nil
}

// ListByBilling 查询进入某张收费单的工时记录（收费来源）。
func (r *TimeEntryRepository) ListByBilling(billingID uint64) ([]model.TimeEntry, error) {
	var list []model.TimeEntry
	if err := r.db.Where("billing_id = ?", billingID).Order("work_date, id").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list time entries by billing: %w", err)
	}
	return list, nil
}

// UnbilledSummary 汇总案件待结算工时的总时长与预计金额。
func (r *TimeEntryRepository) UnbilledSummary(caseID uint64) (hours, amount float64, count int64, err error) {
	var res struct {
		Hours  float64
		Amount float64
		Count  int64
	}
	if err = r.db.Model(&model.TimeEntry{}).
		Select("COALESCE(SUM(hours), 0) AS hours, COALESCE(SUM(hours * hourly_rate), 0) AS amount, COUNT(*) AS count").
		Where("case_id = ? AND billing_id IS NULL", caseID).
		Scan(&res).Error; err != nil {
		return 0, 0, 0, fmt.Errorf("unbilled time entries summary: %w", err)
	}
	return res.Hours, res.Amount, res.Count, nil
}

// Delete 删除工时记录。
func (r *TimeEntryRepository) Delete(id uint64) error {
	if err := r.db.Delete(&model.TimeEntry{}, id).Error; err != nil {
		return fmt.Errorf("delete time entry: %w", err)
	}
	return nil
}

// LockUnbilledForUpdate 在事务内行级锁定案件待结算工时，防止并发生成收费单时重复结算。
func (r *TimeEntryRepository) LockUnbilledForUpdate(tx *gorm.DB, caseID uint64, entryIDs []uint64) ([]model.TimeEntry, error) {
	var list []model.TimeEntry
	q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("case_id = ? AND billing_id IS NULL", caseID)
	if len(entryIDs) > 0 {
		q = q.Where("id IN ?", entryIDs)
	}
	if err := q.Order("id").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("lock unbilled time entries: %w", err)
	}
	return list, nil
}

// BindBilling 在事务内把工时绑定到收费单（仅绑定仍处于待结算状态的工时）。
func (r *TimeEntryRepository) BindBilling(tx *gorm.DB, entryIDs []uint64, billingID uint64) error {
	if len(entryIDs) == 0 {
		return nil
	}
	if err := tx.Model(&model.TimeEntry{}).
		Where("id IN ? AND billing_id IS NULL", entryIDs).
		Update("billing_id", billingID).Error; err != nil {
		return fmt.Errorf("bind time entries to billing: %w", err)
	}
	return nil
}

// ReleaseByBilling 在事务内释放收费单关联的工时，使其回到待结算列表。
func (r *TimeEntryRepository) ReleaseByBilling(tx *gorm.DB, billingID uint64) error {
	if err := tx.Model(&model.TimeEntry{}).
		Where("billing_id = ?", billingID).
		Update("billing_id", nil).Error; err != nil {
		return fmt.Errorf("release time entries of billing: %w", err)
	}
	return nil
}
