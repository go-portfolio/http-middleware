package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"golang.org/x/time/rate"
)

// client хранит состояние для конкретного IP-адреса:
// - limiter: токен-бакет, ограничивающий количество запросов,
// - lastSeen: время последнего запроса (нужно для очистки старых записей).
type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Глобальная таблица клиентов (ключ — IP-адрес).
var clients = make(map[string]*client)

// Мьютекс для синхронизации доступа к clients.
var mu sync.Mutex

// getClientLimiter возвращает rate limiter для указанного IP.
// Если limiter ещё не создан — создаём новый с настройками (1 запрос/сек, burst=5).
func getClientLimiter(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	c, exists := clients[ip]
	if !exists {
		// Ограничение: 1 запрос в секунду, можно "накопить" до 5 сразу.
		limiter := rate.NewLimiter(1, 5)
		clients[ip] = &client{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	// Если клиент уже существует, обновляем время последнего запроса.
	c.lastSeen = time.Now()
	return c.limiter
}

// cleanupClients — фоновая горутина для очистки старых клиентов.
// Каждую минуту проверяет таблицу и удаляет клиентов, которые неактивны > 3 минут.
// Это защищает от утечек памяти при большом количестве уникальных IP.
func cleanupClients() {
	for {
		time.Sleep(time.Minute)

		mu.Lock()
		for ip, c := range clients {
			if time.Since(c.lastSeen) > 3*time.Minute {
				delete(clients, ip)
			}
		}
		mu.Unlock()
	}
}

// RateLimit — middleware для ограничения количества запросов с одного IP.
// Если лимит превышен — возвращает 429 Too Many Requests.
// Иначе передаёт выполнение следующему handler'у.
func RateLimit(next http.Handler) http.Handler {
	// Запускаем горутину для периодической очистки клиентов.
	go cleanupClients()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем IP клиента из RemoteAddr.
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)

		// Получаем (или создаём) rate limiter для IP.
		limiter := getClientLimiter(ip)

		// Проверяем: разрешён ли запрос.
		if !limiter.Allow() {
			utils.JSON(w, http.StatusTooManyRequests,
				map[string]string{"error": "too many requests"})
			return
		}

		// Если лимит не превышен — вызываем следующий handler.
		next.ServeHTTP(w, r)
	})
}
