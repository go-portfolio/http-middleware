package distributedlock

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
	// redisClientLock — глобальный клиент Redis для блокировок
	redisClientLock *redis.Client
	ctxLock         = context.Background()

	// Lua скрипт для установки блокировки
	lockScript = redis.NewScript(`
-- KEYS[1] - ключ блокировки
-- ARGV[1] - уникальный идентификатор клиента
-- ARGV[2] - TTL в миллисекундах

if redis.call("SETNX", KEYS[1], ARGV[1]) == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[2])
    return 1
else
    return 0
end
`)
)

// InitRedisLock инициализирует Redis клиент для блокировок
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

// RedisLockMiddleware — middleware для распределённой блокировки
// lockKey: ключ блокировки (например, "lock:order:123")
// ttlMS: время жизни блокировки в миллисекундах
func RedisLockMiddleware(lockKey string, ttlMS int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Можно использовать IP + путь как уникальный идентификатор клиента
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			clientID := ip + ":" + fmt.Sprint(time.Now().UnixNano())

			// Выполняем Lua скрипт для установки блокировки
			res, err := lockScript.Run(ctxLock, redisClientLock, []string{lockKey}, clientID, ttlMS).Result()
			if err != nil {
				// fail-open: при ошибке Redis пропускаем
				next.ServeHTTP(w, r)
				return
			}

			locked, ok := res.(int64)
			if !ok || locked == 0 {
				// Блокировка занята
				utils.JSON(w, http.StatusTooManyRequests, map[string]string{
					"error": "resource is locked",
				})
				return
			}

			// Запрос разрешён, блокировка установлена
			next.ServeHTTP(w, r)
		})
	}
}
