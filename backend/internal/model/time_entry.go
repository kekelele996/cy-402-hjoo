package model

import "time"

// TimeEntry 工时记录实体。承办律师在案件中登记日期、时长、工作内容与当时费率；
// HourlyRate 为登记时刻的费率快照，之后律师费率调整不改变已登记工时。
type TimeEntry struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	CaseID      uint64    `gorm:"not null;index" json:"case_id"`
	LawyerID    uint64    `gorm:"not null;index" json:"lawyer_id"`
	WorkDate    time.Time `gorm:"type:date;not null;index" json:"work_date"`
	DurationMin int       `gorm:"not null;default:0" json:"duration_min"`
	Description string    `gorm:"type:text;not null;default:''" json:"description"`
	HourlyRate  float64   `gorm:"type:numeric(12,2);not null;default:0" json:"hourly_rate"`
	// BillingID 为空表示尚未进入任何收费单（待结算）；非空表示已归入对应收费单。
	// 收费单作废时会重新置空，工时回到待结算列表。一项工时同一时刻只关联一张收费单。
	BillingID *uint64   `gorm:"index" json:"billing_id"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名。
func (TimeEntry) TableName() string { return "time_entries" }
