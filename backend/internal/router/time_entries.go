package router

import (
	"cylawcase/internal/middleware"

	"github.com/gin-gonic/gin"
)

// registerTimeEntryRoutes 工时路由。
func (r *Router) registerTimeEntryRoutes(g *gin.RouterGroup) {
	entries := g.Group("/time-entries")
	entries.Use(middleware.AuthRequired(r.cfg))
	entries.POST("", r.timeEntry.Create)
	entries.PUT("/:id", r.timeEntry.Update)
	entries.DELETE("/:id", r.timeEntry.Delete)
	// 按收费单查询收费来源（账单页展开查看）。
	entries.GET("/by-billing/:id", r.timeEntry.ListByBilling)
}
