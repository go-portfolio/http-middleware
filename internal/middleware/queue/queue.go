package queue

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
	redisClientQueue *redis.Client
	ctxQueue         = context.Background()

	queueScript = redis.NewScript(`
local item = redis.call("RPOPLPUSH", KEYS[1], KEYS[2])
return item
`)

	// Метрики OpenTelemetry
	meter          = otel.Meter("queue")
	processedCounter metric.Int64Counter
	emptyCounter     metric.Int64Counter
	durationHist     metric.Float64Histogram
)

func init() {
	processedCounter, _ = meter.Int64Counter("queue_processed_total")
	emptyCounter, _ = meter.Int64Counter("queue_empty_total")
	durationHist, _ = meter.Float64Histogram("queue_processing_duration_seconds")
}

func InitRedisQueue(addr, password string, db int) error {
	redisClientQueue = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := redisClientQueue.Ping(ctxQueue).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}
	return nil
}

func QueueMiddleware(sourceKey, processingKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			defer durationHist.Record(r.Context(), time.Since(start).Seconds())

			res, err := queueScript.Run(ctxQueue, redisClientQueue, []string{sourceKey, processingKey}).Result()
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if res == nil {
				emptyCounter.Add(r.Context(), 1)
				utils.JSON(w, http.StatusNoContent, map[string]string{
					"error": "queue is empty",
				})
				return
			}

			item, ok := res.(string)
			if !ok {
				utils.JSON(w, http.StatusInternalServerError, map[string]string{
					"error": "invalid item type",
				})
				return
			}

			w.Header().Set("X-Queue-Item", item)
			processedCounter.Add(r.Context(), 1)

			next.ServeHTTP(w, r)
		})
	}
}
