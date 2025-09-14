package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AgentTarik/finance-api/internal/api"
	authpkg "github.com/AgentTarik/finance-api/internal/auth"
	kafkapkg "github.com/AgentTarik/finance-api/internal/kafka"
	"github.com/AgentTarik/finance-api/internal/storage"
	txworker "github.com/AgentTarik/finance-api/internal/transaction"
	"github.com/AgentTarik/finance-api/telemetry"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// registerCustomValidations adds custom validators to the validator instance.
func registerCustomValidations(v *validator.Validate) {
	_ = v.RegisterValidation("uuid4", func(fl validator.FieldLevel) bool {
		s := fl.Field().String()
		id, err := uuid.Parse(s)
		if err != nil {
			return false
		}
		// must be version 4
		return id.Version() == 4
	})
}

func main() {
	// Logger
	log, _ := telemetry.NewLogger()
	defer log.Sync()

	// Metrics
	// registers collectors and sets up middleware/endpoint later
	telemetry.InitMetrics()

	// Database
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN is required (e.g., postgres://postgres:admin@postgres:5432/finance?sslmode=disable)")
	}
	ps, err := storage.NewPostgres(dsn)
	if err != nil {
		log.Fatal("failed to connect to Postgres", zap.Error(err))
	}
	userRepo := ps
	txRepo := ps
	log.Info("using Postgres repository")

	// Kafka Producer
	brokersCSV := os.Getenv("KAFKA_BROKERS")
	topic := os.Getenv("KAFKA_TOPIC_TRANSACTIONS")

	var prod *kafkapkg.Producer
	if brokersCSV != "" && topic != "" {
		brokers := strings.Split(brokersCSV, ",")
		prod = kafkapkg.NewProducer(brokers, topic)
		log.Info("kafka producer enabled",
			zap.Strings("brokers", brokers),
			zap.String("topic", topic),
		)
		defer prod.Close()
	} else {
		log.Warn("kafka producer disabled (missing KAFKA_BROKERS or KAFKA_TOPIC_TRANSACTIONS)")
	}

	// HTTP payload validator (Gin binding + go-playground/validator)
	v := validator.New()
	registerCustomValidations(v)

	// Async worker (processing + Kafka publish)
	worker := txworker.NewWorker(log, txRepo, 100, 150*time.Millisecond)

	// Inject Kafka publisher
	if prod != nil {
		worker.SetPublisher(prod)
	}

	// Event JSON schema validator
	if evVal, err := kafkapkg.NewValidator(); err != nil {
		log.Fatal("schema validator init failed", zap.Error(err))
	} else {
		worker.SetValidator(evVal)
	}

	// DB health function for /health handler.
	dbPing := ps.DB.PingContext

	issuer, err := authpkg.NewJWTIssuerFromEnv()
	if err != nil {
		log.Fatal("jwt init failed (set JWT_SECRET)", zap.Error(err))
	}

	authH := &api.AuthHandlers{
		Log:     log,
		UsersDB: ps,
		V:       v,
		Tokens:  issuer,
	}
	// HTTP handlers
	h := &api.Handlers{
		Log:          log,
		Users:        userRepo,
		TxRepo:       txRepo,
		V:            v,
		DBPing:       dbPing,
		KafkaEnabled: prod != nil,
		Enqueue: func(t storage.Transaction) {
			worker.Enqueue(t)
		},
		Auth: authH,
	}

	// Gin engine
	r := gin.New()
	r.Use(gin.Recovery())

	// Prometheus HTTP metrics middleware
	r.Use(telemetry.PrometheusMiddleware())

	// Simple structured HTTP log middleware
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

	// App routes.
	api.SetupRoutes(r, h)

	// Expose Prometheus metrics endpoint

	// Run worker and HTTP server
	ctx, cancel := context.WithCancel(context.Background())
	go worker.Run(ctx)

	srv := &http.Server{Addr: ":8080", Handler: r}

	// Start server asynchronously.
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()
	log.Info("server started on :8080")

	// Graceful shutdown on SIGINT/SIGTERM.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	cancel()

	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("server stopped")
}
