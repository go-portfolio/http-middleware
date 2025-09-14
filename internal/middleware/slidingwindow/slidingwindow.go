package slidingwindow

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
	redisClientSliding *redis.Client
	ctxSliding         = context.Background()
	meter              = otel.Meter("slidingwindow")

	allowedCounter metric.Int64Counter
	blockedCounter metric.Int64Counter
	durationHist   metric.Float64Histogram

	slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])

redis.call("ZREMRANGEBYSCORE", key, 0, now - window)
local count = redis.call("ZCARD", key)
if count >= limit then
  return 0
end
redis.call("ZADD", key, now, now)
redis.call("PEXPIRE", key, window)
return 1
`)
)

func init() {
	allowedCounter, _ = meter.Int64Counter("slidingwindow_allowed_total")
	blockedCounter, _ = meter.Int64Counter("slidingwindow_blocked_total")
	durationHist, _ = meter.Float64Histogram("slidingwindow_duration_seconds")
}

func InitRedisSliding(addr, password string, db int) error {
	redisClientSliding = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := redisClientSliding.Ping(ctxSliding).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}
	return nil
}

func SlidingWindow(limit int, windowMS int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			key := "sliding_rate:" + ip

			now := time.Now().UnixMilli()
			res, err := slidingWindowScript.Run(ctxSliding, redisClientSliding, []string{key}, now, windowMS, limit).Result()
			duration := time.Since(start).Seconds()
			durationHist.Record(r.Context(), duration) // время выполнения Lua скрипта

			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			allowed, ok := res.(int64)
			if !ok || allowed == 0 {
				blockedCounter.Add(r.Context(), 1) // заблокировано
				utils.JSON(w, http.StatusTooManyRequests, map[string]string{
					"error": "too many requests",
				})
				return
			}

			allowedCounter.Add(r.Context(), 1) // разрешено
			next.ServeHTTP(w, r)
		})
	}
}
