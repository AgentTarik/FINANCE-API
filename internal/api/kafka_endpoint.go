package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/segmentio/kafka-go"
)

// KafkaMessageView is the JSON shape returned to clients.
type KafkaMessageView struct {
	Offset    int64           `json:"offset"`
	Partition int             `json:"partition"`
	Time      time.Time       `json:"time"`
	Key       string          `json:"key,omitempty"`
	Value     json.RawMessage `json:"value"`
}

func (h *Handlers) KafkaPoll(c *gin.Context) {
	brokers := os.Getenv("KAFKA_BROKERS")
	topic := os.Getenv("KAFKA_TOPIC_TRANSACTIONS")
	if brokers == "" || topic == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Kafka not configured"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 1000 {
		limit = 10
	}
	timeoutMS, _ := strconv.Atoi(c.DefaultQuery("timeout_ms", "1500"))
	if timeoutMS < 100 {
		timeoutMS = 100
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     strings.Split(brokers, ","),
		Topic:       topic,
		Partition:   0,
		MinBytes:    1e3,  // 1KB
		MaxBytes:    10e6, // 10MB
		MaxWait:     200 * time.Millisecond,
		Logger:      nil,
		ErrorLogger: nil,
	})
	defer r.Close()

	_ = r.SetOffset(kafka.FirstOffset)

	messages := make([]KafkaMessageView, 0, limit)
	for i := 0; i < limit; i++ {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			// Return partial data + error
			c.JSON(http.StatusGatewayTimeout, gin.H{
				"topic":    topic,
				"received": len(messages),
				"error":    err.Error(),
				"messages": messages,
			})
			return
		}

		view := KafkaMessageView{
			Offset:    m.Offset,
			Partition: m.Partition,
			Time:      m.Time,
		}
		if len(m.Key) > 0 {
			view.Key = string(m.Key)
		}
		// Preserve JSON if it is JSON
		if json.Valid(m.Value) {
			view.Value = json.RawMessage(m.Value)
		} else {
			b, _ := json.Marshal(string(m.Value))
			view.Value = json.RawMessage(b)
		}
		messages = append(messages, view)
	}

	c.JSON(http.StatusOK, gin.H{
		"topic":    topic,
		"count":    len(messages),
		"messages": messages,
	})
}
