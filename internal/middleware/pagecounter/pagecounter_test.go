package pagecounter

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
)

func TestCounterMiddleware(t *testing.T) {
	// Инициализация Redis
	if err := InitRedisCounter("localhost:6379", "", 1); err != nil {
		t.Fatalf("failed to init redis: %v", err)
	}

	counterKey := "counter:page_view"

	// Очистка ключа перед тестом
	redisClientCounter.Del(ctxCounter, counterKey)

	// Простой handler
	handler := CounterMiddleware(counterKey, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		utils.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// 1. Первый запрос
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	counterHeader := w.Header().Get("X-Counter")
	count, err := strconv.Atoi(counterHeader)
	if err != nil {
		t.Fatalf("failed to parse X-Counter header: %v", err)
	}
	if count != 1 {
		t.Errorf("expected counter 1, got %d", count)
	}

	// 2. Второй запрос
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	counterHeader2 := w2.Header().Get("X-Counter")
	count2, err := strconv.Atoi(counterHeader2)
	if err != nil {
		t.Fatalf("failed to parse X-Counter header: %v", err)
	}
	if count2 != 2 {
		t.Errorf("expected counter 2, got %d", count2)
	}

	// 3. Ждём истечения TTL (1 секунда)
	time.Sleep(1100 * time.Millisecond)

	// 4. Третий запрос — ключ должен был сброситься
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req)

	counterHeader3 := w3.Header().Get("X-Counter")
	count3, err := strconv.Atoi(counterHeader3)
	if err != nil {
		t.Fatalf("failed to parse X-Counter header: %v", err)
	}
	if count3 != 1 {
		t.Errorf("expected counter 1 after TTL reset, got %d", count3)
	}
}
