package ratelimit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	redisClient *redis.Client
	ctx         = context.Background()

	burstLimit    = 5
	windowSeconds = 1

	rateLimitScript = redis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local now = tonumber(ARGV[2])
local window = tonumber(ARGV[3])
local current = redis.call("INCR", key)
if current == 1 then
  redis.call("EXPIRE", key, window)
end
if current > limit then
  return 0
end
return 1
`)

	// Метрики OpenTelemetry
	meter           = otel.Meter("ratelimit")
	requestsCounter metric.Int64Counter
	droppedCounter  metric.Int64Counter
	durationHist    metric.Float64Histogram
)

func init() {
	requestsCounter, _ = meter.Int64Counter("ratelimit_requests_total")
	droppedCounter, _ = meter.Int64Counter("ratelimit_dropped_total")
	durationHist, _ = meter.Float64Histogram("ratelimit_duration_seconds")
}

func InitRedis(addr, password string, db int) error {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}
	return nil
}

func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			durationHist.Record(r.Context(), time.Since(start).Seconds())
		}()

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		key := "rate_limit:" + ip
		now := time.Now().Unix()

		ok, err := rateLimitScript.Run(ctx, redisClient, []string{key}, burstLimit, now, windowSeconds).Result()
		requestsCounter.Add(r.Context(), 1)

		if err != nil {
			// fail-open
			next.ServeHTTP(w, r)
			return
		}

		allowed, okCast := ok.(int64)
		if !okCast || allowed == 0 {
			droppedCounter.Add(r.Context(), 1)
			utils.JSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "too many requests",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
