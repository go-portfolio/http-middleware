package distributedlock

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
)

func TestRedisLockMiddleware(t *testing.T) {
	// Инициализация Redis для теста
	if err := InitRedisLock("localhost:6379", "", 1); err != nil {
		t.Fatalf("failed to init redis: %v", err)
	}

	lockKey := "lock:order:test"

	// Очистка ключа перед тестом
	redisClientLock.Del(ctxLock, lockKey)

	// Простая заглушка handler
	handler := RedisLockMiddleware(lockKey, 500)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		utils.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	ip := "127.0.0.1:12345"

	// 1. Первый запрос — должен пройти
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = ip
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("expected 200 OK for first request, got %d", w1.Code)
	}

	// 2. Второй запрос сразу — должен быть заблокирован
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = ip
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for locked resource, got %d", w2.Code)
	}

	// 3. Ждём истечения TTL (500ms) и повторяем запрос
	time.Sleep(600 * time.Millisecond)
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.RemoteAddr = ip
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("expected 200 OK after TTL expiration, got %d", w3.Code)
	}
}
