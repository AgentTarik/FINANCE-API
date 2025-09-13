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
	// logger
	log, _ := telemetry.NewLogger()
	defer log.Sync()

	// choose storage (Postgres if DB_DSN is set, otherwise in-memory)
	var userRepo storage.UserRepo
	var txRepo storage.TxRepo

	dsn := os.Getenv("DB_DSN")
	if dsn != "" {
		ps, err := storage.NewPostgres(dsn)
		if err != nil {
			log.Fatal("failed to connect postgres", zap.Error(err))
		}
		userRepo = ps
		txRepo = ps
		log.Info("using Postgres repository")
	} else {
		ms := storage.NewMemoryStore()
		userRepo = ms
		txRepo = ms
		log.Info("using in-memory repository")
	}

	// Read envs and create producer
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
		log.Warn("kafka producer disabled (missing env)")
	}

	// validator
	v := validator.New()
	registerCustomValidations(v)

	// create worker (async processing)
	worker := txworker.NewWorker(log, txRepo, 100, 150*time.Millisecond)

	// inject publisher into worker (if available)
	if prod != nil {
		worker.SetPublisher(prod)
	}

	var dbPing func(ctx context.Context) error
	if ps, ok := txRepo.(*storage.PostgresStore); ok {
		dbPing = ps.DB.PingContext
	} else {
		dbPing = func(ctx context.Context) error { return nil }
	}

	// http handlers
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
	}

	// gin engine
	r := gin.New()
	r.Use(gin.Recovery())

	// simple http logging middleware
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

	// routes
	api.SetupRoutes(r, h)

	// run worker loop
	ctx, cancel := context.WithCancel(context.Background())
	go worker.Run(ctx)

	// http server
	srv := &http.Server{Addr: ":8080", Handler: r}

	// start server (async)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()
	log.Info("server started on :8080")

	// graceful shutdown on SIGINT/SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	cancel()

	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("server stopped")
}
