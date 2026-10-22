package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/docparser/channels"
	"github.com/rinat1313/zakupki-parser/internal/docparser/config"
	"github.com/rinat1313/zakupki-parser/internal/docparser/core"
)

type fakeExtractor struct {
	res core.Result
}

func (f fakeExtractor) Extract(ctx context.Context, filename string, data []byte, correlationID string) core.Result {
	r := f.res
	if r.Filename == "" {
		r.Filename = filename
	}
	if r.CorrelationID == "" {
		r.CorrelationID = correlationID
	}
	return r
}

func testServer(ch channels.State) *Server {
	cfg := config.Config{APIKey: "test-key", HTTPAddr: ":8094"}
	store := channels.NewStore(ch, func() bool { return true })
	return New(cfg, store, core.DefaultExtractor{})
}

func authReq(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Authorization", "Bearer test-key")
	return req
}

func TestHealthNoAuth(t *testing.T) {
	s := testServer(channels.State{HTTPIngest: true})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestExtractUnauthorized(t *testing.T) {
	s := testServer(channels.State{HTTPIngest: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/extract", nil)
	rec := httptest.NewRecorder()
	s.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestExtractHTTPIngestOff(t *testing.T) {
	s := testServer(channels.State{HTTPIngest: false})
	rec := doUpload(s, "a.txt", "hello world text content")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestExtractUnsupported(t *testing.T) {
	s := testServer(channels.State{HTTPIngest: true})
	rec := doUpload(s, "x.gif", "data")
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestExtractSuccess(t *testing.T) {
	s := testServer(channels.State{HTTPIngest: true})
	content := "Это нормальный текст документа для проверки извлечения содержимого файла."
	rec := doUpload(s, "sample.txt", content)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got core.Result
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !got.Status {
		t.Fatalf("got %#v", got)
	}
}

func TestExtractTimeoutStatus(t *testing.T) {
	cfg := config.Config{APIKey: "test-key"}
	store := channels.NewStore(channels.State{HTTPIngest: true}, func() bool { return true })
	s := New(cfg, store, fakeExtractor{res: core.Result{Error: "processing timeout"}})
	rec := doUpload(s, "sample.txt", "hello")
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestExtractEngineErrorUnprocessable(t *testing.T) {
	cfg := config.Config{APIKey: "test-key"}
	store := channels.NewStore(channels.State{HTTPIngest: true}, func() bool { return true })
	s := New(cfg, store, fakeExtractor{res: core.Result{Error: "extract failed", Status: false}})
	rec := doUpload(s, "sample.txt", "hello")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestChannelsPutHTTPAlias(t *testing.T) {
	s := testServer(channels.State{HTTPIngest: true})
	body := strings.NewReader(`{"http":false,"kafka":false}`)
	req := authReq(http.MethodPut, "/api/v1/channels", body)
	rec := httptest.NewRecorder()
	s.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if s.channels.Get().HTTPIngest {
		t.Fatal("http should be off")
	}
}

func TestChannelsPutKafkaNoBrokers(t *testing.T) {
	cfg := config.Config{APIKey: "test-key"}
	store := channels.NewStore(channels.State{HTTPIngest: true}, func() bool { return false })
	s := New(cfg, store, core.DefaultExtractor{})
	body := strings.NewReader(`{"http_ingest":true,"kafka_ingest":true}`)
	req := authReq(http.MethodPut, "/api/v1/channels", body)
	rec := httptest.NewRecorder()
	s.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestServerTimeouts(t *testing.T) {
	_, read, write, idle := ServerTimeouts()
	min := time.Duration(core.ProcessingMaxSeconds) * time.Second
	if read < min || write < min || idle < min {
		t.Fatalf("timeouts")
	}
}

func doUpload(s *Server, filename, content string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(core.FormFileField, filepath.Base(filename))
	if err != nil {
		panic(err)
	}
	_, _ = io.WriteString(part, content)
	_ = w.Close()
	req := authReq(http.MethodPost, "/api/v1/extract", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	s.Mux.ServeHTTP(rec, req)
	return rec
}
