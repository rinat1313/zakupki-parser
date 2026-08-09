package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rinat1313/zakupki-parser/cmd/search/swagger"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/config"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/db"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/httpapi"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/worker"
)

func main() {
	cfg := config.FromEnv()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := waitDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	migDir := env("MIGRATIONS_DIR", "migrations/search")
	if abs, err := filepath.Abs(migDir); err == nil {
		migDir = abs
	}
	if err := store.Migrate(ctx, migDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("migrations applied from %s", migDir)

	if n, err := store.DeleteExpiredSessions(ctx); err != nil {
		log.Printf("cleanup sessions: %v", err)
	} else if n > 0 {
		log.Printf("deleted %d expired sessions", n)
	}

	runner := worker.NewRunner(store, cfg)
	api := httpapi.New(store, cfg, runner)
	swagger.Mount(api.Mux)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           withCORS(api.Mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("zakupki-search listening on %s; swagger UI at /swagger/", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func waitDB(ctx context.Context, dsn string) (*db.Store, error) {
	var last error
	for i := 0; i < 30; i++ {
		store, err := db.Connect(ctx, dsn)
		if err == nil {
			return store, nil
		}
		last = err
		log.Printf("waiting for postgres: %v", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, last
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
