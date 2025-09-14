package main

import (
	"context"
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
	"github.com/go-portfolio/http-middleware/internal/middleware/stateupdate"
	"github.com/go-portfolio/http-middleware/internal/utils"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func initProvider() (*sdktrace.TracerProvider, http.Handler, error) {
	reg := promclient.NewRegistry()

	exporter, err := otelprom.New(otelprom.WithRegisterer(reg))
	if err != nil {
		return nil, nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("http-middleware-app"),
		)),
	)
	otel.SetMeterProvider(meterProvider)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("http-middleware-app"),
		)),
	)
	otel.SetTracerProvider(tp)

	// Handler для /metrics
	promHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})

	return tp, promHandler, nil
}

// Middleware тип функции-обёртки
type Middleware func(http.Handler) http.Handler

// Chain оборачивает handler в цепочку middleware
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func orderHandler(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]string{"status": "order processed"})
}

func pageHandler(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"status": "page served",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func processHandler(w http.ResponseWriter, r *http.Request) {
	item := w.Header().Get("X-Queue-Item")
	utils.JSON(w, http.StatusOK, map[string]string{"status": "processed", "task": item})
}

func SecureHandler(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]string{"status": "secure access granted"})
}

func UpdateHandler(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]string{"status": "update applied"})
}

func main() {
	tp, metricsHandler, err := initProvider()
	if err != nil {
		log.Fatalf("failed to init provider: %v", err)
	}
	defer func() { _ = tp.Shutdown(context.Background()) }()

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	if err := ratelimit.InitRedis("localhost:6379", "", 0); err != nil {
		log.Fatalf("Redis init error: %v", err)
	}

	mux := http.NewServeMux()

	// --- /metrics через OpenTelemetry + Prometheus
	mux.Handle("/metrics", metricsHandler)

	mux.Handle("/ping", Chain(http.HandlerFunc(handlers.Ping),
		recovery.Recovery,
		logging.Logging,
		metrics.Metrics))

	mux.Handle("/secure", Chain(http.HandlerFunc(handlers.Secure),
		recovery.Recovery,
		logging.Logging,
		metrics.Metrics,
		auth.Auth,
		ratelimit.RateLimit))

	limit := 10
	windowMS := int64(1000)
	mux.Handle("/sliding", Chain(http.HandlerFunc(handlers.Secure),
		recovery.Recovery,
		logging.Logging,
		metrics.Metrics,
		auth.Auth,
		slidingwindow.SlidingWindow(limit, windowMS)))

	mux.Handle("/order", Chain(http.HandlerFunc(orderHandler),
		recovery.Recovery,
		logging.Logging,
		metrics.Metrics,
		auth.Auth,
		distributedlock.RedisLockMiddleware("lock:order:123", 5000)))

	mux.Handle("/page", Chain(http.HandlerFunc(pageHandler),
		recovery.Recovery,
		logging.Logging,
		metrics.Metrics,
		pagecounter.CounterMiddleware("counter:page_view", 60)))

	mux.Handle("/process", Chain(http.HandlerFunc(processHandler),
		recovery.Recovery,
		logging.Logging,
		metrics.Metrics,
		queue.QueueMiddleware("queue:tasks", "queue:inprogress")))

	mux.Handle("/secure2", Chain(http.HandlerFunc(SecureHandler),
		session.SessionMiddleware(10)))

	mux.Handle("/update", Chain(http.HandlerFunc(UpdateHandler),
		stateupdate.StateUpdateMiddleware("state:item123", "old", "new")))

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
