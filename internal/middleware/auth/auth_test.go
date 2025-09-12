package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	// Устанавливаем переменную окружения с токеном,
	// который должен использовать AuthMiddleware для проверки.
	os.Setenv("AUTH_TOKEN", "tkn")

	// Создаём базовый handler, который возвращает 200 OK,
	// если его вызов прошёл через middleware.
	handler := Auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// --- Тест №1: запрос без токена ---
	// Делаем HTTP-запрос на защищённый маршрут /secure.
	req := httptest.NewRequest("GET", "/secure", nil)
	rr := httptest.NewRecorder()

	// Прогоняем запрос через middleware.
	handler.ServeHTTP(rr, req)

	// Ожидаем статус 401 Unauthorized.
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}

	// --- Тест №2: запрос с правильным токеном ---
	// Делаем новый HTTP-запрос.
	req = httptest.NewRequest("GET", "/secure", nil)

	// Добавляем заголовок Authorization с правильным токеном.
	req.Header.Set("Authorization", "Bearer tkn")

	// Новый recorder для фиксации ответа.
	rr = httptest.NewRecorder()

	// Прогоняем запрос через middleware.
	handler.ServeHTTP(rr, req)

	// Ожидаем статус 200 OK, так как токен верный.
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}
