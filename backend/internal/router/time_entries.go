package router

import (
	"cylawcase/internal/middleware"

	"github.com/gin-gonic/gin"
)

// registerTimeEntryRoutes 工时记录路由。
func (r *Router) registerTimeEntryRoutes(g *gin.RouterGroup) {
	caseEntries := g.Group("/cases")
	caseEntries.Use(middleware.AuthRequired(r.cfg))
	caseEntries.GET("/:id/time-entries", r.timeEntry.List)
	caseEntries.GET("/:id/time-entries/unbilled-summary", r.timeEntry.UnbilledSummary)
	caseEntries.POST("/:id/time-entries", r.timeEntry.Create)

	timeEntries := g.Group("/time-entries")
	timeEntries.Use(middleware.AuthRequired(r.cfg))
	timeEntries.DELETE("/:id", r.timeEntry.Delete)
}
