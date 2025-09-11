package transaction

import (
	"context"
	"time"

	"github.com/AgentTarik/finance-api/internal/storage"
	"go.uber.org/zap"
)

type Worker struct {
	log   *zap.Logger
	repo  storage.TxRepo
	ch    chan storage.Transaction
	delay time.Duration // simulates writing/processing time
}

func NewWorker(log *zap.Logger, repo storage.TxRepo, queueSize int, delay time.Duration) *Worker {
	return &Worker{
		log:   log,
		repo:  repo,
		ch:    make(chan storage.Transaction, queueSize),
		delay: delay,
	}
}


func (w *Worker) Enqueue(t storage.Transaction) {
	select {
	case w.ch <- t:
		//ok
	default:
		// queue Full — status 429
		w.log.Warn("transaction queue full; packet dropping", zap.String("tx_id", t.TransactionID.String()))
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
			time.Sleep(w.delay)
			t.Status = "processed"
			if err := w.repo.UpsertTx(t); err!= nil {
				w.log.Error("failed to upsert tx", zap.Error(err))
				continue
			}
			w.log.Info("transaction processed", zap.String("tx_id", t.TransactionID.String()))
		}
	}
}

