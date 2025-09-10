package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-portfolio/http-middleware/internal/handlers"
	"github.com/go-portfolio/http-middleware/internal/middleware"

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

func main() {
	// Порт по умолчанию — 8080. Если в окружении задана переменная PORT — используем её.
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
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
		middleware.Recovery, middleware.Logging, middleware.Metrics,))

	// Регистрируем маршрут /secure (защищённый).
	// Здесь цепочка длиннее:
	// - Recovery: ловит паники.
	// - Logging: логирует запросы.
	// - Auth: проверяет заголовок Authorization (Bearer <token>).
	// - RateLimit: ограничивает частоту запросов.
	mux.Handle("/secure", Chain(http.HandlerFunc(handlers.Secure),
		middleware.Recovery, middleware.Logging, middleware.Metrics, middleware.Auth, middleware.RateLimit))

	// Запускаем сервер и логируем адрес.
	log.Printf("listening on %s", addr)
	// ListenAndServe блокирует выполнение; при ошибке (например, порт занят) сервер завершится с log.Fatal.
	log.Fatal(http.ListenAndServe(addr, mux))
}
