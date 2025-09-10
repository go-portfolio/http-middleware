package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"github.com/go-portfolio/http-middleware/internal/metrics"
)

// Metrics — middleware для сбора метрик Prometheus по HTTP-запросам.
// Оборачивает обработчик, чтобы измерять количество запросов и время их выполнения.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now() // фиксируем время начала обработки запроса

		// Создаём обёртку вокруг ResponseWriter,
		// чтобы отслеживать статус код ответа
		rw := utils.NewResponseWriter(w)

		// Вызываем следующий обработчик в цепочке, передавая обёрнутый ResponseWriter
		next.ServeHTTP(rw, r)

		// Вычисляем длительность обработки запроса в секундах
		duration := time.Since(start).Seconds()

		// Получаем статус код ответа из обёрнутого ResponseWriter
		statusCode := rw.Status
		// Получаем путь и метод HTTP запроса
		path := r.URL.Path
		method := r.Method

		// Увеличиваем счётчик запросов с соответствующими лейблами
		metrics.HttpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(statusCode)).Inc()

		// Добавляем значение длительности запроса в гистограмму
		metrics.HttpRequestDuration.WithLabelValues(method, path).Observe(duration)
	})
}
