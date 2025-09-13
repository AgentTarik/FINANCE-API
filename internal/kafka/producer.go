package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	w *kafka.Writer
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		w: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Producer) Publish(ctx context.Context, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: b,
		Time:  time.Now(),
	})
}

func (p *Producer) Close() error { return p.w.Close() }
