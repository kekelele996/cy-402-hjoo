package router

import (
	"cylawcase/internal/middleware"

	"github.com/gin-gonic/gin"
)

// registerCaseRoutes 案件路由。
func (r *Router) registerCaseRoutes(g *gin.RouterGroup) {
	cases := g.Group("/cases")
	cases.Use(middleware.AuthRequired(r.cfg))
	cases.GET("", r.caseH.List)
	cases.GET("/:id", r.caseH.Get)
	cases.POST("", r.caseH.Create)
	cases.PUT("/:id", r.caseH.Update)
	cases.POST("/:id/status", r.caseH.ChangeStatus)
	cases.POST("/:id/assign", r.caseH.Assign)

	// 案件工时：登记、列表（可按 unbilled/billed 过滤）、待收费汇总。
	cases.POST("/:id/time-entries", r.timeEntry.Create)
	cases.GET("/:id/time-entries", r.timeEntry.ListByCase)
	cases.GET("/:id/time-summary", r.timeEntry.SummaryByCase)
}
