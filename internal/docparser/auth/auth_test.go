package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidBearer(t *testing.T) {
	if !ValidBearer("Bearer abc", "abc") {
		t.Fatal("expected valid")
	}
	if ValidBearer("Bearer abc", "xyz") {
		t.Fatal("expected invalid")
	}
	if ValidBearer("Basic abc", "abc") {
		t.Fatal("expected invalid scheme")
	}
	if ValidBearer("Bearer ", "abc") {
		t.Fatal("expected empty token invalid")
	}
}

func TestMiddlewareRejects(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := Middleware("key", next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestMiddlewareAllows(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := Middleware("key", next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer key")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("code=%d called=%v", rec.Code, called)
	}
}
