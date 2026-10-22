package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/docparser/channels"
	"github.com/rinat1313/zakupki-parser/internal/docparser/core"
	"github.com/segmentio/kafka-go"
)

type fakeReader struct {
	msgs []kafka.Message
	idx  int
}

func (f *fakeReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	for {
		if f.idx >= len(f.msgs) {
			select {
			case <-ctx.Done():
				return kafka.Message{}, ctx.Err()
			case <-time.After(10 * time.Millisecond):
				continue
			}
		}
		m := f.msgs[f.idx]
		f.idx++
		return m, nil
	}
}

func (f *fakeReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	return nil
}

func (f *fakeReader) Close() error { return nil }

type fakeWriter struct {
	mu   sync.Mutex
	sent []kafka.Message
}

func (f *fakeWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	f.mu.Lock()
	f.sent = append(f.sent, msgs...)
	f.mu.Unlock()
	return nil
}

func (f *fakeWriter) Close() error { return nil }

type stubExtractor struct{}

func (stubExtractor) Extract(ctx context.Context, filename string, data []byte, correlationID string) core.Result {
	return core.Result{
		Status:        true,
		Filename:      filename,
		Body:          string(data),
		Engine:        "copy",
		CorrelationID: correlationID,
	}
}

func TestRunnerProcessesMessage(t *testing.T) {
	in := InMessage{
		CorrelationID: "corr-1",
		Filename:      "a.txt",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("hello kafka")),
	}
	payload, _ := json.Marshal(in)
	reader := &fakeReader{msgs: []kafka.Message{{Value: payload}}}
	writer := &fakeWriter{}
	ch := channels.NewStore(channels.State{KafkaIngest: true}, func() bool { return true })
	runner := NewRunner(reader, writer, ch, stubExtractor{})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = runner.Run(ctx)

	writer.mu.Lock()
	n := len(writer.sent)
	writer.mu.Unlock()
	if n != 1 {
		t.Fatalf("sent=%d", n)
	}
	var out OutMessage
	writer.mu.Lock()
	_ = json.Unmarshal(writer.sent[0].Value, &out)
	writer.mu.Unlock()
	if out.CorrelationID != "corr-1" || !out.Status {
		t.Fatalf("out=%+v", out)
	}
}

func TestValidateEnabled(t *testing.T) {
	if err := ValidateEnabled(true, nil); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateEnabled(false, nil); err != nil {
		t.Fatal(err)
	}
}
