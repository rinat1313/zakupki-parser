package core

import (
	"context"
	"strings"
	"testing"
)

func TestExtractTXT(t *testing.T) {
	content := "Это нормальный текст документа для проверки извлечения содержимого файла."
	var ex DefaultExtractor
	res := ex.Extract(context.Background(), "sample.txt", []byte(content), "cid-1")
	if !res.Status {
		t.Fatalf("status false: %v", res)
	}
	if res.CorrelationID != "cid-1" {
		t.Fatalf("correlation=%q", res.CorrelationID)
	}
	if !strings.Contains(res.Body, "нормальный текст") {
		t.Fatalf("body=%q", res.Body)
	}
}

func TestExtractUnsupported(t *testing.T) {
	var ex DefaultExtractor
	res := ex.Extract(context.Background(), "x.gif", []byte("x"), "c")
	if res.Status {
		t.Fatal("expected failure")
	}
	if res.Error == "" {
		t.Fatal("expected error")
	}
}

func TestExtractTimeout(t *testing.T) {
	var ex DefaultExtractor
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := ex.Extract(ctx, "sample.txt", []byte("hello world document text content here"), "c")
	if res.Status {
		t.Fatal("expected timeout failure")
	}
	if res.Error == "" {
		t.Fatal("expected error message")
	}
}
