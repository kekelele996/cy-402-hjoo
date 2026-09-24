package model

import "time"

// TimeEntry 工时记录实体：承办律师在案件下登记的计费工时。
type TimeEntry struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	CaseID      uint64    `gorm:"not null;index" json:"case_id"`
	LawyerID    uint64    `gorm:"not null;index" json:"lawyer_id"`
	WorkDate    time.Time `gorm:"type:date;not null" json:"work_date"`
	Hours       float64   `gorm:"type:numeric(8,2);not null" json:"hours"`
	Description string    `gorm:"size:500;not null" json:"description"`
	// HourlyRate 登记时的费率快照；之后费率调整不回改旧工时。
	HourlyRate float64 `gorm:"type:numeric(12,2);not null;default:0" json:"hourly_rate"`
	// BillingID 该工时进入的收费单；一张有效收费单独占一项工时，账单作废后释放回待结算。
	BillingID *uint64   `gorm:"index" json:"billing_id"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名。
func (TimeEntry) TableName() string { return "time_entries" }

// Amount 该工时的收费金额 = 时长 × 登记时费率。
func (e *TimeEntry) Amount() float64 { return e.Hours * e.HourlyRate }
