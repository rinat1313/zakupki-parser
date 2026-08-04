package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/adapter"
	"github.com/rinat1313/zakupki-parser/pkg/collect"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	col := collect.New(true)
	if col.Client != nil && col.Client.HTTP != nil {
		col.Client.HTTP.Timeout = 45 * time.Second
	}
	pipe := &adapter.Pipeline{EIS: col}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "zakupki-parser",
			"adapters": adapterHosts(),
		})
	})
	mux.HandleFunc("POST /api/v1/fetch", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RegNumber  string `json:"reg_number"`
			SourceSite string `json:"source_site"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if req.RegNumber == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reg_number required"})
			return
		}
		out := pipe.Resolve(r.Context(), req.RegNumber, req.SourceSite)
		writeJSON(w, http.StatusOK, map[string]any{
			"reg_number":     req.RegNumber,
			"source_site":    req.SourceSite,
			"source_used":    out.SourceUsed,
			"failed":         out.FailedAnalyze || out.Result == nil,
			"message":        out.Message,
			"result":         out.Result,
		})
	})

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8091"
	}
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("zakupki-parser listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	_ = srv.Shutdown(shCtx)
}

func adapterHosts() []string {
	// best-effort list from known stubs + eis
	return []string{
		"zakupki.gov.ru (eis)",
		"tektorg.ru", "zakupki.mos.ru", "roseltorg.ru", "sberbank-ast.ru",
		"rts-tender.ru", "etp.ets.ru", "fabrikant.ru", "etp.gpb.ru",
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
