package pagecounter

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-redis/redis/v8"
)

var (
	// redisClientCounter — глобальный клиент Redis для счётчиков
	redisClientCounter *redis.Client
	ctxCounter         = context.Background()

	// Lua скрипт для инкремента счётчика с TTL
	counterScript = redis.NewScript(`
-- KEYS[1] - ключ счётчика
-- ARGV[1] - TTL в секундах

local current = redis.call("INCR", KEYS[1])

if current == 1 then
    redis.call("EXPIRE", KEYS[1], ARGV[1])
end

return current
`)
)

// InitRedisCounter инициализирует Redis клиент для счётчиков
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

// CounterMiddleware — middleware для инкремента счётчика
// key: ключ счётчика (например, "counter:page_view")
// ttlSec: TTL ключа в секундах
func CounterMiddleware(key string, ttlSec int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Выполняем Lua скрипт для увеличения счётчика
			res, err := counterScript.Run(ctxCounter, redisClientCounter, []string{key}, ttlSec).Result()
			if err != nil {
				// fail-open: при ошибке Redis пропускаем запрос
				next.ServeHTTP(w, r)
				return
			}

			count, ok := res.(int64)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			// Можно добавить заголовок X-Counter для мониторинга
			w.Header().Set("X-Counter", fmt.Sprintf("%d", count))

			// Продолжаем выполнение handler'а
			next.ServeHTTP(w, r)
		})
	}
}
