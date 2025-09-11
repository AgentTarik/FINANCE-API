package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AgentTarik/finance-api/internal/api"
	"github.com/AgentTarik/finance-api/internal/storage"
	txworker "github.com/AgentTarik/finance-api/internal/transaction"
	"github.com/AgentTarik/finance-api/telemetry"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func registerCustomValidations(v *validator.Validate) {
	_ = v.RegisterValidation("uuid4", func(fl validator.FieldLevel) bool {
		s := fl.Field().String()
		id, err := uuid.Parse(s)
		if err != nil {
			return false
		}
		// v4
		return id.Version() == 4
	})
}

func main() {
	log, _ := telemetry.NewLogger()
	defer log.Sync()

	// In memory store
	store := storage.NewMemoryStore()

	// validator
	v := validator.New()
	registerCustomValidations(v)

	// worker: queue 100, 150ms delay simulating "processing"
	worker := txworker.NewWorker(log, store, 100, 150*time.Millisecond)

	// handlers with dependencies
	h := &api.Handlers{
		Log:    log,
		Users:  store,
		TxRepo: store,
		V:      v,
		Enqueue: func(t storage.Transaction) {
			worker.Enqueue(t)
		},
	}

	// gin engine
	r := gin.New()
	r.Use(gin.Recovery())

	// middleware de log http simples
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.FullPath()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("dur", time.Since(start)),
		)
	})

	api.SetupRoutes(r, h)

	// inicia worker
	ctx, cancel := context.WithCancel(context.Background())
	go worker.Run(ctx)

	srv := &http.Server{Addr: ":8080", Handler: r}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()
	log.Info("server started on :8080")

	// graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	cancel()
	ctxTimeout, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	_ = srv.Shutdown(ctxTimeout)
	log.Info("server stopped")
}
