package transaction

import (
	"context"
	"time"

	"github.com/AgentTarik/finance-api/internal/storage"
	"go.uber.org/zap"
)

type EventPublisher interface {
	Publish(ctx context.Context, key string, v any) error
}

// Validator
type EventValidator interface {
	Validate(v any) error
}

type Worker struct {
	log              *zap.Logger
	repo             storage.TxRepo
	ch               chan storage.Transaction
	delay            time.Duration
	pub              EventPublisher // can be nil
	validator        EventValidator // can be nil
	publishTimeout   time.Duration
	maxRetries       int
	retryBaseBackoff time.Duration
}

// keeps current signature
func NewWorker(log *zap.Logger, repo storage.TxRepo, queueSize int, delay time.Duration) *Worker {
	return &Worker{
		log:              log,
		repo:             repo,
		ch:               make(chan storage.Transaction, queueSize),
		delay:            delay,
		publishTimeout:   5 * time.Second,        // default
		maxRetries:       3,                      // default
		retryBaseBackoff: 200 * time.Millisecond, // default
	}
}

// inject/adjust dependencies at runtime (follows current pattern)
func (w *Worker) SetPublisher(pub EventPublisher)   { w.pub = pub }
func (w *Worker) SetValidator(v EventValidator)     { w.validator = v }
func (w *Worker) SetPublishTimeout(d time.Duration) { w.publishTimeout = d }
func (w *Worker) SetRetry(max int, baseBackoff time.Duration) {
	w.maxRetries = max
	w.retryBaseBackoff = baseBackoff
}

func (w *Worker) Enqueue(t storage.Transaction) {
	select {
	case w.ch <- t:
	default:
		w.log.Warn("transaction queue full; dropping",
			zap.String("tx_id", t.TransactionID.String()))
	}
}

func (w *Worker) Run(ctx context.Context) {
	w.log.Info("transaction worker started")
	for {
		select {
		case <-ctx.Done():
			w.log.Info("transaction worker stopped")
			return

		case t := <-w.ch:
			// 1) simulated "processing"
			time.Sleep(w.delay)

			// 2) mark as processed and persist
			t.Status = "processed"
			if err := w.repo.UpsertTx(t); err != nil {
				w.log.Error("db upsert failed", zap.Error(err),
					zap.String("tx_id", t.TransactionID.String()))
				continue
			}
			w.log.Info("transaction processed",
				zap.String("tx_id", t.TransactionID.String()))

			// 3) build the event for Kafka
			evt := map[string]any{
				"type":      "transaction.created",
				"version":   1,
				"id":        t.TransactionID.String(),
				"user_id":   t.UserID.String(),
				"amount":    t.Amount,
				"timestamp": t.Timestamp.UTC().Format(time.RFC3339),
			}

			// 4) validate the event (schema)
			if w.validator != nil {
				if err := w.validator.Validate(evt); err != nil {
					w.log.Error("schema validation failed",
						zap.Error(err),
						zap.String("tx_id", t.TransactionID.String()))
					// here you could send to a DLQ, error outbox, etc.
					continue
				}
			}

			// 5) publish to Kafka (with timeout + retries)
			if w.pub != nil {
				ctxPub, cancel := context.WithTimeout(ctx, w.publishTimeout)

				var err error
				for attempt := 0; attempt <= w.maxRetries; attempt++ {
					w.log.Info("kafka publish attempt",
						zap.Int("attempt", attempt+1),
						zap.String("tx_id", t.TransactionID.String()))
					err = w.pub.Publish(ctxPub, t.TransactionID.String(), evt)
					if err == nil {
						w.log.Info("kafka published",
							zap.String("tx_id", t.TransactionID.String()))
						break
					}
					// prepare backoff for next attempt
					if attempt < w.maxRetries {
						sleep := time.Duration(1<<attempt) * w.retryBaseBackoff
						w.log.Warn("kafka publish retrying",
							zap.Error(err),
							zap.Duration("sleep", sleep),
							zap.String("tx_id", t.TransactionID.String()))
						time.Sleep(sleep)
					}
				}
				cancel() // avoid leaking context

				if err != nil {
					w.log.Error("kafka publish failed permanently",
						zap.Error(err),
						zap.String("tx_id", t.TransactionID.String()))
					// optional: persist in outbox/DLQ for later reprocessing
				}
			} else {
				w.log.Warn("kafka publisher is nil; skipping publish",
					zap.String("tx_id", t.TransactionID.String()))
			}
		}
	}
}
