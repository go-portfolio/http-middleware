package slidingwindow

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
)

func TestSlidingWindowMiddleware(t *testing.T) {
	if err := InitRedisSliding("localhost:6379", "", 1); err != nil {
		t.Fatalf("failed to init redis: %v", err)
	}

	// Очистка ключа перед тестом
	key := "sliding_rate:127.0.0.1"
	redisClientSliding.Del(ctxSliding, key)

	handler := SlidingWindow(5, 1000)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		utils.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	ip := "127.0.0.1:12345"

	// Отправляем 5 запросов подряд
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, w.Code)
		}

		time.Sleep(10 * time.Millisecond) // маленькая задержка для корректного счёта
	}

	// 6-й запрос — должен быть заблокирован
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429 after exceeding limit, got %d", w.Code)
	}

	// Ждём пока окно времени пройдёт
	time.Sleep(1100 * time.Millisecond)

	// После ожидания запрос снова проходит
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 after window reset, got %d", w.Code)
	}
}

