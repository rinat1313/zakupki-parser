package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/docparser/auth"
	"github.com/rinat1313/zakupki-parser/internal/docparser/channels"
	"github.com/rinat1313/zakupki-parser/internal/docparser/config"
	"github.com/rinat1313/zakupki-parser/internal/docparser/core"
	"github.com/rinat1313/zakupki-parser/internal/extract"
)

type Server struct {
	Mux       *http.ServeMux
	cfg       config.Config
	channels  *channels.Store
	extractor core.Extractor
}

func New(cfg config.Config, ch *channels.Store, ex core.Extractor) *Server {
	s := &Server{
		Mux:       http.NewServeMux(),
		cfg:       cfg,
		channels:  ch,
		extractor: ex,
	}
	s.Mux.HandleFunc("GET /health", s.handleHealth)
	s.Mux.HandleFunc("GET /openapi.yaml", s.handleOpenAPI)
	protected := auth.Middleware(cfg.APIKey, http.HandlerFunc(s.handleExtract))
	s.Mux.Handle("POST /api/v1/extract", protected)
	s.Mux.Handle("GET /api/v1/channels", auth.Middleware(cfg.APIKey, http.HandlerFunc(s.handleGetChannels)))
	s.Mux.Handle("PUT /api/v1/channels", auth.Middleware(cfg.APIKey, http.HandlerFunc(s.handlePutChannels)))
	return s
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	st := s.channels.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"service":  config.ServiceName,
		"channels": st,
	})
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(openAPISpec))
}

func (s *Server) handleGetChannels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.channels.Get())
}

func (s *Server) handlePutChannels(w http.ResponseWriter, r *http.Request) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	cur := s.channels.Get()
	if v, ok := boolField(raw, "http_ingest", "http"); ok {
		cur.HTTPIngest = v
	}
	if v, ok := boolField(raw, "kafka_ingest", "kafka"); ok {
		cur.KafkaIngest = v
	}
	if err := s.channels.Set(cur); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.channels.Get())
}

func boolField(raw map[string]json.RawMessage, keys ...string) (bool, bool) {
	for _, k := range keys {
		b, ok := raw[k]
		if !ok {
			continue
		}
		var v bool
		if err := json.Unmarshal(b, &v); err != nil {
			continue
		}
		return v, true
	}
	return false, false
}

func (s *Server) handleExtract(w http.ResponseWriter, r *http.Request) {
	setLongRunningHeaders(w)
	if !s.channels.Get().HTTPIngest {
		writeJSON(w, http.StatusServiceUnavailable, core.Result{Error: "http ingest disabled"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, core.MaxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(core.MaxUploadBytes); err != nil {
		if isRequestTooLarge(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, core.Result{Error: "upload too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, core.Result{Error: "invalid multipart"})
		return
	}
	file, hdr, err := r.FormFile(core.FormFileField)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, core.Result{Error: "missing file field"})
		return
	}
	defer file.Close()

	name := filepath.Base(hdr.Filename)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "upload.bin"
	}
	corr := strings.TrimSpace(r.Header.Get("X-Correlation-ID"))
	if corr == "" {
		corr = strings.TrimSpace(r.FormValue("correlation_id"))
	}

	data, err := io.ReadAll(file)
	if err != nil {
		if isRequestTooLarge(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, core.Result{Filename: name, CorrelationID: corr, Error: "upload too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, core.Result{Filename: name, CorrelationID: corr, Error: err.Error()})
		return
	}
	if len(data) > core.MaxUploadBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, core.Result{Filename: name, CorrelationID: corr, Error: "upload too large"})
		return
	}

	ext := strings.ToLower(filepath.Ext(name))
	if !isSupportedExt(ext) {
		code := http.StatusUnsupportedMediaType
		if ext == "" {
			code = http.StatusBadRequest
		}
		writeJSON(w, code, core.Result{Filename: name, CorrelationID: corr, Error: core.UnsupportedExtensionErr(ext)})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(core.ProcessingMaxSeconds)*time.Second)
	defer cancel()

	res := s.extractor.Extract(ctx, name, data, corr)
	if res.Error == "processing timeout" {
		writeJSON(w, http.StatusGatewayTimeout, res)
		return
	}
	if res.Error == "extracted body too large" {
		writeJSON(w, http.StatusRequestEntityTooLarge, res)
		return
	}
	if res.Error != "" && !res.Status {
		writeJSON(w, http.StatusUnprocessableEntity, res)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func isSupportedExt(ext string) bool {
	return extract.SupportedExtensions[ext]
}

func isRequestTooLarge(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "too large") || strings.Contains(msg, "request body too large")
}

func setLongRunningHeaders(w http.ResponseWriter) {
	sec := strconv.Itoa(core.ProcessingMaxSeconds)
	w.Header().Set("X-Processing-Max-Seconds", sec)
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Keep-Alive", "timeout="+sec)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func ServerTimeouts() (readHeader, read, write, idle time.Duration) {
	return 10 * time.Second,
		time.Duration(core.ProcessingMaxSeconds+60) * time.Second,
		time.Duration(core.ProcessingMaxSeconds+60) * time.Second,
		time.Duration(core.ProcessingMaxSeconds+60) * time.Second
}

func ShutdownTimeout() time.Duration {
	return time.Duration(core.ProcessingMaxSeconds+60) * time.Second
}

const openAPISpec = `openapi: 3.0.3
info:
  title: doc-parser
  version: 1.0.0
paths:
  /health:
    get:
      summary: Health check
      responses:
        '200':
          description: OK
  /openapi.yaml:
    get:
      summary: OpenAPI document
      responses:
        '200':
          description: YAML spec
  /api/v1/extract:
    post:
      summary: Extract text from document
      security:
        - bearerAuth: []
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file:
                  type: string
                  format: binary
      responses:
        '200':
          description: Extract result
        '401':
          description: Unauthorized
        '413':
          description: Payload too large
        '415':
          description: Unsupported media type
        '503':
          description: HTTP ingest disabled
        '504':
          description: Timeout
  /api/v1/channels:
    get:
      summary: Channel flags
      security:
        - bearerAuth: []
      responses:
        '200':
          description: Current channels
    put:
      summary: Update channel flags
      security:
        - bearerAuth: []
      responses:
        '200':
          description: Updated
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
`
