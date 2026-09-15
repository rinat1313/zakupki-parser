package extractapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/extract"
)

const (
	MaxUploadBytes       = 50 << 20
	MaxBodyBytes         = 10 << 20
	ProcessingMaxSeconds = 300
	FormFileField        = "file"
)

type Response struct {
	Status   bool   `json:"status"`
	Filename string `json:"filename"`
	Body     string `json:"body"`
}

type extractOutcome struct {
	res  extract.Result
	body []byte
	err  error
}

func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setLongRunningHeaders(w)
		r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes+1<<20)
		if err := r.ParseMultipartForm(MaxUploadBytes); err != nil {
			writeExtractJSON(w, http.StatusBadRequest, Response{})
			return
		}
		file, hdr, err := r.FormFile(FormFileField)
		if err != nil {
			writeExtractJSON(w, http.StatusBadRequest, Response{})
			return
		}
		defer file.Close()

		name := filepath.Base(hdr.Filename)
		if name == "." || name == "" || name == string(filepath.Separator) {
			name = "upload.bin"
		}
		ext := strings.ToLower(filepath.Ext(name))
		if !extract.SupportedExtensions[ext] {
			writeExtractJSON(w, http.StatusOK, Response{Filename: name})
			return
		}

		tmpDir, err := os.MkdirTemp("", "zakupki-extract-*")
		if err != nil {
			writeExtractJSON(w, http.StatusOK, Response{Filename: name})
			return
		}
		defer os.RemoveAll(tmpDir)

		srcPath := filepath.Join(tmpDir, name)
		dst, err := os.Create(srcPath)
		if err != nil {
			writeExtractJSON(w, http.StatusOK, Response{Filename: name})
			return
		}
		if _, err := io.Copy(dst, file); err != nil {
			dst.Close()
			writeExtractJSON(w, http.StatusOK, Response{Filename: name})
			return
		}
		if err := dst.Close(); err != nil {
			writeExtractJSON(w, http.StatusOK, Response{Filename: name})
			return
		}

		txtPath := srcPath + ".out.txt"
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(ProcessingMaxSeconds)*time.Second)
		defer cancel()

		ch := make(chan extractOutcome, 1)
		go func() {
			res := extract.ToTextOut(srcPath, txtPath)
			if res.Error != "" {
				ch <- extractOutcome{res: res, err: errors.New(res.Error)}
				return
			}
			b, err := os.ReadFile(txtPath)
			ch <- extractOutcome{res: res, body: b, err: err}
		}()

		var out extractOutcome
		select {
		case <-ctx.Done():
			writeExtractJSON(w, http.StatusOK, Response{Filename: name})
			return
		case out = <-ch:
		}
		if out.err != nil {
			writeExtractJSON(w, http.StatusOK, Response{Filename: name})
			return
		}
		if len(out.body) > MaxBodyBytes {
			writeExtractJSON(w, http.StatusOK, Response{Filename: name})
			return
		}
		writeExtractJSON(w, http.StatusOK, Response{
			Status:   true,
			Filename: name,
			Body:     string(out.body),
		})
	}
}

func setLongRunningHeaders(w http.ResponseWriter) {
	sec := strconv.Itoa(ProcessingMaxSeconds)
	w.Header().Set("X-Processing-Max-Seconds", sec)
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Keep-Alive", "timeout="+sec)
}

func writeExtractJSON(w http.ResponseWriter, code int, v Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func ServerTimeouts() (readHeader, read, write, idle time.Duration) {
	return 10 * time.Second,
		time.Duration(ProcessingMaxSeconds+60) * time.Second,
		time.Duration(ProcessingMaxSeconds+60) * time.Second,
		time.Duration(ProcessingMaxSeconds+60) * time.Second
}

func ShutdownTimeout() time.Duration {
	return time.Duration(ProcessingMaxSeconds+60) * time.Second
}
