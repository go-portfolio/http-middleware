package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
)

func TestRateLimitMiddleware(t *testing.T) {
	// Шаг 0: подключаемся к Redis.
	// Мне нужен реальный Redis, чтобы проверить интеграцию middleware с внешним хранилищем.
	// Если Redis не отвечает, тест смысла не имеет, поэтому падаем сразу.
	if err := InitRedis("localhost:6379", "", 1); err != nil {
		t.Fatalf("failed to init redis: %v", err)
	}

	// Шаг 1: создаём простой HTTP handler.
	// Я хочу, чтобы middleware мог "пропустить" запрос дальше,
	// поэтому нужен рабочий handler, который всегда возвращает 200 OK.
	handler := RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		utils.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	// Шаг 2: фиксируем IP для теста.
	// RateLimit работает по IP, поэтому используем один и тот же IP для проверки лимита.
	ip := "127.0.0.1:12345"

	// Шаг 3: отправляем количество запросов равное burstLimit.
	// Я ожидаю, что все эти запросы пройдут, потому что лимит ещё не превышен.
	for i := 0; i < burstLimit; i++ {
		// Создаём новый HTTP запрос
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip

		// Используем httptest.Recorder, чтобы поймать ответ
		w := httptest.NewRecorder()

		// Отправляем запрос через middleware
		handler.ServeHTTP(w, req)

		// Проверяем статус ответа
		if w.Code != http.StatusOK {
			// Если первый burstLimit запросов падает — это баг
			t.Errorf("request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	// Шаг 4: отправляем ещё один запрос — он должен быть заблокирован
	// Логика: burstLimit + 1 → превышение лимита → 429 Too Many Requests
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429 after exceeding limit, got %d", w.Code)
	}

	// Шаг 5: ждём, пока истечёт окно времени лимита.
	// После этого Redis должен сбросить ключ, и новый запрос должен пройти.
	time.Sleep(time.Duration(windowSeconds) * time.Second)

	// Шаг 6: проверяем, что после ожидания лимит сбросился
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		// Если здесь не 200 OK — значит windowSeconds работает неправильно
		t.Errorf("expected status 200 after window reset, got %d", w.Code)
	}
}
