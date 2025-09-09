package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-portfolio/http-middleware/internal/middleware"
)

func TestAuthMiddleware(t *testing.T) {
	os.Setenv("AUTH_TOKEN", "tkn")
	handler := middleware.Auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// no token -> 401
	req := httptest.NewRequest("GET", "/secure", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}

	// valid token -> 200
	req = httptest.NewRequest("GET", "/secure", nil)
	req.Header.Set("Authorization", "Bearer tkn")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}
