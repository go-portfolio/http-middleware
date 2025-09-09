package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-portfolio/http-middleware/internal/utils"
)

func Auth(next http.Handler) http.Handler {
	token := os.Getenv("AUTH_TOKEN") // читать из окружения
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
			utils.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
