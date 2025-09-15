package api

import (
	"context"
	"net/http"
	"time"

	"github.com/AgentTarik/finance-api/internal/storage"
	"github.com/AgentTarik/finance-api/telemetry"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Handlers struct {
	Log          *zap.Logger
	Users        storage.UserRepo
	TxRepo       storage.TxRepo
	V            *validator.Validate
	DBPing       func(ctx context.Context) error
	KafkaEnabled bool

	// Enqueuer function (send to worker)
	Enqueue func(storage.Transaction)
	Auth    *AuthHandlers
}

// Health godoc
// @Summary      Health check
// @Tags         health
// @Success      200  {object}  map[string]any
// @Router       /health [get]
func (h *Handlers) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
	defer cancel()

	db := "ok"
	if h.DBPing != nil {
		if err := h.DBPing(ctx); err != nil {
			db = "down"
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"db":            db,
		"kafka_enabled": h.KafkaEnabled,
	})
}

// user handler

func (h *Handlers) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		telemetry.IncUsersGet(false)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	u, err := h.Users.GetUser(id)
	if err != nil {
		status := http.StatusInternalServerError
		if err == storage.ErrUserNotFound {
			status = http.StatusNotFound
		}
		telemetry.IncUsersGet(false)
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	telemetry.IncUsersGet(true)
	c.JSON(http.StatusOK, UserResponse{ID: u.ID.String(), Name: u.Name})
}

// CreateTransaction godoc
// @Summary      Create a transaction
// @Description  Enqueues a transaction; user is taken from JWT.
// @Tags         transactions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access token>"
// @Param        payload  body      CreateTransactionRequest  true  "Transaction payload"
// @Success      202      {object}  map[string]string
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Failure      422      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /transactions [post]
func (h *Handlers) CreateTransaction(c *gin.Context) {

	// Extract authenticated user from context (set by JWT middleware).
	uidVal, ok := c.Get("user_id")
	if !ok || uidVal == nil {
		// Should not happen because the route is protected, but guard anyway.
		telemetry.IncTransactionsFailed("validation")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing auth context"})
		return
	}
	authUserIDStr, _ := uidVal.(string)
	authUserID, err := uuid.Parse(authUserIDStr)
	if err != nil {
		telemetry.IncTransactionsFailed("validation")
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid auth subject"})
		return
	}

	var req CreateTransactionRequest
	if err := c.BindJSON(&req); err != nil {
		telemetry.IncTransactionsFailed("validation")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	if err := h.V.Struct(req); err != nil {
		telemetry.IncTransactionsFailed("validation")
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	txID, _ := uuid.Parse(req.TransactionID)
	ts, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		telemetry.IncTransactionsFailed("validation")
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid timestamp (RFC3339 expected)"})
		return
	}

	// store as queued
	t := storage.Transaction{
		TransactionID: txID,
		UserID:        authUserID,
		Amount:        req.Amount,
		Timestamp:     ts,
		Status:        "queued",
	}
	if err := h.TxRepo.UpsertTx(t); err != nil {
		telemetry.IncTransactionsFailed("db")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist"})
		return
	}

	// ennqueue for async processing
	h.Enqueue(t)

	c.JSON(http.StatusAccepted, gin.H{
		"transaction_id": req.TransactionID,
		"status":         "queued",
	})
}

// ListTransactions godoc
// @Summary      List transactions
// @Description  Lists transactions for the authenticated user.
// @Tags         transactions
// @Security     BearerAuth
// @Produce      json
// @Param        Authorization header string true "Bearer <access token>"
// @Success      200      {array}   storage.Transaction
// @Failure      401      {object}  map[string]string
// @Router       /transactions [get]
func (h *Handlers) ListTransactions(c *gin.Context) {
	txs, err := h.TxRepo.ListTx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list"})
		return
	}
	out := make([]Transaction, 0, len(txs))
	for _, t := range txs {
		out = append(out, Transaction{
			TransactionID: t.TransactionID.String(),
			UserID:        t.UserID.String(),
			Amount:        t.Amount,
			Timestamp:     t.Timestamp,
			Status:        t.Status,
		})
	}
	c.JSON(http.StatusOK, out)
}

// Reports godoc
// @Summary      Summary reports
// @Description  Aggregations for the authenticated user.
// @Tags         reports
// @Security     BearerAuth
// @Produce      json
// @Param        Authorization header string true "Bearer <access token>"
// @Success      200      {object}  map[string]any
// @Failure      401      {object}  map[string]string
// @Router       /reports [get]
func (h *Handlers) Reports(c *gin.Context) {
	// agregação simples: soma por usuário (apenas processed)
	txs, err := h.TxRepo.ListTx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list"})
		return
	}
	agg := map[string]float64{}
	for _, t := range txs {
		if t.Status == "processed" {
			agg[t.UserID.String()] += t.Amount
		}
	}
	c.JSON(http.StatusOK, gin.H{"sum_by_user": agg})
}
