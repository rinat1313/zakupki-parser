package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

const bearerPrefix = "Bearer "

func ValidBearer(header string, expected string) bool {
	if expected == "" {
		return false
	}
	if !strings.HasPrefix(header, bearerPrefix) {
		return false
	}
	token := strings.TrimSpace(header[len(bearerPrefix):])
	if token == "" {
		return false
	}
	got := sha256.Sum256([]byte(token))
	want := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

func Middleware(expected string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ValidBearer(r.Header.Get("Authorization"), expected) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}` + "\n"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
