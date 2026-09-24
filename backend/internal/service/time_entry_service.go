package service

import (
	"errors"
	"log/slog"
	"time"

	"cylawcase/internal/constants"
	"cylawcase/internal/model"
	"cylawcase/internal/repository"
	"cylawcase/internal/util"
)

// TimeEntryService 工时业务逻辑：登记、修改、删除、查询与案件待结算汇总。
type TimeEntryService struct {
	repo     *repository.TimeEntryRepository
	caseRepo *repository.CaseRepository
	userRepo *repository.UserRepository
	logger   *slog.Logger
}

// NewTimeEntryService 构造工时服务。
func NewTimeEntryService(repo *repository.TimeEntryRepository, caseRepo *repository.CaseRepository,
	userRepo *repository.UserRepository, logger *slog.Logger) *TimeEntryService {
	return &TimeEntryService{repo: repo, caseRepo: caseRepo, userRepo: userRepo, logger: logger}
}

// Create 承办律师在案件里登记一条工时，记录当时费率快照。
// 未显式传入费率时，取承办（当前登录）律师的当前费率；旧工时不会因之后费率调整而改变。
func (s *TimeEntryService) Create(lawyerID, caseID uint64, workDate time.Time, durationMin int,
	description string, hourlyRate *float64) (*model.TimeEntry, error) {
	if _, err := s.caseRepo.FindByID(caseID); err != nil {
		return nil, util.Wrap(err, "TimeEntry[case_id=%d] create: case not found", caseID)
	}
	if _, err := s.userRepo.FindByID(lawyerID); err != nil {
		return nil, util.Wrap(err, "TimeEntry[lawyer_id=%d] create: lawyer not found", lawyerID)
	}
	rate := constants.DefaultHourlyRate
	if lawyer, err := s.userRepo.FindByID(lawyerID); err == nil && lawyer.HourlyRate > 0 {
		rate = lawyer.HourlyRate
	}
	if hourlyRate != nil {
		rate = *hourlyRate
	}
	t := &model.TimeEntry{
		CaseID:      caseID,
		LawyerID:    lawyerID,
		WorkDate:    workDate,
		DurationMin: durationMin,
		Description: description,
		HourlyRate:  rate,
	}
	if err := s.repo.Create(t); err != nil {
		s.logger.Error(constants.LogTimeEntryCreateFailed, "error", err.Error())
		return nil, util.Wrap(err, "TimeEntry[case_id=%d] create failed", caseID)
	}
	s.logger.Info(constants.LogTimeEntryCreateSuccess, "time_entry_id", t.ID,
		"case_id", caseID, "lawyer_id", lawyerID, "hourly_rate", rate)
	return t, nil
}

// Update 修改工时。已进入有效收费单的工时不允许修改，避免与已开金额不一致。
func (s *TimeEntryService) Update(id uint64, workDate *time.Time, durationMin *int,
	description *string, hourlyRate *float64) (*model.TimeEntry, error) {
	t, err := s.repo.FindByID(id)
	if err != nil {
		return nil, util.Wrap(err, "TimeEntry[id=%d] update find failed", id)
	}
	if t.BillingID != nil {
		return nil, util.NewAppError(constants.CodeTimeEntryBilled,
			"TimeEntry[id="+u64(id)+"] update failed: already attached to billing_id="+u64(*t.BillingID))
	}
	if workDate != nil {
		t.WorkDate = *workDate
	}
	if durationMin != nil {
		t.DurationMin = *durationMin
	}
	if description != nil {
		t.Description = *description
	}
	if hourlyRate != nil {
		t.HourlyRate = *hourlyRate
	}
	if err := s.repo.Update(t); err != nil {
		return nil, util.Wrap(err, "TimeEntry[id=%d] update save failed", id)
	}
	s.logger.Info(constants.LogTimeEntryUpdateSuccess, "time_entry_id", id)
	return t, nil
}

// Delete 删除工时。已进入有效收费单的工时不允许删除。
func (s *TimeEntryService) Delete(id uint64) error {
	t, err := s.repo.FindByID(id)
	if err != nil {
		return util.Wrap(err, "TimeEntry[id=%d] delete find failed", id)
	}
	if t.BillingID != nil {
		return util.NewAppError(constants.CodeTimeEntryBilled,
			"TimeEntry[id="+u64(id)+"] delete failed: already attached to billing_id="+u64(*t.BillingID))
	}
	if err := s.repo.Delete(id); err != nil {
		return util.Wrap(err, "TimeEntry[id=%d] delete failed", id)
	}
	s.logger.Info(constants.LogTimeEntryDeleteSuccess, "time_entry_id", id)
	return nil
}

// ListByCase 查询案件工时，可按 unbilled/billed 过滤。
func (s *TimeEntryService) ListByCase(caseID uint64, statusFilter string) ([]model.TimeEntry, error) {
	if _, err := s.caseRepo.FindByID(caseID); err != nil {
		return nil, util.Wrap(err, "TimeEntry[case_id=%d] list: case not found", caseID)
	}
	list, err := s.repo.ListByCase(caseID, normalizeTimeEntryFilter(statusFilter))
	if err != nil {
		return nil, util.Wrap(err, "TimeEntry[case_id=%d] list failed", caseID)
	}
	return list, nil
}

// ListByBilling 查询一张收费单的收费来源工时。
func (s *TimeEntryService) ListByBilling(billingID uint64) ([]model.TimeEntry, error) {
	list, err := s.repo.ListByBilling(billingID)
	if err != nil {
		return nil, util.Wrap(err, "TimeEntry[billing_id=%d] list sources failed", billingID)
	}
	return list, nil
}

// UnbilledSummary 案件待收费工时汇总：总分钟数、总小时数、预计金额。
func (s *TimeEntryService) UnbilledSummary(caseID uint64) (minutes int64, hours float64, amount float64, err error) {
	if _, e := s.caseRepo.FindByID(caseID); e != nil {
		if errors.Is(e, repository.ErrNotFound) {
			return 0, 0, 0, util.Wrap(e, "TimeEntry[case_id=%d] summary: case not found", caseID)
		}
		return 0, 0, 0, util.Wrap(e, "TimeEntry[case_id=%d] summary find case failed", caseID)
	}
	minutes, amount, err = s.repo.UnbilledSummary(caseID)
	if err != nil {
		return 0, 0, 0, util.Wrap(err, "TimeEntry[case_id=%d] summary failed", caseID)
	}
	return minutes, float64(minutes) / 60.0, amount, nil
}

// ParseWorkDate 解析工时日期（YYYY-MM-DD）。
func ParseWorkDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func normalizeTimeEntryFilter(f string) string {
	switch f {
	case repository.TimeEntryFilterUnbilled, repository.TimeEntryFilterBilled:
		return f
	default:
		return repository.TimeEntryFilterAll
	}
}
