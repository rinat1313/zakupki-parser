package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/searchsvc/config"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/db"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/models"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/worker"
)

type Server struct {
	Store  *db.Store
	Cfg    config.Config
	Runner *worker.Runner
	Mux    *http.ServeMux
	logger *log.Logger
}

func New(store *db.Store, cfg config.Config, runner *worker.Runner) *Server {
	s := &Server{
		Store:  store,
		Cfg:    cfg,
		Runner: runner,
		Mux:    http.NewServeMux(),
		logger: log.Default(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.Mux.HandleFunc("GET /health", s.handleHealth)

	s.Mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.Mux.HandleFunc("POST /api/v1/auth/logout", s.requireAuth(s.handleLogout))
	s.Mux.HandleFunc("GET /api/v1/auth/me", s.requireAuth(s.handleMe))

	s.Mux.HandleFunc("GET /api/v1/searchers", s.requireAuth(s.handleListSearchers))
	s.Mux.HandleFunc("POST /api/v1/searchers", s.requireAuth(s.handleCreateSearcher))
	s.Mux.HandleFunc("GET /api/v1/searchers/{id}", s.requireAuth(s.handleGetSearcher))
	s.Mux.HandleFunc("PUT /api/v1/searchers/{id}", s.requireAuth(s.handleUpdateSearcher))
	s.Mux.HandleFunc("DELETE /api/v1/searchers/{id}", s.requireAuth(s.handleDeleteSearcher))
	s.Mux.HandleFunc("PUT /api/v1/searchers/{id}/auto-ai", s.requireAuth(s.handleSetAutoAI))
	s.Mux.HandleFunc("POST /api/v1/searchers/{id}/run", s.requireAuth(s.handleRunSearcher))
	s.Mux.HandleFunc("GET /api/v1/searchers/{id}/tenders", s.requireAuth(s.handleListTenders))

	// Legacy-compatible aliases (same data, profile-shaped response where needed).
	s.Mux.HandleFunc("GET /api/v1/search-profiles", s.requireAuth(s.handleListProfiles))
	s.Mux.HandleFunc("POST /api/v1/search-profiles", s.requireAuth(s.handleCreateProfile))
	s.Mux.HandleFunc("GET /api/v1/search-profiles/{id}", s.requireAuth(s.handleGetProfile))
	s.Mux.HandleFunc("PUT /api/v1/search-profiles/{id}", s.requireAuth(s.handleUpdateProfile))
	s.Mux.HandleFunc("DELETE /api/v1/search-profiles/{id}", s.requireAuth(s.handleDeleteSearcher))
	s.Mux.HandleFunc("GET /api/v1/search-profiles/{id}/eis-url", s.requireAuth(s.handleProfileEISURL))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "zakupki-search",
	})
}

type ctxKey int

const userCtxKey ctxKey = 1

func userFrom(r *http.Request) models.User {
	u, _ := r.Context().Value(userCtxKey).(models.User)
	return u
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodeJSON allows unknown fields (UI may send extras).
func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(dst)
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if c, err := r.Cookie("session"); err == nil {
		return c.Value
	}
	return ""
}

func setSessionCookie(w http.ResponseWriter, token string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
