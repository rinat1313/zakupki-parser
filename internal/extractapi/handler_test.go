package extractapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServerTimeoutsCoverFiveMinutes(t *testing.T) {
	_, read, write, idle := ServerTimeouts()
	min := time.Duration(ProcessingMaxSeconds) * time.Second
	if read < min || write < min || idle < min {
		t.Fatalf("timeouts too short: read=%v write=%v idle=%v", read, write, idle)
	}
}

func TestHandlerMissingFile(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/extract", nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
	var got Response
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Status {
		t.Fatal("expected status false")
	}
	assertLongRunningHeaders(t, rec.Header())
}

func TestHandlerUnsupportedExtension(t *testing.T) {
	rec := doUpload(t, "photo.png", "not-a-real-image")
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var got Response
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Status {
		t.Fatal("expected status false")
	}
	if got.Filename != "photo.png" {
		t.Fatalf("filename=%q", got.Filename)
	}
	if got.Body != "" {
		t.Fatalf("body should be empty, got %q", got.Body)
	}
	assertLongRunningHeaders(t, rec.Header())
}

func TestHandlerTXTSuccess(t *testing.T) {
	content := "Это нормальный текст документа для проверки извлечения содержимого файла."
	rec := doUpload(t, "sample.txt", content)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got Response
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !got.Status {
		t.Fatalf("expected success, got %#v", got)
	}
	if got.Filename != "sample.txt" {
		t.Fatalf("filename=%q", got.Filename)
	}
	if !strings.Contains(got.Body, "нормальный текст документа") {
		t.Fatalf("body=%q", got.Body)
	}
	assertLongRunningHeaders(t, rec.Header())
}

func TestHandlerHTMLSuccess(t *testing.T) {
	content := "<html><body><p>Договор поставки оборудования для проверки конвертации HTML в текст документа.</p></body></html>"
	rec := doUpload(t, "notice.html", content)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got Response
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !got.Status {
		t.Fatalf("expected success, got %#v", got)
	}
	if got.Filename != "notice.html" {
		t.Fatalf("filename=%q", got.Filename)
	}
	if !strings.Contains(got.Body, "Договор поставки") {
		t.Fatalf("body=%q", got.Body)
	}
}

func doUpload(t *testing.T, filename, content string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(FormFileField, filepath.Base(filename))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/extract", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
	return rec
}

func assertLongRunningHeaders(t *testing.T, h http.Header) {
	t.Helper()
	if h.Get("X-Processing-Max-Seconds") != "300" {
		t.Fatalf("X-Processing-Max-Seconds=%q", h.Get("X-Processing-Max-Seconds"))
	}
	if h.Get("Content-Type") == "" {
		t.Fatal("missing Content-Type")
	}
}
