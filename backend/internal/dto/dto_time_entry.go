package dto

// TimeEntryCreateRequest 登记工时请求。案件 ID 由路由 /cases/:id/time-entries 提供。
// HourlyRate 为指针：不传时由服务端取承办律师当前费率作为快照，显式传 0 表示免费工时。
type TimeEntryCreateRequest struct {
	CaseID      uint64   `json:"case_id"`
	WorkDate    string   `json:"work_date" binding:"required"`
	DurationMin int      `json:"duration_min" binding:"required,min=1,max=1440"`
	Description string   `json:"description" binding:"max=2000"`
	HourlyRate  *float64 `json:"hourly_rate" binding:"omitempty,min=0"`
}

// TimeEntryUpdateRequest 更新工时请求（仅允许更新未结算工时）。
type TimeEntryUpdateRequest struct {
	WorkDate    *string  `json:"work_date"`
	DurationMin *int     `json:"duration_min" binding:"omitempty,min=1,max=1440"`
	Description *string  `json:"description" binding:"omitempty,max=2000"`
	HourlyRate  *float64 `json:"hourly_rate" binding:"omitempty,min=0"`
}

// InvoiceGenerateRequest 为案件汇总未收费工时生成收费单请求。
type InvoiceGenerateRequest struct {
	CaseID      uint64 `json:"case_id" binding:"required"`
	InvoiceInfo string `json:"invoice_info" binding:"max=255"`
}
