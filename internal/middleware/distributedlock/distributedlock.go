package distributedlock

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
	redisClientLock *redis.Client
	ctxLock         = context.Background()

	lockScript = redis.NewScript(`
if redis.call("SETNX", KEYS[1], ARGV[1]) == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[2])
    return 1
else
    return 0
end
`)

	meter       = otel.Meter("distributedlock")
	lockSuccess metric.Int64Counter
	lockFailed  metric.Int64Counter
	lockTime    metric.Float64Histogram
)

func init() {
	lockSuccess, _ = meter.Int64Counter("lock_success_total")
	lockFailed, _ = meter.Int64Counter("lock_failed_total")
	lockTime, _ = meter.Float64Histogram("lock_duration_seconds")
}

func InitRedisLock(addr, password string, db int) error {
	redisClientLock = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := redisClientLock.Ping(ctxLock).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}
	return nil
}

func RedisLockMiddleware(lockKey string, ttlMS int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			defer lockTime.Record(r.Context(), time.Since(start).Seconds())

			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			clientID := ip + ":" + fmt.Sprint(time.Now().UnixNano())

			res, err := lockScript.Run(ctxLock, redisClientLock, []string{lockKey}, clientID, ttlMS).Result()
			if err != nil {
				lockFailed.Add(r.Context(), 1)
				next.ServeHTTP(w, r)
				return
			}

			locked, ok := res.(int64)
			if !ok || locked == 0 {
				lockFailed.Add(r.Context(), 1)
				utils.JSON(w, http.StatusTooManyRequests, map[string]string{
					"error": "resource is locked",
				})
				return
			}

			lockSuccess.Add(r.Context(), 1)
			next.ServeHTTP(w, r)
		})
	}
}
