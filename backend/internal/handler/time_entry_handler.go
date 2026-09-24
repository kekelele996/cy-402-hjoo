package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"cylawcase/internal/constants"
	"cylawcase/internal/dto"
	"cylawcase/internal/middleware"
	"cylawcase/internal/service"
	"cylawcase/internal/util"

	"github.com/gin-gonic/gin"
)

// TimeEntryHandler 工时记录 HTTP 处理器。
type TimeEntryHandler struct {
	svc    *service.TimeEntryService
	logger *slog.Logger
}

// NewTimeEntryHandler 构造工时记录处理器。
func NewTimeEntryHandler(svc *service.TimeEntryService, logger *slog.Logger) *TimeEntryHandler {
	return &TimeEntryHandler{svc: svc, logger: logger}
}

// List 案件工时列表，?unbilled=true 时只看待结算工时。
func (h *TimeEntryHandler) List(c *gin.Context) {
	caseID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry list: invalid case id")
		return
	}
	list, err := h.svc.ListByCase(caseID, c.Query("unbilled") == "true")
	if err != nil {
		h.wrapError(c, err, "TimeEntry list failed")
		return
	}
	OK(c, list)
}

// UnbilledSummary 案件待结算工时汇总（待收费时长与预计金额）。
func (h *TimeEntryHandler) UnbilledSummary(c *gin.Context) {
	caseID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry unbilled summary: invalid case id")
		return
	}
	hours, amount, count, err := h.svc.UnbilledSummary(caseID)
	if err != nil {
		h.wrapError(c, err, "TimeEntry unbilled summary failed")
		return
	}
	OK(c, dto.UnbilledSummaryResponse{Hours: hours, Amount: amount, Count: count})
}

// Create 登记工时（承办律师记录日期、时长、工作内容和当时费率）。
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
	workDate, err := time.Parse("2006-01-02", req.WorkDate)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry create: invalid work_date")
		return
	}
	e, err := h.svc.Create(caseID, middleware.GetUserID(c), workDate, req.Hours, req.HourlyRate, req.Description)
	if err != nil {
		h.wrapError(c, err, "TimeEntry[case_id="+strconv.FormatUint(caseID, 10)+"] create failed")
		return
	}
	OKWithMessage(c, constants.MsgTimeEntryCreated, e)
}

// Delete 删除待结算工时。
func (h *TimeEntryHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "TimeEntry[id] delete: invalid id")
		return
	}
	if err := h.svc.Delete(id); err != nil {
		h.wrapError(c, err, "TimeEntry delete failed")
		return
	}
	OKWithMessage(c, constants.MsgTimeEntryDeleted, nil)
}

func (h *TimeEntryHandler) wrapError(c *gin.Context, err error, ctx string) {
	var appErr *util.AppError
	if errors.As(err, &appErr) {
		c.Set("audit_detail", appErr.Message)
		h.logger.Warn("time entry handler error", "context", ctx, "error", appErr.Error())
		Fail(c, appErrorStatus(appErr.Code), appErr.Code, appErr.Message)
		return
	}
	h.logger.Error("time entry handler error", "context", ctx, "error", err.Error())
	Fail(c, http.StatusInternalServerError, constants.CodeInternalError, constants.MsgInternalError)
}
