package recovery

import (
	"log"
	"net/http"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter           = otel.Meter("recovery")
	panicCounter    metric.Int64Counter
	successCounter  metric.Int64Counter
	durationHist    metric.Float64Histogram
)

func init() {
	panicCounter, _ = meter.Int64Counter("recovery_panic_total")
	successCounter, _ = meter.Int64Counter("recovery_success_total")
	durationHist, _ = meter.Float64Histogram("recovery_duration_seconds")
}

// Recovery — middleware, перехватывает паники и возвращает 500
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			duration := time.Since(start).Seconds()
			durationHist.Record(r.Context(), duration)

			if rec := recover(); rec != nil {
				panicCounter.Add(r.Context(), 1)
				log.Printf("panic: %v", rec)
				utils.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
				return
			}
			successCounter.Add(r.Context(), 1)
		}()

		next.ServeHTTP(w, r)
	})
}
