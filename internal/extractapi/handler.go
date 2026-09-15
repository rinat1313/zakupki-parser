package extractapi

import (
	"encoding/json"
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
	MaxUploadBytes       = 64 << 20
	ProcessingMaxSeconds = 300
	FormFileField        = "file"
)

type Response struct {
	Status   bool   `json:"status"`
	Filename string `json:"filename"`
	Body     string `json:"body"`
}

func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setLongRunningHeaders(w)
		r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes+1<<20)
		if err := r.ParseMultipartForm(MaxUploadBytes); err != nil {
			writeExtractJSON(w, http.StatusBadRequest, Response{
				Status:   false,
				Filename: "",
				Body:     "",
			})
			return
		}
		file, hdr, err := r.FormFile(FormFileField)
		if err != nil {
			writeExtractJSON(w, http.StatusBadRequest, Response{
				Status:   false,
				Filename: "",
				Body:     "",
			})
			return
		}
		defer file.Close()

		name := filepath.Base(hdr.Filename)
		if name == "." || name == "" || name == string(filepath.Separator) {
			name = "upload.bin"
		}
		ext := strings.ToLower(filepath.Ext(name))
		if !extract.SupportedExtensions[ext] {
			writeExtractJSON(w, http.StatusOK, Response{
				Status:   false,
				Filename: name,
				Body:     "",
			})
			return
		}

		tmpDir, err := os.MkdirTemp("", "zakupki-extract-*")
		if err != nil {
			writeExtractJSON(w, http.StatusOK, Response{
				Status:   false,
				Filename: name,
				Body:     "",
			})
			return
		}
		defer os.RemoveAll(tmpDir)

		srcPath := filepath.Join(tmpDir, name)
		dst, err := os.Create(srcPath)
		if err != nil {
			writeExtractJSON(w, http.StatusOK, Response{
				Status:   false,
				Filename: name,
				Body:     "",
			})
			return
		}
		if _, err := io.Copy(dst, file); err != nil {
			dst.Close()
			writeExtractJSON(w, http.StatusOK, Response{
				Status:   false,
				Filename: name,
				Body:     "",
			})
			return
		}
		if err := dst.Close(); err != nil {
			writeExtractJSON(w, http.StatusOK, Response{
				Status:   false,
				Filename: name,
				Body:     "",
			})
			return
		}

		txtPath := filepath.Join(tmpDir, strings.TrimSuffix(name, ext)+".txt")
		res := extract.ToTextOut(srcPath, txtPath)
		if res.Error != "" {
			writeExtractJSON(w, http.StatusOK, Response{
				Status:   false,
				Filename: name,
				Body:     "",
			})
			return
		}
		b, err := os.ReadFile(txtPath)
		if err != nil {
			writeExtractJSON(w, http.StatusOK, Response{
				Status:   false,
				Filename: name,
				Body:     "",
			})
			return
		}
		writeExtractJSON(w, http.StatusOK, Response{
			Status:   true,
			Filename: name,
			Body:     string(b),
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
