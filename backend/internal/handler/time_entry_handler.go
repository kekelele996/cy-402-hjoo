package handler

import (
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"cylawcase/internal/constants"
	"cylawcase/internal/dto"
	"cylawcase/internal/middleware"
	"cylawcase/internal/repository"
	"cylawcase/internal/service"
	"cylawcase/internal/util"

	"github.com/gin-gonic/gin"
)

// TimeEntryHandler 工时 HTTP 处理器。
type TimeEntryHandler struct {
	svc    *service.TimeEntryService
	logger *slog.Logger
}

// NewTimeEntryHandler 构造工时处理器。
func NewTimeEntryHandler(svc *service.TimeEntryService, logger *slog.Logger) *TimeEntryHandler {
	return &TimeEntryHandler{svc: svc, logger: logger}
}

// Create 承办律师登记工时（案件作用域路由 POST /cases/:id/time-entries）。
func (h *TimeEntryHandler) Create(c *gin.Context) {
	caseID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry create: invalid case id")
		return
	}
	var req dto.TimeEntryCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry create: "+err.Error())
		return
	}
	workDate, err := service.ParseWorkDate(req.WorkDate)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry create: invalid work_date, expect YYYY-MM-DD")
		return
	}
	t, err := h.svc.Create(middleware.GetUserID(c), caseID, workDate, req.DurationMin, req.Description, req.HourlyRate)
	if err != nil {
		h.wrapError(c, err, "TimeEntry[case_id="+strconv.FormatUint(caseID, 10)+"] create failed")
		return
	}
	OKWithMessage(c, constants.MsgTimeEntryCreated, t)
}

// Update 修改工时。
func (h *TimeEntryHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry[id] update: invalid id")
		return
	}
	var req dto.TimeEntryUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry[id="+strconv.FormatUint(id, 10)+"] update: "+err.Error())
		return
	}
	var workDatePtr *time.Time
	if req.WorkDate != nil {
		wd, perr := service.ParseWorkDate(*req.WorkDate)
		if perr != nil {
			Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry update: invalid work_date, expect YYYY-MM-DD")
			return
		}
		workDatePtr = &wd
	}
	t, err := h.svc.Update(id, workDatePtr, req.DurationMin, req.Description, req.HourlyRate)
	if err != nil {
		h.wrapError(c, err, "TimeEntry[id="+strconv.FormatUint(id, 10)+"] update failed")
		return
	}
	OKWithMessage(c, constants.MsgTimeEntryUpdated, t)
}

// Delete 删除工时。
func (h *TimeEntryHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry[id] delete: invalid id")
		return
	}
	if err := h.svc.Delete(id); err != nil {
		h.wrapError(c, err, "TimeEntry[id="+strconv.FormatUint(id, 10)+"] delete failed")
		return
	}
	OKWithMessage(c, constants.MsgTimeEntryDeleted, nil)
}

// ListByCase 某案件工时列表（支持 status=unbilled/billed）。
func (h *TimeEntryHandler) ListByCase(c *gin.Context) {
	caseID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry list: invalid case id")
		return
	}
	list, err := h.svc.ListByCase(caseID, c.Query("status"))
	if err != nil {
		h.wrapError(c, err, "TimeEntry list by case failed")
		return
	}
	OK(c, list)
}

// ListByBilling 某张收费单的收费来源工时（账单页展开查看）。
func (h *TimeEntryHandler) ListByBilling(c *gin.Context) {
	billingID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry sources: invalid billing id")
		return
	}
	list, err := h.svc.ListByBilling(billingID)
	if err != nil {
		h.wrapError(c, err, "TimeEntry list by billing failed")
		return
	}
	OK(c, list)
}

// SummaryByCase 某案件待收费工时汇总（总分钟/小时数/预计金额）。
func (h *TimeEntryHandler) SummaryByCase(c *gin.Context) {
	caseID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry summary: invalid case id")
		return
	}
	minutes, hours, amount, err := h.svc.UnbilledSummary(caseID)
	if err != nil {
		h.wrapError(c, err, "TimeEntry summary failed")
		return
	}
	OK(c, gin.H{"case_id": caseID, "unbilled_minutes": minutes,
		"unbilled_hours": hours, "estimated_amount": math.Round(amount*100) / 100})
}

func (h *TimeEntryHandler) wrapError(c *gin.Context, err error, ctx string) {
	var appErr *util.AppError
	if errors.As(err, &appErr) {
		c.Set("audit_detail", appErr.Message)
		h.logger.Warn("time entry handler error", "context", ctx, "error", appErr.Error())
		Fail(c, appErrorStatus(appErr.Code), appErr.Code, appErr.Message)
		return
	}
	if errors.Is(err, repository.ErrNotFound) {
		Fail(c, http.StatusNotFound, constants.CodeNotFound, constants.MsgNotFound)
		return
	}
	h.logger.Error("time entry handler error", "context", ctx, "error", err.Error())
	Fail(c, http.StatusInternalServerError, constants.CodeInternalError, constants.MsgInternalError)
}
