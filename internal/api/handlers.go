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
}

// health handler
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

func (h *Handlers) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.BindJSON(&req); err != nil {
		telemetry.IncUsersCreateFailed("validation")
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	if err := h.V.Struct(req); err != nil {
		telemetry.IncUsersCreateFailed("validation")
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	id, _ := uuid.Parse(req.ID)
	if err := h.Users.CreateUser(storage.User{ID: id, Name: req.Name}); err != nil {
		status := http.StatusInternalServerError
		if err == storage.ErrUserAlreadyExists {
			telemetry.IncUsersCreateFailed("conflict")
			status = http.StatusConflict
		} else {
			telemetry.IncUsersCreateFailed("db")
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	telemetry.IncUsersCreated()
	c.JSON(http.StatusCreated, UserResponse{ID: req.ID, Name: req.Name})
}

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

// transactions handler

func (h *Handlers) CreateTransaction(c *gin.Context) {
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
	userID, _ := uuid.Parse(req.UserID)
	ts, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid timestamp (RFC3339 expected)"})
		return
	}

	// store as queued
	t := storage.Transaction{
		TransactionID: txID,
		UserID:        userID,
		Amount:        req.Amount,
		Timestamp:     ts,
		Status:        "queued",
	}
	if err := h.TxRepo.UpsertTx(t); err != nil {
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
