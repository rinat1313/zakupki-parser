package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/docextract/logsetup"
	"github.com/rinat1313/zakupki-parser/internal/extract"
)

const (
	MaxUploadBytes       = 50 << 20
	MaxBodyBytes         = 10 << 20
	ProcessingMaxSeconds = 300
	FormFileField        = "file"
)

type Result struct {
	Status        bool   `json:"status"`
	Filename      string `json:"filename"`
	Body          string `json:"body"`
	Engine        string `json:"engine"`
	Error         string `json:"error"`
	CorrelationID string `json:"correlation_id"`
}

type Extractor interface {
	Extract(ctx context.Context, filename string, data []byte, correlationID string) Result
}

type DefaultExtractor struct{}

func (DefaultExtractor) Extract(ctx context.Context, filename string, data []byte, correlationID string) Result {
	start := time.Now()
	logsetup.LogStart("Extract", "filename", filename, "correlation_id", correlationID)
	if correlationID == "" {
		correlationID = NewCorrelationID()
	}
	out := Result{Filename: filename, CorrelationID: correlationID}

	name := filepath.Base(filename)
	if name == "" || name == "." {
		name = "upload.bin"
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !extract.SupportedExtensions[ext] {
		out.Error = "unsupported extension: " + ext
		logsetup.LogDone("Extract", start, nil, "filename", filename)
		return out
	}

	tmpDir, err := os.MkdirTemp("", "docextract-*")
	if err != nil {
		out.Error = err.Error()
		logsetup.LogDone("Extract", start, err, "filename", filename)
		return out
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, name)
	if err := os.WriteFile(srcPath, data, 0o600); err != nil {
		out.Error = err.Error()
		logsetup.LogDone("Extract", start, err, "filename", filename)
		return out
	}

	txtPath := srcPath + ".out.txt"
	ch := make(chan extract.Result, 1)
	go func() {
		ch <- extract.ToTextOut(srcPath, txtPath)
	}()

	var res extract.Result
	select {
	case <-ctx.Done():
		out.Error = "processing timeout"
		logsetup.LogDone("Extract", start, ctx.Err(), "filename", filename)
		return out
	case res = <-ch:
	}

	out.Engine = res.Engine
	if res.Error != "" {
		out.Error = res.Error
		logsetup.LogDone("Extract", start, nil, "filename", filename, "engine", res.Engine)
		return out
	}

	body, err := os.ReadFile(txtPath)
	if err != nil {
		out.Error = err.Error()
		logsetup.LogDone("Extract", start, err, "filename", filename)
		return out
	}
	if len(body) > MaxBodyBytes {
		out.Error = "extracted body too large"
		logsetup.LogDone("Extract", start, nil, "filename", filename)
		return out
	}
	out.Status = true
	out.Body = string(body)
	logsetup.LogDone("Extract", start, nil, "filename", filename, "engine", res.Engine)
	return out
}

func UnsupportedExtensionErr(ext string) string {
	return "unsupported extension: " + ext
}

func NewCorrelationID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}
