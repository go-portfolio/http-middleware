package metrics

import "github.com/prometheus/client_golang/prometheus"

// HttpRequestsTotal — счетчик общего количества HTTP-запросов.
// Метки: method (HTTP метод), path (путь запроса), status (HTTP статус код).
var (
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Общее количество HTTP-запросов",
		},
		[]string{"method", "path", "status"},
	)

	// HttpRequestDuration — гистограмма длительности HTTP-запросов в секундах.
	// Метки: method (HTTP метод), path (путь запроса).
	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Длительность HTTP-запросов",
			Buckets: prometheus.DefBuckets, // Используются стандартные интервалы для гистограммы.
		},
		[]string{"method", "path"},
	)
)

// Init регистрирует метрики в Prometheus.
func Init() {
	prometheus.MustRegister(HttpRequestsTotal, HttpRequestDuration)
}
