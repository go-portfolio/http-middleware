package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"github.com/go-portfolio/http-middleware/internal/metrics"
)

// Metrics — middleware для сбора метрик Prometheus по HTTP-запросам.
// Основная идея: обернуть обработчик, чтобы автоматически измерять ключевые показатели (метрики)
// без изменения логики самого обработчика — отделить мониторинг от бизнес-логики.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Фиксируем стартовое время до обработки запроса,
		// чтобы точно измерить сколько времени уходит на полный цикл обработки.
		start := time.Now()

		// Создаём обёртку вокруг ResponseWriter,
		// потому что стандартный ResponseWriter не позволяет напрямую узнать код ответа,
		// а статус — важная метрика для понимания поведения сервиса.
		rw := utils.NewResponseWriter(w)

		// Вызываем следующий обработчик, подставляя обёрнутый ResponseWriter,
		// чтобы мы могли перехватить статус и при этом пропустить все вызовы дальше.
		next.ServeHTTP(rw, r)

		// Считаем длительность выполнения запроса в секундах.
		// Это одна из ключевых метрик для оценки производительности.
		duration := time.Since(start).Seconds()

		// Извлекаем статус код из обёрнутого ResponseWriter.
		// Он показывает успешность или ошибочность обработки.
		statusCode := rw.Status

		// Получаем путь и метод запроса — чтобы метрики были более детализированными
		// и можно было понять, где именно происходят узкие места или ошибки.
		path := r.URL.Path
		method := r.Method

		// Увеличиваем счётчик запросов с метками метода, пути и статуса.
		// Это важно для подсчёта количества разных типов ответов на разные запросы.
		metrics.HttpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(statusCode)).Inc()

		// Добавляем наблюдение длительности запроса в гистограмму,
		// что позволяет анализировать распределение времени обработки и выявлять аномалии.
		metrics.HttpRequestDuration.WithLabelValues(method, path).Observe(duration)
	})
}
