package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"github.com/go-portfolio/http-middleware/internal/metrics"
)

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Обёртка, чтобы получить статус ответа
		rw := utils.NewResponseWriter(w)

		// Обработка запроса
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		statusCode := rw.Status
		path := r.URL.Path
		method := r.Method

		metrics.HttpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(statusCode)).Inc()
		metrics.HttpRequestDuration.WithLabelValues(method, path).Observe(duration)
	})
}
