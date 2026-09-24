package service

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"cylawcase/internal/constants"
	"cylawcase/internal/model"
	"cylawcase/internal/repository"
	"cylawcase/internal/util"

	"gorm.io/gorm"
)

// BillingService 账单业务逻辑。
type BillingService struct {
	repo          *repository.BillingRepository
	caseRepo      *repository.CaseRepository
	clientRepo    *repository.ClientRepository
	timeEntryRepo *repository.TimeEntryRepository
	logger        *slog.Logger
}

// NewBillingService 构造账单服务。
func NewBillingService(repo *repository.BillingRepository, caseRepo *repository.CaseRepository,
	clientRepo *repository.ClientRepository, timeEntryRepo *repository.TimeEntryRepository, logger *slog.Logger) *BillingService {
	return &BillingService{repo: repo, caseRepo: caseRepo, clientRepo: clientRepo, timeEntryRepo: timeEntryRepo, logger: logger}
}

// Create 创建账单。
func (s *BillingService) Create(caseID, clientID uint64, billingType string, amount float64, invoiceInfo string) (*model.Billing, error) {
	if !constants.IsValidBillingType(billingType) {
		return nil, util.NewAppError(constants.CodeValidationFailed, "Billing[billing_type="+billingType+"] create: invalid type")
	}
	if _, err := s.caseRepo.FindByID(caseID); err != nil {
		return nil, util.Wrap(err, "Billing[case_id=%d] create: case not found", caseID)
	}
	if _, err := s.clientRepo.FindByID(clientID); err != nil {
		return nil, util.Wrap(err, "Billing[client_id=%d] create: client not found", clientID)
	}
	if amount < 0 {
		return nil, util.NewAppError(constants.CodeValidationFailed, "Billing[amount="+strconv.FormatFloat(amount, 'f', 2, 64)+"] create: amount must be >= 0")
	}
	b := &model.Billing{BillNo: genBillNo(), BillingType: billingType,
		Amount: amount, Status: constants.BillingStatusPending,
		CaseID: caseID, ClientID: clientID, InvoiceInfo: invoiceInfo}
	if err := s.repo.Create(b); err != nil {
		s.logger.Error(constants.LogBillingCreateFailed, "error", err.Error())
		return nil, util.Wrap(err, "Billing[case_id=%d] create failed", caseID)
	}
	s.logger.Info(constants.LogBillingCreateSuccess, "billing_id", b.ID, "bill_no", b.BillNo)
	return b, nil
}

// GenerateFromTimeEntries 汇总案件未收费工时生成收费单。
// 整个流程在单事务内完成：任一步失败工时保持待结算；行级锁保证一项工时只进入一张有效收费单。
func (s *BillingService) GenerateFromTimeEntries(caseID uint64, entryIDs []uint64, invoiceInfo string) (*model.Billing, []model.TimeEntry, error) {
	cs, err := s.caseRepo.FindByID(caseID)
	if err != nil {
		return nil, nil, util.Wrap(err, "Billing[case_id=%d] generate: case not found", caseID)
	}
	entryIDs = dedupeIDs(entryIDs)
	var billing *model.Billing
	var entries []model.TimeEntry
	err = s.repo.Transaction(func(tx *gorm.DB) error {
		locked, err := s.timeEntryRepo.LockUnbilledForUpdate(tx, caseID, entryIDs)
		if err != nil {
			return fmt.Errorf("lock unbilled time entries: %w", err)
		}
		if len(locked) == 0 {
			return util.NewAppError(constants.CodeNoUnbilledTimeEntries,
				"Billing[case_id="+u64(caseID)+"] generate: no unbilled time entries")
		}
		if len(entryIDs) > 0 && len(locked) != len(entryIDs) {
			return util.NewAppError(constants.CodeTimeEntryAlreadyBilled,
				"Billing[case_id="+u64(caseID)+"] generate: some time entries already billed")
		}
		b := &model.Billing{BillNo: genBillNo(), BillingType: constants.BillingTypeAttorneyFee,
			Amount: sumTimeEntryAmount(locked), Status: constants.BillingStatusPending,
			CaseID: caseID, ClientID: cs.ClientID, InvoiceInfo: invoiceInfo}
		if err := s.repo.CreateTx(tx, b); err != nil {
			return fmt.Errorf("create billing from time entries: %w", err)
		}
		ids := make([]uint64, 0, len(locked))
		for _, e := range locked {
			ids = append(ids, e.ID)
		}
		if err := s.timeEntryRepo.BindBilling(tx, ids, b.ID); err != nil {
			return fmt.Errorf("bind time entries to billing: %w", err)
		}
		billing = b
		entries = locked
		return nil
	})
	if err != nil {
		s.logger.Error(constants.LogBillingGenerateFailed, "case_id", caseID, "error", err.Error())
		return nil, nil, err
	}
	s.logger.Info(constants.LogBillingGenerateSuccess, "billing_id", billing.ID, "bill_no", billing.BillNo, "entry_count", len(entries))
	return billing, entries, nil
}

// ListTimeEntries 查询账单的收费来源工时。
func (s *BillingService) ListTimeEntries(billingID uint64) ([]model.TimeEntry, error) {
	if _, err := s.repo.FindByID(billingID); err != nil {
		return nil, util.Wrap(err, "Billing[id=%d] time entries: billing not found", billingID)
	}
	return s.timeEntryRepo.ListByBilling(billingID)
}

// sumTimeEntryAmount 按“时长 × 登记时费率”汇总工时金额，保留两位小数。
func sumTimeEntryAmount(entries []model.TimeEntry) float64 {
	total := 0.0
	for _, e := range entries {
		total += e.Amount()
	}
	return round2(total)
}

// dedupeIDs 去重，避免指定工时重复结算校验误判。
func dedupeIDs(ids []uint64) []uint64 {
	if len(ids) == 0 {
		return ids
	}
	seen := make(map[uint64]struct{}, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// MarkPaid 标记支付（pending -> paid）。
func (s *BillingService) MarkPaid(id uint64) (*model.Billing, error) {
	b, err := s.repo.FindByID(id)
	if err != nil {
		return nil, util.Wrap(err, "Billing[id=%d] paid find failed", id)
	}
	if b.Status != constants.BillingStatusPending {
		s.logger.Warn(constants.LogBillingPaidFailed, "billing_id", id, "status", b.Status)
		return nil, util.NewAppError(constants.CodeBillingStatusConflict, "Billing[id="+u64(id)+"] paid failed: status="+b.Status)
	}
	b.Status = constants.BillingStatusPaid
	if err := s.repo.Update(b); err != nil {
		return nil, util.Wrap(err, "Billing[id=%d] paid save failed", id)
	}
	s.logger.Info(constants.LogBillingPaidSuccess, "billing_id", b.ID)
	return b, nil
}

// MarkInvoiced 开票（paid -> invoiced）。
func (s *BillingService) MarkInvoiced(id uint64, invoiceInfo string) (*model.Billing, error) {
	b, err := s.repo.FindByID(id)
	if err != nil {
		return nil, util.Wrap(err, "Billing[id=%d] invoiced find failed", id)
	}
	if b.Status != constants.BillingStatusPaid {
		return nil, util.NewAppError(constants.CodeBillingStatusConflict, "Billing[id="+u64(id)+"] invoiced failed: status="+b.Status)
	}
	b.Status = constants.BillingStatusInvoiced
	if invoiceInfo != "" {
		b.InvoiceInfo = invoiceInfo
	}
	if err := s.repo.Update(b); err != nil {
		return nil, util.Wrap(err, "Billing[id=%d] invoiced save failed", id)
	}
	s.logger.Info(constants.LogBillingInvoicedSuccess, "billing_id", b.ID)
	return b, nil
}

// Void 作废账单（事务：同时释放关联工时回待结算列表，保证工时只挂在一张有效收费单上）。
func (s *BillingService) Void(id uint64) (*model.Billing, error) {
	b, err := s.repo.FindByID(id)
	if err != nil {
		return nil, util.Wrap(err, "Billing[id=%d] void find failed", id)
	}
	if b.Status == constants.BillingStatusVoid {
		return nil, util.NewAppError(constants.CodeBillingStatusConflict, "Billing[id="+u64(id)+"] void failed: already void")
	}
	b.Status = constants.BillingStatusVoid
	if err := s.repo.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.UpdateTx(tx, b); err != nil {
			return fmt.Errorf("void billing: %w", err)
		}
		if err := s.timeEntryRepo.ReleaseByBilling(tx, id); err != nil {
			return fmt.Errorf("release time entries of billing %d: %w", id, err)
		}
		return nil
	}); err != nil {
		return nil, util.Wrap(err, "Billing[id=%d] void save failed", id)
	}
	s.logger.Info(constants.LogBillingVoidSuccess, "billing_id", b.ID)
	return b, nil
}

// List 分页查询账单。
func (s *BillingService) List(page, pageSize int, caseID, clientID uint64, status string) ([]model.Billing, int64, error) {
	return s.repo.List(page, pageSize, caseID, clientID, status)
}

// ListByCase 查询某案件账单。
func (s *BillingService) ListByCase(caseID uint64) ([]model.Billing, error) {
	return s.repo.ListByCase(caseID)
}

// Summary 本月应收/已收/待收汇总。
func (s *BillingService) Summary() (map[string]float64, error) {
	sum, err := s.repo.Summary(time.Now())
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogBillingSummary, "summary", fmt.Sprintf("%v", sum))
	return sum, nil
}

func genBillNo() string {
	return fmt.Sprintf("BILL%d%06d", time.Now().Year(), time.Now().UnixNano()%1000000)
}
