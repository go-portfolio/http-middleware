package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-portfolio/http-middleware/internal/utils"
)

// Auth — middleware для проверки авторизации по токену.
// Читает ожидаемый токен из переменной окружения AUTH_TOKEN.
// Проверяет заголовок Authorization: "Bearer <token>".
// Если токен отсутствует или неверный → возвращает 401 Unauthorized.
// Если токен корректный → передаёт управление следующему handler'у.
func Auth(next http.Handler) http.Handler {
	// Получаем эталонный токен из окружения (например, AUTH_TOKEN=secret123).
	token := os.Getenv("AUTH_TOKEN")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем значение заголовка Authorization.
		auth := r.Header.Get("Authorization")

		// Проверяем: должен начинаться с "Bearer " и совпадать с ожидаемым токеном.
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
			// Если токен неверный — возвращаем 401 с JSON-ответом.
			utils.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		// Если токен валиден — вызываем следующий handler.
		next.ServeHTTP(w, r)
	})
}
