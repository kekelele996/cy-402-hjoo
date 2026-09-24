package dto

// TimeEntryCreateRequest 登记工时请求。
type TimeEntryCreateRequest struct {
	WorkDate    string  `json:"work_date" binding:"required"` // YYYY-MM-DD
	Hours       float64 `json:"hours" binding:"required,gt=0,lte=24"`
	Description string  `json:"description" binding:"required,max=500"`
	HourlyRate  float64 `json:"hourly_rate" binding:"gte=0"`
}

// UnbilledSummaryResponse 案件待结算工时汇总。
type UnbilledSummaryResponse struct {
	Hours  float64 `json:"hours"`
	Amount float64 `json:"amount"`
	Count  int64   `json:"count"`
}

// BillingGenerateRequest 汇总未收费工时生成收费单请求。
type BillingGenerateRequest struct {
	CaseID      uint64   `json:"case_id" binding:"required"`
	EntryIDs    []uint64 `json:"entry_ids"` // 为空表示汇总该案件全部待结算工时
	InvoiceInfo string   `json:"invoice_info" binding:"max=255"`
}
