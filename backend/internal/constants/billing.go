package constants

// BillingType 费用类型枚举。
const (
	BillingTypeAttorneyFee = "attorney_fee"
	BillingTypeCourtFee    = "court_fee"
	BillingTypeTravelFee   = "travel_fee"
	BillingTypeOther       = "other"
)

// BillingTypeValues 全部费用类型值。
var BillingTypeValues = []string{BillingTypeAttorneyFee, BillingTypeCourtFee, BillingTypeTravelFee, BillingTypeOther}

// BillingStatus 账单状态枚举。
const (
	BillingStatusPending  = "pending"
	BillingStatusPaid     = "paid"
	BillingStatusInvoiced = "invoiced"
	BillingStatusVoid     = "void"
)

// BillingStatusValues 全部账单状态值。
var BillingStatusValues = []string{BillingStatusPending, BillingStatusPaid, BillingStatusInvoiced, BillingStatusVoid}

// BillingSource 账单来源枚举。
const (
	BillingSourceManual      = "manual"       // 手工创建
	BillingSourceTimeEntries = "time_entries" // 未收费工时汇总生成
)

// BillingSourceValues 全部账单来源值。
var BillingSourceValues = []string{BillingSourceManual, BillingSourceTimeEntries}

// DefaultHourlyRate 律师默认每小时费率（元/小时），登记工时未显式给费率时使用。
const DefaultHourlyRate = 500.0

// IsValidBillingSource 校验账单来源。
func IsValidBillingSource(s string) bool {
	for _, v := range BillingSourceValues {
		if v == s {
			return true
		}
	}
	return false
}

// IsValidBillingType 校验费用类型。
func IsValidBillingType(s string) bool {
	for _, v := range BillingTypeValues {
		if v == s {
			return true
		}
	}
	return false
}

// IsValidBillingStatus 校验账单状态。
func IsValidBillingStatus(s string) bool {
	for _, v := range BillingStatusValues {
		if v == s {
			return true
		}
	}
	return false
}

// DocumentFileType 文档类型枚举。
const (
	DocTypeComplaint = "complaint"
	DocTypeDefense   = "defense"
	DocTypeEvidence  = "evidence"
	DocTypeJudgment  = "judgment"
	DocTypeContract  = "contract"
	DocTypeOther     = "other"
)

// DocumentFileTypeValues 全部文档类型值。
var DocumentFileTypeValues = []string{DocTypeComplaint, DocTypeDefense, DocTypeEvidence, DocTypeJudgment, DocTypeContract, DocTypeOther}
