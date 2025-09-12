package session

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"github.com/go-redis/redis/v8"
)

var (
	redisClientSession *redis.Client
	ctxSession         = context.Background()

	// Lua скрипт для продления TTL сессии
	sessionScript = redis.NewScript(`
local val = redis.call("GET", KEYS[1])
if val then
    redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return val
`)
)

// InitRedisSession инициализирует Redis клиент для сессий
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

// SessionMiddleware — middleware для продления TTL сессии
// ttlSec: время жизни сессии в секундах
func SessionMiddleware(ttlSec int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем идентификатор сессии из cookie (или из заголовка)
			cookie, err := r.Cookie("session_id")
			if err != nil || cookie.Value == "" {
				utils.JSON(w, http.StatusUnauthorized, map[string]string{
					"error": "missing session_id",
				})
				return
			}

			sessionKey := "session:" + cookie.Value

			// Выполняем Lua скрипт: получаем значение и продлеваем TTL
			res, err := sessionScript.Run(ctxSession, redisClientSession, []string{sessionKey}, ttlSec).Result()
			if err != nil {
				// fail-open: если Redis недоступен, пропускаем запрос
				next.ServeHTTP(w, r)
				return
			}

			if res == nil {
				utils.JSON(w, http.StatusUnauthorized, map[string]string{
					"error": "session not found",
				})
				return
			}

			// Значение сессии можно передать дальше через контекст, если нужно
			// ctx := context.WithValue(r.Context(), "session_value", res.(string))
			// r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
