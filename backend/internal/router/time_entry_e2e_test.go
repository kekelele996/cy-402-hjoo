package router

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"cylawcase/internal/config"
	"cylawcase/internal/constants"
	"cylawcase/internal/handler"
	"cylawcase/internal/model"
	"cylawcase/internal/repository"
	"cylawcase/internal/service"
	"cylawcase/internal/util"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupE2E(t *testing.T) (engine http.Handler, db *gorm.DB, token string, caseID uint64) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:e2e_router?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.User{}, &model.Client{}, &model.Case{}, &model.Document{},
		&model.Billing{}, &model.TimeEntry{}, &model.AuditLog{},
	); err != nil {
		t.Fatal(err)
	}

	lawyer := &model.User{Username: "lawyer_e2e", RealName: "E2E律师", Role: constants.RoleLawyer, HourlyRate: 600}
	client := &model.Client{Name: "E2E客户"}
	if err := db.Create(lawyer).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(client).Error; err != nil {
		t.Fatal(err)
	}
	cs := &model.Case{
		CaseNo: "CY-E2E-1", Title: "E2E案件", CaseType: constants.CaseTypeCivil,
		Status: constants.CaseStatusFiled, ClientID: client.ID, LeadLawyerID: lawyer.ID,
		CoLawyerIDs: model.CoLawyerJSON([]byte("[]")),
	}
	if err := db.Create(cs).Error; err != nil {
		t.Fatal(err)
	}

	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(&testLogWriter{t: t}, &slog.HandlerOptions{Level: slog.LevelError}))

	userRepo := repository.NewUserRepository(db)
	clientRepo := repository.NewClientRepository(db)
	caseRepo := repository.NewCaseRepository(db)
	docRepo := repository.NewDocumentRepository(db)
	billingRepo := repository.NewBillingRepository(db)
	teRepo := repository.NewTimeEntryRepository(db)

	userSvc := service.NewUserService(userRepo, logger)
	clientSvc := service.NewClientService(clientRepo, caseRepo, logger)
	caseSvc := service.NewCaseService(caseRepo, clientRepo, userRepo, logger)
	docSvc := service.NewDocumentService(docRepo, caseRepo, logger)
	billingSvc := service.NewBillingService(billingRepo, caseRepo, clientRepo, teRepo, logger)
	teSvc := service.NewTimeEntryService(teRepo, caseRepo, userRepo, logger)

	r := New(cfg, db, logger,
		handler.NewUserHandler(userSvc, logger),
		handler.NewClientHandler(clientSvc, logger),
		handler.NewCaseHandler(caseSvc, logger),
		handler.NewDocumentHandler(docSvc, logger),
		handler.NewBillingHandler(billingSvc, logger),
		handler.NewTimeEntryHandler(teSvc, logger),
		handler.NewUploadHandler(cfg, logger),
		handler.NewAuditLogHandler(db, logger),
	)

	token, err = util.GenerateToken(cfg.JWTSecret, cfg.JWTExpireDuration(), lawyer.ID, lawyer.Username, lawyer.Role)
	if err != nil {
		t.Fatal(err)
	}
	return r.Setup(), db, token, cs.ID
}

type testLogWriter struct{ t *testing.T }

func (w *testLogWriter) Write(p []byte) (int, error) { w.t.Logf("%s", p); return len(p), nil }

func doJSON(t *testing.T, engine http.Handler, method, path, token string, body any) (int, apiResp) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	var resp apiResp
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	}
	return rec.Code, resp
}

// TestTimeEntryInvoiceEndToEnd 走通：登记工时 → 案件待收费汇总 → 生成收费单 → 查看收费来源 →
// 重复生成失败（工时不重复入账）→ 作废后工时退回待结算。
func TestTimeEntryInvoiceEndToEnd(t *testing.T) {
	engine, _, token, caseID := setupE2E(t)

	// 未带 token 应被拒绝
	if code, _ := doJSON(t, engine, http.MethodGet, "/api/v1/cases/"+itoa(caseID)+"/time-summary", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated summary code = %d, want 401", code)
	}

	// 登记两条工时，费率快照分别为 600 / 400
	if code, resp := doJSON(t, engine, http.MethodPost, "/api/v1/cases/"+itoa(caseID)+"/time-entries", token,
		map[string]any{"work_date": "2026-09-20", "duration_min": 60, "description": "会见", "hourly_rate": 600}); code != http.StatusOK || resp.Code != 0 {
		t.Fatalf("create entry1 code=%d body=%s", code, resp.Message)
	}
	if code, resp := doJSON(t, engine, http.MethodPost, "/api/v1/cases/"+itoa(caseID)+"/time-entries", token,
		map[string]any{"work_date": "2026-09-21", "duration_min": 90, "description": "阅卷", "hourly_rate": 400}); code != http.StatusOK || resp.Code != 0 {
		t.Fatalf("create entry2 code=%d body=%s", code, resp.Message)
	}

	// 案件待收费汇总：150 分钟 = 2.5 小时；金额 600 + 600 = 1200
	code, resp := doJSON(t, engine, http.MethodGet, "/api/v1/cases/"+itoa(caseID)+"/time-summary", token, nil)
	if code != http.StatusOK {
		t.Fatalf("summary code=%d", code)
	}
	var summary struct {
		UnbilledMinutes int     `json:"unbilled_minutes"`
		UnbilledHours   float64 `json:"unbilled_hours"`
		EstimatedAmount float64 `json:"estimated_amount"`
	}
	_ = json.Unmarshal(resp.Data, &summary)
	if summary.UnbilledMinutes != 150 || summary.EstimatedAmount != 1200 {
		t.Fatalf("summary = %+v, want minutes=150 amount=1200", summary)
	}

	// 生成收费单
	code, resp = doJSON(t, engine, http.MethodPost, "/api/v1/billings/invoice-from-time", token,
		map[string]any{"case_id": caseID})
	if code != http.StatusOK || resp.Code != 0 {
		t.Fatalf("generate invoice code=%d msg=%s", code, resp.Message)
	}
	var invoice model.Billing
	_ = json.Unmarshal(resp.Data, &invoice)
	if invoice.Amount != 1200 || invoice.Source != constants.BillingSourceTimeEntries {
		t.Fatalf("invoice = %+v", invoice)
	}

	// 账单页展开查看收费来源：两条工时
	code, resp = doJSON(t, engine, http.MethodGet, "/api/v1/time-entries/by-billing/"+itoa(invoice.ID), token, nil)
	if code != http.StatusOK {
		t.Fatalf("sources code=%d", code)
	}
	var sources []model.TimeEntry
	_ = json.Unmarshal(resp.Data, &sources)
	if len(sources) != 2 {
		t.Fatalf("sources len = %d, want 2", len(sources))
	}

	// 待收费汇总应已清零
	_, resp = doJSON(t, engine, http.MethodGet, "/api/v1/cases/"+itoa(caseID)+"/time-summary", token, nil)
	_ = json.Unmarshal(resp.Data, &summary)
	if summary.UnbilledMinutes != 0 || summary.EstimatedAmount != 0 {
		t.Fatalf("summary after invoice = %+v, want zero", summary)
	}

	// 再次生成应失败（422），且不会新增账单——一项工时只能进入一张有效收费单
	if code, _ := doJSON(t, engine, http.MethodPost, "/api/v1/billings/invoice-from-time", token,
		map[string]any{"case_id": caseID}); code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate generate code = %d, want 422", code)
	}

	// 作废收费单，工时退回待结算
	if code, resp := doJSON(t, engine, http.MethodPost, "/api/v1/billings/"+itoa(invoice.ID)+"/void", token, nil); code != http.StatusOK || resp.Code != 0 {
		t.Fatalf("void code=%d msg=%s", code, resp.Message)
	}
	_, resp = doJSON(t, engine, http.MethodGet, "/api/v1/cases/"+itoa(caseID)+"/time-summary", token, nil)
	_ = json.Unmarshal(resp.Data, &summary)
	if summary.UnbilledMinutes != 150 || summary.EstimatedAmount != 1200 {
		t.Fatalf("summary after void = %+v, want minutes=150 amount=1200", summary)
	}

	// 作废后可重新汇总生成新收费单
	code, resp = doJSON(t, engine, http.MethodPost, "/api/v1/billings/invoice-from-time", token,
		map[string]any{"case_id": caseID})
	if code != http.StatusOK {
		t.Fatalf("regenerate after void code=%d msg=%s", code, resp.Message)
	}
	var invoice2 model.Billing
	_ = json.Unmarshal(resp.Data, &invoice2)
	if invoice2.ID == invoice.ID {
		t.Fatal("expected a new billing id after re-invoicing")
	}
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
