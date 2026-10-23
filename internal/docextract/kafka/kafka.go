package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/docextract/channels"
	"github.com/rinat1313/zakupki-parser/internal/docextract/core"
	"github.com/rinat1313/zakupki-parser/internal/docextract/logsetup"
	"github.com/segmentio/kafka-go"
)

type InMessage struct {
	CorrelationID string `json:"correlation_id"`
	Filename      string `json:"filename"`
	ContentBase64 string `json:"content_base64"`
}

type OutMessage struct {
	CorrelationID string `json:"correlation_id"`
	Filename      string `json:"filename"`
	Status        bool   `json:"status"`
	Body          string `json:"body"`
	Engine        string `json:"engine"`
	Error         string `json:"error"`
}

type MessageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type MessageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type Runner struct {
	reader    MessageReader
	writer    MessageWriter
	channels  *channels.Store
	extractor core.Extractor
	timeout   time.Duration
}

func NewRunner(reader MessageReader, writer MessageWriter, ch *channels.Store, ex core.Extractor) *Runner {
	return &Runner{
		reader:    reader,
		writer:    writer,
		channels:  ch,
		extractor: ex,
		timeout:   time.Duration(core.ProcessingMaxSeconds) * time.Second,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	start := time.Now()
	logsetup.LogStart("KafkaRunner.Run")
	var runErr error
	defer func() {
		logsetup.LogDone("KafkaRunner.Run", start, runErr)
	}()
	for {
		if ctx.Err() != nil {
			runErr = ctx.Err()
			return runErr
		}
		if !r.channels.Get().KafkaIngest {
			if err := sleepContext(ctx, time.Second); err != nil {
				runErr = err
				return runErr
			}
			continue
		}
		msg, err := r.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				runErr = ctx.Err()
				return runErr
			}
			slog.Warn("kafka fetch failed", "err", err)
			if err := sleepContext(ctx, time.Second); err != nil {
				runErr = err
				return runErr
			}
			continue
		}
		if err := r.handleMessage(ctx, msg); err != nil {
			slog.Error("kafka handle failed", "err", err)
		}
		if err := r.reader.CommitMessages(ctx, msg); err != nil {
			slog.Warn("kafka commit failed", "err", err)
		}
	}
}

func (r *Runner) handleMessage(ctx context.Context, msg kafka.Message) error {
	start := time.Now()
	logsetup.LogStart("KafkaRunner.handleMessage")
	var err error
	defer func() {
		logsetup.LogDone("KafkaRunner.handleMessage", start, err)
	}()

	var in InMessage
	if err = json.Unmarshal(msg.Value, &in); err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(in.ContentBase64)
	if err != nil {
		return err
	}
	if in.CorrelationID == "" {
		in.CorrelationID = core.NewCorrelationID()
	}
	procCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	res := r.extractor.Extract(procCtx, in.Filename, data, in.CorrelationID)
	out := OutMessage{
		CorrelationID: res.CorrelationID,
		Filename:      res.Filename,
		Status:        res.Status,
		Body:          res.Body,
		Engine:        res.Engine,
		Error:         res.Error,
	}
	payload, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return r.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(out.CorrelationID),
		Value: payload,
	})
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func NewReader(brokers []string, topic, groupID string) MessageReader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: groupID,
	})
}

func NewWriter(brokers []string, topic string) MessageWriter {
	return &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
}

func ValidateEnabled(enabled bool, brokers []string) error {
	if !enabled {
		return nil
	}
	if len(brokers) == 0 {
		return errors.New("KAFKA_ENABLED requires KAFKA_BROKERS")
	}
	return nil
}
