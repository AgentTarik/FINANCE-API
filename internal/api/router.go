package api

import (
	"github.com/AgentTarik/finance-api/telemetry"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, h *Handlers) {
	v1 := r.Group("/v1")
	{
		v1.POST("/users", h.CreateUser)
		v1.GET("/users/:id", h.GetUser)

		v1.POST("/transactions", h.CreateTransaction)
		v1.GET("/transactions", h.ListTransactions)

		v1.GET("/reports", h.Reports)
	}
	r.GET("/health", h.Health)
	r.GET("/metrics", telemetry.MetricsHandler())
}
