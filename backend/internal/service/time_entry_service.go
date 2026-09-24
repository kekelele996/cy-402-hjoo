package service

import (
	"log/slog"
	"math"
	"time"

	"cylawcase/internal/constants"
	"cylawcase/internal/model"
	"cylawcase/internal/repository"
	"cylawcase/internal/util"
)

// TimeEntryService 工时记录业务逻辑。
type TimeEntryService struct {
	repo     *repository.TimeEntryRepository
	caseRepo *repository.CaseRepository
	logger   *slog.Logger
}

// NewTimeEntryService 构造工时记录服务。
func NewTimeEntryService(repo *repository.TimeEntryRepository, caseRepo *repository.CaseRepository,
	logger *slog.Logger) *TimeEntryService {
	return &TimeEntryService{repo: repo, caseRepo: caseRepo, logger: logger}
}

// Create 登记工时：记录日期、时长、工作内容与当时费率（费率快照，之后调整不回改）。
func (s *TimeEntryService) Create(caseID, lawyerID uint64, workDate time.Time, hours, hourlyRate float64,
	description string) (*model.TimeEntry, error) {
	if _, err := s.caseRepo.FindByID(caseID); err != nil {
		return nil, util.Wrap(err, "TimeEntry[case_id=%d] create: case not found", caseID)
	}
	if hours <= 0 || hours > 24 {
		return nil, util.NewAppError(constants.CodeValidationFailed, "TimeEntry[hours] create: must be in (0, 24]")
	}
	if hourlyRate < 0 {
		return nil, util.NewAppError(constants.CodeValidationFailed, "TimeEntry[hourly_rate] create: must be >= 0")
	}
	if description == "" {
		return nil, util.NewAppError(constants.CodeValidationFailed, "TimeEntry[description] create: required")
	}
	e := &model.TimeEntry{
		CaseID: caseID, LawyerID: lawyerID, WorkDate: workDate,
		Hours: hours, Description: description, HourlyRate: hourlyRate,
	}
	if err := s.repo.Create(e); err != nil {
		s.logger.Error(constants.LogTimeEntryCreateFailed, "error", err.Error())
		return nil, util.Wrap(err, "TimeEntry[case_id=%d] create failed", caseID)
	}
	s.logger.Info(constants.LogTimeEntryCreateSuccess, "time_entry_id", e.ID, "case_id", caseID, "lawyer_id", lawyerID)
	return e, nil
}

// ListByCase 查询案件工时，unbilledOnly 为 true 时只看待结算工时。
func (s *TimeEntryService) ListByCase(caseID uint64, unbilledOnly bool) ([]model.TimeEntry, error) {
	if _, err := s.caseRepo.FindByID(caseID); err != nil {
		return nil, util.Wrap(err, "TimeEntry[case_id=%d] list: case not found", caseID)
	}
	return s.repo.ListByCase(caseID, unbilledOnly)
}

// UnbilledSummary 案件待结算工时汇总：总时长与预计金额（按登记时费率计算）。
func (s *TimeEntryService) UnbilledSummary(caseID uint64) (hours, amount float64, count int64, err error) {
	if _, err := s.caseRepo.FindByID(caseID); err != nil {
		return 0, 0, 0, util.Wrap(err, "TimeEntry[case_id=%d] unbilled summary: case not found", caseID)
	}
	hours, amount, count, err = s.repo.UnbilledSummary(caseID)
	if err != nil {
		return 0, 0, 0, err
	}
	return round2(hours), round2(amount), count, nil
}

// Delete 删除工时；已进入有效收费单的工时禁止删除。
func (s *TimeEntryService) Delete(id uint64) error {
	e, err := s.repo.FindByID(id)
	if err != nil {
		return util.Wrap(err, "TimeEntry[id=%d] delete find failed", id)
	}
	if e.BillingID != nil {
		s.logger.Warn(constants.LogTimeEntryDeleteFailed, "time_entry_id", id, "billing_id", *e.BillingID)
		return util.NewAppError(constants.CodeTimeEntryAlreadyBilled, "TimeEntry[id="+u64(id)+"] delete failed: already billed")
	}
	if err := s.repo.Delete(id); err != nil {
		s.logger.Error(constants.LogTimeEntryDeleteFailed, "error", err.Error())
		return util.Wrap(err, "TimeEntry[id=%d] delete failed", id)
	}
	s.logger.Info(constants.LogTimeEntryDeleteSuccess, "time_entry_id", id)
	return nil
}

// round2 保留两位小数。
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
