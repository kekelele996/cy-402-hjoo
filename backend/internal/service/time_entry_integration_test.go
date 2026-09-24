package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"cylawcase/internal/constants"
	"cylawcase/internal/model"
	"cylawcase/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// newTestDB 构造内存 SQLite 并迁移全部实体（SQLite 驱动会忽略 FOR UPDATE 行锁，
// 事务的原子提交/回滚语义与 PostgreSQL 一致，足以验证"失败则工时留在待结算列表"）。
var testDBCounter int64

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:memdb%d?mode=memory&cache=shared", atomic.AddInt64(&testDBCounter, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// 内存库在最后一个连接关闭时销毁；保持连接池长连接避免中途丢库。
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := db.AutoMigrate(
		&model.User{}, &model.Client{}, &model.Case{}, &model.Document{},
		&model.Billing{}, &model.TimeEntry{}, &model.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedCaseFixtures(t *testing.T, db *gorm.DB, rate float64) (lawyerID, clientID, caseID uint64) {
	t.Helper()
	lawyer := &model.User{Username: "lawyer_t", RealName: "测试律师", Role: constants.RoleLawyer, HourlyRate: rate}
	if err := db.Create(lawyer).Error; err != nil {
		t.Fatalf("create lawyer: %v", err)
	}
	client := &model.Client{Name: "测试客户"}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	co := model.CoLawyerJSON(json.RawMessage("[]"))
	cs := &model.Case{
		CaseNo: "CY-TEST-1", Title: "测试案件", CaseType: constants.CaseTypeCivil,
		Status: constants.CaseStatusFiled, ClientID: client.ID, LeadLawyerID: lawyer.ID, CoLawyerIDs: co,
	}
	if err := db.Create(cs).Error; err != nil {
		t.Fatalf("create case: %v", err)
	}
	return lawyer.ID, client.ID, cs.ID
}

func newSvc(t *testing.T) (db *gorm.DB, bs *BillingService, ts *TimeEntryService) {
	t.Helper()
	db = newTestDB(t)
	logger := slog.New(slog.NewTextHandler(&testWriter{t: t}, &slog.HandlerOptions{Level: slog.LevelError}))
	userRepo := repository.NewUserRepository(db)
	caseRepo := repository.NewCaseRepository(db)
	clientRepo := repository.NewClientRepository(db)
	billingRepo := repository.NewBillingRepository(db)
	teRepo := repository.NewTimeEntryRepository(db)
	bs = NewBillingService(billingRepo, caseRepo, clientRepo, teRepo, logger)
	ts = NewTimeEntryService(teRepo, caseRepo, userRepo, logger)
	return db, bs, ts
}

type testWriter struct{ t *testing.T }

func (w *testWriter) Write(p []byte) (int, error) { w.t.Logf("%s", p); return len(p), nil }

// TestGenerateInvoice_HappyPath 汇总未收费工时生成收费单：金额按快照费率，工时被挂载。
func TestGenerateInvoice_HappyPath(t *testing.T) {
	db, bs, ts := newSvc(t)
	lawyerID, _, caseID := seedCaseFixtures(t, db, 600)

	d := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	createEntry(t, ts, lawyerID, caseID, d, 60, "会见", ptr(600.0))
	createEntry(t, ts, lawyerID, caseID, d.AddDate(0, 0, 1), 90, "阅卷", ptr(400.0))

	b, err := bs.GenerateInvoiceFromTimeEntries(caseID, "")
	if err != nil {
		t.Fatalf("generate invoice: %v", err)
	}
	// 1h*600 + 1.5h*400 = 600 + 600 = 1200
	if b.Amount != 1200 {
		t.Fatalf("amount = %.2f, want 1200.00", b.Amount)
	}
	if b.Source != constants.BillingSourceTimeEntries {
		t.Fatalf("source = %s, want time_entries", b.Source)
	}
	if b.BillingType != constants.BillingTypeAttorneyFee || b.Status != constants.BillingStatusPending {
		t.Fatalf("unexpected billing type/status: %s/%s", b.BillingType, b.Status)
	}

	// 工时应全部挂到该收费单
	var entries []model.TimeEntry
	if err := db.Where("case_id = ?", caseID).Find(&entries).Error; err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.BillingID == nil || *e.BillingID != b.ID {
			t.Fatalf("time entry %d not attached to billing %d (got %v)", e.ID, b.ID, e.BillingID)
		}
	}

	// 待结算汇总应归零
	_, hours, amount, err := ts.UnbilledSummary(caseID)
	if err != nil {
		t.Fatal(err)
	}
	if hours != 0 || amount != 0 {
		t.Fatalf("unbilled summary after invoice = %.2fh/%.2f, want 0/0", hours, amount)
	}
}

// TestGenerateInvoice_NoEntries_FailsAndKeepsList 没有未收费工时时生成失败，不残留账单，工时仍在待结算列表。
func TestGenerateInvoice_NoEntries_FailsAndKeepsList(t *testing.T) {
	db, bs, _ := newSvc(t)
	lawyerID, _, caseID := seedCaseFixtures(t, db, 600)
	_ = lawyerID

	_, err := bs.GenerateInvoiceFromTimeEntries(caseID, "")
	if err == nil {
		t.Fatal("expected error when no unbilled time entries")
	}

	var billingCount int64
	db.Model(&model.Billing{}).Where("case_id = ?", caseID).Count(&billingCount)
	if billingCount != 0 {
		t.Fatalf("expected no billings on failure, got %d", billingCount)
	}
}

// TestGenerateInvoice_AlreadyBilled_SecondCallFails 一项工时只能进入一张有效收费单：
// 第一次生成后，再次生成因没有未收费工时失败，已入单工时不重复结算。
func TestGenerateInvoice_AlreadyBilled_SecondCallFails(t *testing.T) {
	db, bs, ts := newSvc(t)
	lawyerID, _, caseID := seedCaseFixtures(t, db, 600)
	d := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	createEntry(t, ts, lawyerID, caseID, d, 120, "检索", ptr(600.0))

	first, err := bs.GenerateInvoiceFromTimeEntries(caseID, "")
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	if first.Amount != 1200 {
		t.Fatalf("first amount = %.2f, want 1200", first.Amount)
	}

	if _, err := bs.GenerateInvoiceFromTimeEntries(caseID, ""); err == nil {
		t.Fatal("second generate must fail because entries already billed")
	}

	// 只有一张工时来源收费单，且每条工时只关联一次
	var cnt int64
	db.Model(&model.Billing{}).Where("case_id = ? AND source = ?", caseID, constants.BillingSourceTimeEntries).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("expected 1 time-entry billing, got %d", cnt)
	}
	var attached int64
	db.Model(&model.TimeEntry{}).Where("billing_id = ?", first.ID).Count(&attached)
	if attached != 1 {
		t.Fatalf("expected 1 attached entry, got %d", attached)
	}
}

// TestVoidInvoice_ReleasesEntries 作废工时收费单后，工时回到待结算列表，可重新生成收费单。
func TestVoidInvoice_ReleasesEntries(t *testing.T) {
	db, bs, ts := newSvc(t)
	lawyerID, _, caseID := seedCaseFixtures(t, db, 600)
	d := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	createEntry(t, ts, lawyerID, caseID, d, 120, "检索", ptr(600.0))

	first, err := bs.GenerateInvoiceFromTimeEntries(caseID, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := bs.Void(first.ID); err != nil {
		t.Fatalf("void: %v", err)
	}

	// 工时 billing_id 应被释放
	var e model.TimeEntry
	if err := db.Where("case_id = ?", caseID).First(&e).Error; err != nil {
		t.Fatal(err)
	}
	if e.BillingID != nil {
		t.Fatalf("expected released entry, got billing_id=%d", *e.BillingID)
	}

	// 待结算汇总恢复
	minutes, _, amount, err := ts.UnbilledSummary(caseID)
	if err != nil {
		t.Fatal(err)
	}
	if minutes != 120 || amount != 1200 {
		t.Fatalf("summary after void = %dmin/%.2f, want 120/1200", minutes, amount)
	}

	// 作废账单是"无效"收费单，可重新汇总生成新收费单
	second, err := bs.GenerateInvoiceFromTimeEntries(caseID, "")
	if err != nil {
		t.Fatalf("regenerate after void: %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("expected a new billing after re-invoicing")
	}
}

// TestRateChangeDoesNotAffectOldEntries 费率调整后旧工时快照费率不变。
func TestRateChangeDoesNotAffectOldEntries(t *testing.T) {
	db, _, ts := newSvc(t)
	lawyerID, _, caseID := seedCaseFixtures(t, db, 500)
	d := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)

	created, err := ts.Create(lawyerID, caseID, d, 60, "旧工时", nil)
	if err != nil {
		t.Fatalf("create old entry: %v", err)
	}
	if created.HourlyRate != 500 {
		t.Fatalf("old entry rate = %.2f, want 500", created.HourlyRate)
	}

	// 律师费率上调到 800
	if err := db.Model(&model.User{}).Where("id = ?", lawyerID).Update("hourly_rate", 800).Error; err != nil {
		t.Fatal(err)
	}

	// 新登记工时取新费率
	newEntry, err := ts.Create(lawyerID, caseID, d.AddDate(0, 0, 10), 60, "新工时", nil)
	if err != nil {
		t.Fatalf("create new entry: %v", err)
	}
	if newEntry.HourlyRate != 800 {
		t.Fatalf("new entry rate = %.2f, want 800", newEntry.HourlyRate)
	}

	// 旧工时费率仍是 500
	var old model.TimeEntry
	if err := db.First(&old, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if old.HourlyRate != 500 {
		t.Fatalf("old entry rate changed to %.2f, want 500", old.HourlyRate)
	}
}

// TestUpdateBilledEntryRejected 已进入有效收费单的工时不允许修改/删除。
func TestUpdateBilledEntryRejected(t *testing.T) {
	db, bs, ts := newSvc(t)
	lawyerID, _, caseID := seedCaseFixtures(t, db, 600)
	d := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	entry := createEntry(t, ts, lawyerID, caseID, d, 60, "x", ptr(600.0))
	b := genInvoice(t, bs, caseID)

	if _, err := ts.Update(entry.ID, nil, ptr(90), nil, nil); err == nil {
		t.Fatal("expected update of billed entry to fail")
	}
	if err := ts.Delete(entry.ID); err == nil {
		t.Fatal("expected delete of billed entry to fail")
	}

	// 作废后可编辑
	if _, err := bs.Void(b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Update(entry.ID, nil, ptr(90), nil, nil); err != nil {
		t.Fatalf("update after void should succeed: %v", err)
	}
}

func must(t *testing.T, v *model.TimeEntry, err error) *model.TimeEntry {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	return v
}

func mustB(t *testing.T, b *model.Billing, err error) *model.Billing {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	return b
}

func createEntry(t *testing.T, ts *TimeEntryService, lawyerID, caseID uint64, d time.Time, min int, desc string, rate *float64) *model.TimeEntry {
	t.Helper()
	v, err := ts.Create(lawyerID, caseID, d, min, desc, rate)
	return must(t, v, err)
}

func genInvoice(t *testing.T, bs *BillingService, caseID uint64) *model.Billing {
	t.Helper()
	b, err := bs.GenerateInvoiceFromTimeEntries(caseID, "")
	return mustB(t, b, err)
}

func ptr[T any](v T) *T { return &v }

// 确保测试引用到 clause（锁定子句），与生产代码使用保持一致。
var _ = clause.Locking{}
