package stateupdate

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	redisClientState *redis.Client
	ctxState         = context.Background()
	meter            = otel.Meter("stateupdate") // создаём Meter для метрик

	successCounter metric.Int64Counter
	failCounter    metric.Int64Counter
	durationHist   metric.Float64Histogram

	stateUpdateScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    redis.call("SET", KEYS[1], ARGV[2])
    return 1
else
    return 0
end
`)
)

func init() {
	// создаём метрики
	successCounter, _ = meter.Int64Counter("stateupdate_success_total")
	failCounter, _ = meter.Int64Counter("stateupdate_fail_total")
	durationHist, _ = meter.Float64Histogram("stateupdate_duration_seconds")
}

func InitRedisState(addr, password string, db int) error {
	redisClientState = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := redisClientState.Ping(ctxState).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}
	return nil
}

// StateUpdateMiddleware с метриками
func StateUpdateMiddleware(key, expected, newVal string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			res, err := stateUpdateScript.Run(ctxState, redisClientState, []string{key}, expected, newVal).Result()
			duration := time.Since(start).Seconds()
			durationHist.Record(r.Context(), duration) // записываем время выполнения

			if err != nil {
				failCounter.Add(r.Context(), 1)
				next.ServeHTTP(w, r)
				return
			}

			success, ok := res.(int64)
			if !ok || success == 0 {
				failCounter.Add(r.Context(), 1)
				utils.JSON(w, http.StatusConflict, map[string]string{
					"error": "update failed, expected value did not match",
				})
				return
			}

			successCounter.Add(r.Context(), 1)
			next.ServeHTTP(w, r)
		})
	}
}
