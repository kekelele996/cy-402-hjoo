package service

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"cylawcase/internal/constants"
	"cylawcase/internal/model"
	"cylawcase/internal/repository"
	"cylawcase/internal/util"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	return &BillingService{repo: repo, caseRepo: caseRepo, clientRepo: clientRepo,
		timeEntryRepo: timeEntryRepo, logger: logger}
}

// Create 创建账单（手工录入，来源标记为 manual）。
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
		CaseID: caseID, ClientID: clientID, InvoiceInfo: invoiceInfo,
		Source: constants.BillingSourceManual}
	if err := s.repo.Create(b); err != nil {
		s.logger.Error(constants.LogBillingCreateFailed, "error", err.Error())
		return nil, util.Wrap(err, "Billing[case_id=%d] create failed", caseID)
	}
	s.logger.Info(constants.LogBillingCreateSuccess, "billing_id", b.ID, "bill_no", b.BillNo)
	return b, nil
}

// GenerateInvoiceFromTimeEntries 为案件汇总全部未收费工时生成一张律师费收费单。
//
// 整个过程在单个数据库事务中完成：
//  1. 锁定案件行（串行化同一案件的并发结算）；
//  2. SELECT ... FOR UPDATE 锁定该案件所有 billing_id IS NULL 的工时；
//  3. 若没有任何未收费工时，整体回滚，工时仍留在待结算列表；
//  4. 按每条工时的费率快照计算金额合计，创建 pending 律师费账单；
//  5. 把这批工时的 billing_id 挂到新账单上并提交。
//
// 任一步失败都会回滚，工时不会被部分占用；一项工时因此只能进入一张有效收费单。
func (s *BillingService) GenerateInvoiceFromTimeEntries(caseID uint64, invoiceInfo string) (*model.Billing, error) {
	cs, err := s.caseRepo.FindByID(caseID)
	if err != nil {
		return nil, util.Wrap(err, "Billing[case_id=%d] invoice generate: case not found", caseID)
	}
	s.logger.Info(constants.LogInvoiceGenerateStart, "case_id", caseID)

	var billing *model.Billing
	txErr := s.timeEntryRepo.Transaction(func(tx *gorm.DB) error {
		// 锁定案件行，避免两个请求同时为同一案件结算。
		var locked model.Case
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, caseID).Error; err != nil {
			return util.Wrap(err, "Billing[case_id=%d] invoice generate: lock case failed", caseID)
		}

		entries, err := s.timeEntryRepo.ListUnbilledByCaseForUpdate(tx, caseID)
		if err != nil {
			return util.Wrap(err, "Billing[case_id=%d] invoice generate: lock time entries failed", caseID)
		}
		if len(entries) == 0 {
			return util.NewAppError(constants.CodeNoUnbilledTimeEntries,
				"Billing[case_id="+strconv.FormatUint(caseID, 10)+"] invoice generate: no unbilled time entries")
		}

		var total float64
		ids := make([]uint64, 0, len(entries))
		for i := range entries {
			ids = append(ids, entries[i].ID)
		}
		total = invoiceAmount(entries)

		b := &model.Billing{
			BillNo:      genBillNo(),
			BillingType: constants.BillingTypeAttorneyFee,
			Amount:      total,
			Status:      constants.BillingStatusPending,
			CaseID:      caseID,
			ClientID:    cs.ClientID,
			InvoiceInfo: invoiceInfo,
			Source:      constants.BillingSourceTimeEntries,
		}
		if err := s.repo.CreateTx(tx, b); err != nil {
			return util.Wrap(err, "Billing[case_id=%d] invoice generate: create billing failed", caseID)
		}

		attached, err := s.timeEntryRepo.AttachToBilling(tx, ids, b.ID)
		if err != nil {
			return util.Wrap(err, "Billing[case_id=%d] invoice generate: attach entries failed", caseID)
		}
		if attached != int64(len(ids)) {
			// 并发竞争：有工时在本事务锁定前已被别的收费单占用，整体回滚，保持工时留在待结算列表。
			return util.NewAppError(constants.CodeTimeEntryBilled,
				"Billing[case_id="+strconv.FormatUint(caseID, 10)+"] invoice generate: time entry already attached to another billing")
		}
		billing = b
		return nil
	})
	if txErr != nil {
		s.logger.Error(constants.LogInvoiceGenerateFailed, "case_id", caseID, "error", txErr.Error())
		return nil, txErr
	}
	s.logger.Info(constants.LogInvoiceGenerateSuccess, "billing_id", billing.ID,
		"bill_no", billing.BillNo, "case_id", caseID, "amount", billing.Amount)
	return billing, nil
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

// Void 作废账单。若该账单由工时汇总生成，则在同一事务中把其下工时释放回待结算列表，
// 使这些工时可以重新进入下一张有效收费单（一项工时只属于一张"有效"收费单）。
func (s *BillingService) Void(id uint64) (*model.Billing, error) {
	var result *model.Billing
	txErr := s.timeEntryRepo.Transaction(func(tx *gorm.DB) error {
		b, err := s.repo.FindByIDTx(tx, id)
		if err != nil {
			return util.Wrap(err, "Billing[id=%d] void find failed", id)
		}
		if b.Status == constants.BillingStatusVoid {
			return util.NewAppError(constants.CodeBillingStatusConflict, "Billing[id="+u64(id)+"] void failed: already void")
		}
		b.Status = constants.BillingStatusVoid
		if err := s.repo.UpdateTx(tx, b); err != nil {
			return util.Wrap(err, "Billing[id=%d] void save failed", id)
		}
		if b.Source == constants.BillingSourceTimeEntries {
			released, err := s.timeEntryRepo.ReleaseByBilling(tx, b.ID)
			if err != nil {
				return util.Wrap(err, "Billing[id=%d] void release time entries failed", id)
			}
			s.logger.Info(constants.LogInvoiceReleaseEntries, "billing_id", id, "released", released)
		}
		result = b
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	s.logger.Info(constants.LogBillingVoidSuccess, "billing_id", id)
	return result, nil
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

// round2 金额四舍五入保留两位小数。
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// invoiceAmount 按每条工时登记时的费率快照汇总收费金额（分钟 / 60 * 当时费率），两位小数。
func invoiceAmount(entries []model.TimeEntry) float64 {
	var total float64
	for i := range entries {
		total += float64(entries[i].DurationMin) / 60.0 * entries[i].HourlyRate
	}
	return round2(total)
}
