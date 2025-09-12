package slidingwindow

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"github.com/go-redis/redis/v8"
)

var (
	// redisClientSliding — глобальный клиент Redis для Sliding Window
	redisClientSliding *redis.Client

	// ctxSliding — контекст для операций Redis
	ctxSliding = context.Background()

	// Lua скрипт для скользящего окна
	slidingWindowScript = redis.NewScript(`
local key = KEYS[1]              -- ключ для пользователя
local now = tonumber(ARGV[1])    -- текущее время в миллисекундах
local window = tonumber(ARGV[2]) -- размер окна времени (мс)
local limit = tonumber(ARGV[3])  -- максимальное количество запросов

-- Удаляем все старые записи за пределами окна
redis.call("ZREMRANGEBYSCORE", key, 0, now - window)

-- Считаем количество запросов в текущем окне
local count = redis.call("ZCARD", key)
if count >= limit then
  return 0  -- лимит превышен
end

-- Добавляем текущий запрос
redis.call("ZADD", key, now, now)

-- Устанавливаем TTL на ключ
redis.call("PEXPIRE", key, window)

return 1  -- запрос разрешён
`)
)

// InitRedisSliding инициализирует глобальный Redis клиент для Sliding Window
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

// SlidingWindow — middleware для скользящего окна
// limit: максимальное количество запросов
// windowMS: размер окна в миллисекундах
func SlidingWindow(limit int, windowMS int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем IP клиента
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			key := "sliding_rate:" + ip

			now := time.Now().UnixMilli()

			// Выполняем Lua скрипт в Redis
			res, err := slidingWindowScript.Run(ctxSliding, redisClientSliding, []string{key}, now, windowMS, limit).Result()
			if err != nil {
				// fail-open: если Redis недоступен, пропускаем запрос
				next.ServeHTTP(w, r)
				return
			}

			allowed, ok := res.(int64)
			if !ok || allowed == 0 {
				// лимит превышен
				utils.JSON(w, http.StatusTooManyRequests, map[string]string{
					"error": "too many requests",
				})
				return
			}

			// запрос разрешён
			next.ServeHTTP(w, r)
		})
	}
}
