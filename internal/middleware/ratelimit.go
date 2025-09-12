package middleware

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
	// redisClient — глобальный клиент Redis,
	// инициализируется один раз при старте приложения,
	// чтобы переиспользовать соединение и не создавать клиента на каждый запрос.
	redisClient *redis.Client

	// ctx — контекст для операций Redis.
	// Используем background, т.к. запросы не отменяются из middleware.
	ctx = context.Background()

	// rateLimit — базовый лимит запросов в секунду (например, 1 запрос в секунду).
	// burstLimit — "всплеск" запросов, которые можно сделать сразу (например, 5).
	// windowSeconds — размер окна времени для лимита (1 секунда),
	// в течение которого считаются запросы.
	burstLimit    = 5 // максимальное количество запросов в окне
	windowSeconds = 1 // окно в секундах

	// Lua-скрипт для атомарного увеличения счётчика запросов и проверки лимита.
	// Используем Lua, чтобы гарантировать, что инкремент и проверка
	// выполняются одной атомарной операцией, без гонок.
	rateLimitScript = redis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local now = tonumber(ARGV[2])
local window = tonumber(ARGV[3])

-- Увеличиваем значение счётчика по ключу на 1
local current = redis.call("INCR", key)

-- Если это первый запрос за окно, задаём TTL, чтобы ключ автоматически удалился.
if current == 1 then
  redis.call("EXPIRE", key, window)
end

-- Если количество запросов превысило лимит, возвращаем 0 (запрещено).
if current > limit then
  return 0
end

-- Иначе возвращаем 1 (разрешено).
return 1
`)
)

// InitRedis инициализирует глобальный Redis клиент.
// Предполагается вызвать один раз при старте сервера,
// чтобы не создавать клиент на каждый HTTP запрос.
func InitRedis(addr, password string, db int) error{
	redisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Проверяем соединение
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}

	return nil
}

// RateLimit — middleware для ограничение запросов с помощью Redis.
// Основная идея: для каждого IP храним счётчик запросов в Redis с TTL,
// чтобы считать количество запросов за "окно" времени,
// и отклонять запросы, если лимит превышен.
func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем IP из RemoteAddr.
		// Если не удаётся сплитнуть (что редко),
		// используем как есть (чтобы всё равно ограничивать).
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		// Формируем уникальный ключ для Redis,
		// чтобы считать запросы по IP (можно расширить и по пути, если надо).
		key := "rate_limit:" + ip

		// Берём текущее время в секундах (не обязательно сейчас используется в скрипте,
		// но можно для расширения логики).
		now := time.Now().Unix()

		// Запускаем Lua-скрипт с ключом и параметрами:
		// burstLimit — максимально разрешённое число запросов за окно,
		// now — текущее время (пока не используется в скрипте),
		// windowSeconds — окно в секундах.
		ok, err := rateLimitScript.Run(ctx, redisClient, []string{key},
			burstLimit, now, windowSeconds).Result()

		if err != nil {
			// Если произошла ошибка с Redis (например, сеть упала),
			// по умолчанию "пропускаем" запрос — fail open.
			// Это можно изменить, если критично строго ограничивать.
			next.ServeHTTP(w, r)
			return
		}

		// Проверяем результат выполнения скрипта:
		// 1 — разрешено, 0 — превышен лимит.
		allowed, okCast := ok.(int64)
		if !okCast || allowed == 0 {
			// Если запросов больше лимита — отвечаем 429.
			utils.JSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "too many requests",
			})
			return
		}

		// Если всё ок — передаём запрос дальше.
		next.ServeHTTP(w, r)
	})
}
