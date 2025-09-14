package pagecounter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	redisClientCounter *redis.Client
	ctxCounter         = context.Background()

	counterScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
    redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return current
`)

	meter        = otel.Meter("pagecounter")
	pageCounter  metric.Int64Counter
	pageDuration metric.Float64Histogram
)

func init() {
	pageCounter, _ = meter.Int64Counter("page_view_total")
	pageDuration, _ = meter.Float64Histogram("page_view_duration_seconds")
}

func InitRedisCounter(addr, password string, db int) error {
	redisClientCounter = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := redisClientCounter.Ping(ctxCounter).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}
	return nil
}

func CounterMiddleware(key string, ttlSec int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			defer pageDuration.Record(r.Context(), time.Since(start).Seconds())

			res, err := counterScript.Run(ctxCounter, redisClientCounter, []string{key}, ttlSec).Result()
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			count, ok := res.(int64)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-Counter", fmt.Sprintf("%d", count))

			// Метрика общего числа просмотров страницы
			pageCounter.Add(r.Context(), 1)

			next.ServeHTTP(w, r)
		})
	}
}
