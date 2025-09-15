package api

import (
	"github.com/AgentTarik/finance-api/internal/auth"
	"github.com/AgentTarik/finance-api/telemetry"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, h *Handlers) {
	v1 := r.Group("/v1")
	{
		v1.POST("/auth/register", h.Auth.Register)
		v1.POST("/auth/login", h.Auth.Login)
		v1.GET("/users/:id", h.GetUser)

		protected := v1.Group("/")
		protected.Use(auth.RequireAuth())

		protected.POST("/transactions", h.CreateTransaction)
		protected.GET("/transactions", h.ListTransactions)

		protected.GET("/reports", h.Reports)
		
		v1.GET("/kafka/poll", h.KafkaPoll)

		v1.GET("/health", h.Health)
	}


	r.GET("/metrics", telemetry.MetricsHandler())
}
