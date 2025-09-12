package queue

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"github.com/go-redis/redis/v8"
)

var (
	redisClientQueue *redis.Client
	ctxQueue         = context.Background()

	// Lua скрипт для RPOPLPUSH
	queueScript = redis.NewScript(`
local item = redis.call("RPOPLPUSH", KEYS[1], KEYS[2])
return item
`)
)

// InitRedisQueue инициализирует Redis клиент для очередей
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

// QueueMiddleware — middleware для обработки очереди задач
// sourceKey: исходная очередь
// processingKey: очередь обработки
func QueueMiddleware(sourceKey, processingKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Перемещаем элемент из исходной очереди в очередь обработки
			res, err := queueScript.Run(ctxQueue, redisClientQueue, []string{sourceKey, processingKey}).Result()
			if err != nil {
				// fail-open: при ошибке Redis пропускаем запрос
				next.ServeHTTP(w, r)
				return
			}

			if res == nil {
				// Очередь пуста
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

			// Добавляем заголовок с текущим элементом для мониторинга
			w.Header().Set("X-Queue-Item", item)

			// Передаём элемент дальше в handler
			next.ServeHTTP(w, r)
		})
	}
}
