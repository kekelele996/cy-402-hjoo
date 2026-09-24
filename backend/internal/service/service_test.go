package service

import (
	"testing"
	"time"

	"cylawcase/internal/constants"
	"cylawcase/internal/model"
)

func TestCanFlow(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{constants.CaseStatusFiled, constants.CaseStatusInvestigating, true},
		{constants.CaseStatusInvestigating, constants.CaseStatusHearing, true},
		{constants.CaseStatusHearing, constants.CaseStatusClosed, true},
		{constants.CaseStatusClosed, constants.CaseStatusArchived, true},
		{constants.CaseStatusFiled, constants.CaseStatusClosed, false},
		{constants.CaseStatusClosed, constants.CaseStatusFiled, false},
		{constants.CaseStatusInvestigating, constants.CaseStatusFiled, true},
	}
	for _, tc := range cases {
		if got := canFlow(tc.from, tc.to); got != tc.want {
			t.Errorf("canFlow(%s->%s) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestContains(t *testing.T) {
	if !contains(constants.CaseTypeValues, constants.CaseTypeLabor) {
		t.Error("labor should be in case types")
	}
	if contains(constants.CaseTypeValues, "bogus") {
		t.Error("bogus should not be in case types")
	}
}

func TestU64(t *testing.T) {
	if u64(42) != "42" {
		t.Error("u64(42) != 42")
	}
}

func TestStatusValidators(t *testing.T) {
	if !constants.IsValidCaseStatus(constants.CaseStatusArchived) {
		t.Error("archived should be valid")
	}
	if constants.IsValidCaseStatus("bogus") {
		t.Error("bogus should be invalid")
	}
	if !constants.IsValidBillingStatus(constants.BillingStatusInvoiced) {
		t.Error("invoiced should be valid")
	}
	if !constants.IsValidBillingType(constants.BillingTypeTravelFee) {
		t.Error("travel_fee should be valid")
	}
}

func TestSumTimeEntryAmount(t *testing.T) {
	entries := []model.TimeEntry{
		{CaseID: 1, WorkDate: time.Now(), Hours: 2.5, HourlyRate: 1500},
		{CaseID: 1, WorkDate: time.Now(), Hours: 1.2, HourlyRate: 1800},
		{CaseID: 1, WorkDate: time.Now(), Hours: 0.5, HourlyRate: 0},
	}
	// 2.5*1500 + 1.2*1800 + 0 = 3750 + 2160 = 5910
	if got := sumTimeEntryAmount(entries); got != 5910 {
		t.Errorf("sumTimeEntryAmount = %v, want 5910", got)
	}
	if got := sumTimeEntryAmount(nil); got != 0 {
		t.Errorf("sumTimeEntryAmount(nil) = %v, want 0", got)
	}
}

func TestSumTimeEntryAmountRounding(t *testing.T) {
	entries := []model.TimeEntry{
		{Hours: 0.1, HourlyRate: 0.1},
		{Hours: 0.2, HourlyRate: 0.2},
	}
	got := sumTimeEntryAmount(entries)
	if got != 0.05 {
		t.Errorf("sumTimeEntryAmount rounding = %v, want 0.05", got)
	}
}

func TestTimeEntryAmountSnapshot(t *testing.T) {
	e := model.TimeEntry{Hours: 3, HourlyRate: 1500}
	if e.Amount() != 4500 {
		t.Errorf("Amount() = %v, want 4500", e.Amount())
	}
	// 费率快照：修改外部费率变量不影响已登记工时（费率存于记录本身）
	e.HourlyRate = 1800
	if e.Amount() != 5400 {
		t.Errorf("Amount() after snapshot change = %v, want 5400", e.Amount())
	}
}

func TestDedupeIDs(t *testing.T) {
	got := dedupeIDs([]uint64{1, 2, 2, 3, 1})
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("dedupeIDs = %v, want [1 2 3]", got)
	}
	if dedupeIDs(nil) != nil {
		t.Error("dedupeIDs(nil) should be nil")
	}
}
