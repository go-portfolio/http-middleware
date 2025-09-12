package logging

import (
	"log"
	"net/http"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
)

// Logging — middleware для логирования HTTP-запросов.
// Оно фиксирует:
//   - метод (GET, POST и т. д.),
//   - путь (/ping, /secure),
//   - статус ответа (200, 401, 500 и т. д.),
//   - время выполнения запроса.
//
// Логирование помогает анализировать нагрузку, отлавливать ошибки и понимать,
// как сервер обрабатывает запросы.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Запоминаем время начала обработки запроса.
		start := time.Now()

		// Оборачиваем ResponseWriter, чтобы иметь возможность получить статус ответа.
		rw := utils.NewResponseWriter(w)

		// Передаём управление следующему handler'у.
		next.ServeHTTP(rw, r)

		// Считаем время обработки запроса.
		duration := time.Since(start)

		// Логируем метод, путь, статус и время выполнения.
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.Status, duration)
	})
}
