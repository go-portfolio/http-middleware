package session

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
	redisClientSession *redis.Client
	ctxSession         = context.Background()
	meter              = otel.Meter("session")

	sessionRenewedCounter   metric.Int64Counter
	sessionMissingCounter   metric.Int64Counter
	sessionNotFoundCounter  metric.Int64Counter
	sessionDurationHist     metric.Float64Histogram

	sessionScript = redis.NewScript(`
local val = redis.call("GET", KEYS[1])
if val then
    redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return val
`)
)

func init() {
	sessionRenewedCounter, _ = meter.Int64Counter("session_renewed_total")
	sessionMissingCounter, _ = meter.Int64Counter("session_missing_total")
	sessionNotFoundCounter, _ = meter.Int64Counter("session_notfound_total")
	sessionDurationHist, _ = meter.Float64Histogram("session_duration_seconds")
}

func InitRedisSession(addr, password string, db int) error {
	redisClientSession = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := redisClientSession.Ping(ctxSession).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}
	return nil
}

func SessionMiddleware(ttlSec int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			cookie, err := r.Cookie("session_id")
			if err != nil || cookie.Value == "" {
				sessionMissingCounter.Add(r.Context(), 1)
				utils.JSON(w, http.StatusUnauthorized, map[string]string{
					"error": "missing session_id",
				})
				return
			}

			sessionKey := "session:" + cookie.Value
			res, err := sessionScript.Run(ctxSession, redisClientSession, []string{sessionKey}, ttlSec).Result()
			duration := time.Since(start).Seconds()
			sessionDurationHist.Record(r.Context(), duration) // время выполнения Lua скрипта

			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if res == nil {
				sessionNotFoundCounter.Add(r.Context(), 1)
				utils.JSON(w, http.StatusUnauthorized, map[string]string{
					"error": "session not found",
				})
				return
			}

			sessionRenewedCounter.Add(r.Context(), 1) // сессия успешно продлена
			next.ServeHTTP(w, r)
		})
	}
}
