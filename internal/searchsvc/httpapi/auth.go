package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/searchsvc/auth"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/db"
)

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
	User  any    `json:"user"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Login = strings.TrimSpace(req.Login)
	if req.Login == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "login and password required")
		return
	}

	u, err := s.Store.GetUserByLogin(r.Context(), req.Login)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid login or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid login or password")
		return
	}

	plain, hash, err := auth.NewToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}
	expires := time.Now().Add(s.Cfg.SessionTTL)
	if err := s.Store.CreateSession(r.Context(), u.ID, hash, expires); err != nil {
		writeError(w, http.StatusInternalServerError, "session error")
		return
	}

	setSessionCookie(w, plain, s.Cfg.SessionTTL)
	writeJSON(w, http.StatusOK, loginResponse{
		Token: plain,
		User:  u.Public(),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if t := bearerToken(r); t != "" {
		_ = s.Store.DeleteSession(r.Context(), auth.HashToken(t))
	}
	clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, userFrom(r))
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		u, err := s.Store.UserByTokenHash(r.Context(), auth.HashToken(token))
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "auth error")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, u)
		next(w, r.WithContext(ctx))
	}
}
