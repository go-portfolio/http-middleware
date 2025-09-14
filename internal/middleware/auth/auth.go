package auth

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter         = otel.Meter("auth")
	authSuccess   metric.Int64Counter
	authFailed    metric.Int64Counter
)

func init() {
	authSuccess, _ = meter.Int64Counter("auth_success_total")
	authFailed, _ = meter.Int64Counter("auth_failed_total")
}

// Auth — middleware для проверки авторизации по токену.
// Метрики:
// - auth_success_total — успешные авторизации
// - auth_failed_total — неуспешные авторизации
func Auth(next http.Handler) http.Handler {
	token := os.Getenv("AUTH_TOKEN")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")

		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
			authFailed.Add(r.Context(), 1)
			utils.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		authSuccess.Add(r.Context(), 1)
		next.ServeHTTP(w, r)
	})
}
