package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-portfolio/http-middleware/internal/handlers"
	"github.com/go-portfolio/http-middleware/internal/middleware/auth"
	"github.com/go-portfolio/http-middleware/internal/middleware/distributedlock"
	"github.com/go-portfolio/http-middleware/internal/middleware/logging"
	"github.com/go-portfolio/http-middleware/internal/middleware/metrics"
	"github.com/go-portfolio/http-middleware/internal/middleware/pagecounter"
	"github.com/go-portfolio/http-middleware/internal/middleware/queue"
	"github.com/go-portfolio/http-middleware/internal/middleware/ratelimit"
	"github.com/go-portfolio/http-middleware/internal/middleware/recovery"
	"github.com/go-portfolio/http-middleware/internal/middleware/session"
	"github.com/go-portfolio/http-middleware/internal/middleware/slidingwindow"
	"github.com/go-portfolio/http-middleware/internal/utils"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Middleware — тип функции-обёртки, которая принимает http.Handler и возвращает новый http.Handler.
// Таким образом можно строить "цепочки" middleware вокруг конечного обработчика.
type Middleware func(http.Handler) http.Handler

// Chain — функция для последовательного оборачивания handler'а в цепочку middleware.
// Middleware применяются справа налево (Auth → Logging → Recovery → Handler).
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func orderHandler(w http.ResponseWriter, r *http.Request) {
	// Тут бизнес-логика обработки заказа
	utils.JSON(w, http.StatusOK, map[string]string{
		"status": "order processed",
	})
}

// PageHandler — пример обработчика страницы
func pageHandler(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"status": "page served",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// очереди
func processHandler(w http.ResponseWriter, r *http.Request) {
	item := w.Header().Get("X-Queue-Item")
	utils.JSON(w, http.StatusOK, map[string]string{
		"status": "processed",
		"task":   item,
	})
}

// SecureHandler — пример защищённого обработчика
func SecureHandler(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]string{
		"status": "secure access granted",
	})
}

func main() {
	// Порт по умолчанию — 8080. Если в окружении задана переменная PORT — используем её.
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	if err := ratelimit.InitRedis("localhost:6379", "", 0); err != nil {
		log.Fatalf("Redis init error: %v", err)
	}

	// Создаём новый HTTP mux (маршрутизатор).
	mux := http.NewServeMux()

	// Экспорт метрик
	mux.Handle("/metrics", promhttp.Handler())

	// Регистрируем маршрут /ping (открытый).
	// Он оборачивается в Recovery и Logging middleware:
	// - Recovery ловит панику и возвращает 500 в JSON.
	// - Logging пишет в лог метод, путь, статус и время выполнения.
	mux.Handle("/ping", Chain(http.HandlerFunc(handlers.Ping),
		recovery.Recovery, logging.Logging, metrics.Metrics))

	// Регистрируем маршрут /secure (защищённый).
	// Здесь цепочка длиннее:
	// - Recovery: ловит паники.
	// - Logging: логирует запросы.
	// - Auth: проверяет заголовок Authorization (Bearer <token>).
	// - RateLimit: ограничивает частоту запросов.
	mux.Handle("/secure", Chain(http.HandlerFunc(handlers.Secure),
		recovery.Recovery, logging.Logging, metrics.Metrics, auth.Auth, ratelimit.RateLimit))

	// Пример использования SlidingWindow middleware на /sliding
	// Разрешаем 10 запросов на окно 1 секунда (1000 мс)
	limit := 10
	windowMS := int64(1000)
	mux.Handle("/sliding", Chain(http.HandlerFunc(handlers.Secure),
		recovery.Recovery,
		logging.Logging,
		metrics.Metrics,
		auth.Auth,
		slidingwindow.SlidingWindow(limit, windowMS),
	))

	mux.Handle("/order", Chain(http.HandlerFunc(orderHandler),
		recovery.Recovery,
		logging.Logging,
		metrics.Metrics,
		auth.Auth,
		distributedlock.RedisLockMiddleware("lock:order:123", 5000), // блокировка на 5 секунд
	))

	mux.Handle("/page", Chain(http.HandlerFunc(pageHandler),
		recovery.Recovery,
		logging.Logging,
		metrics.Metrics,
		pagecounter.CounterMiddleware("counter:page_view", 60), // TTL 60 секунд
	))

	mux.Handle("/process", Chain(http.HandlerFunc(processHandler),
		recovery.Recovery,
		logging.Logging,
		metrics.Metrics,
		queue.QueueMiddleware("queue:tasks", "queue:inprogress"),
	))

	// Защищённый маршрут с SessionMiddleware (TTL 10 секунд)
	mux.Handle("/secure2", Chain(
		http.HandlerFunc(SecureHandler),
		session.SessionMiddleware(10),
	))

	// Запускаем сервер и логируем адрес.
	log.Printf("listening on %s", addr)
	// ListenAndServe блокирует выполнение; при ошибке (например, порт занят) сервер завершится с log.Fatal.
	log.Fatal(http.ListenAndServe(addr, mux))
}
