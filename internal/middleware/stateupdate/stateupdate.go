package stateupdate

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"github.com/go-redis/redis/v8"
)

var (
	redisClientState *redis.Client
	ctxState         = context.Background()

	// Lua скрипт для атомарного обновления
	stateUpdateScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    redis.call("SET", KEYS[1], ARGV[2])
    return 1
else
    return 0
end
`)
)

// InitRedisState инициализирует Redis клиент для обновления состояния
func InitRedisState(addr, password string, db int) error {
	redisClientState = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := redisClientState.Ping(ctxState).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}
	return nil
}

// StateUpdateMiddleware — middleware для атомарного обновления значения
// key: ключ в Redis
// expected: ожидаемое текущее значение
// newVal: новое значение
func StateUpdateMiddleware(key, expected, newVal string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Выполняем Lua скрипт для атомарного обновления
			res, err := stateUpdateScript.Run(ctxState, redisClientState, []string{key}, expected, newVal).Result()
			if err != nil {
				// fail-open: если Redis недоступен, пропускаем запрос
				next.ServeHTTP(w, r)
				return
			}

			success, ok := res.(int64)
			if !ok || success == 0 {
				utils.JSON(w, http.StatusConflict, map[string]string{
					"error": "update failed, expected value did not match",
				})
				return
			}

			// Успешное обновление
			next.ServeHTTP(w, r)
		})
	}
}
