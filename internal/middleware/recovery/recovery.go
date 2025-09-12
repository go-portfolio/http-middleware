package recovery

import (
	"log"
	"net/http"

	"github.com/go-portfolio/http-middleware/internal/utils"
)

// Recovery — middleware-функция, которая перехватывает паники (panic) во время обработки HTTP-запросов
// и возвращает пользователю стандартный ответ об ошибке сервера (500 Internal Server Error).
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Используем defer для отложенного вызова анонимной функции, 
		// которая будет выполнена при выходе из текущей функции.
		defer func() {
			// recover() позволяет перехватить панику, если она произошла.
			if rec := recover(); rec != nil {
				// Логируем информацию о панике для последующего анализа.
				log.Printf("panic: %v", rec)

				// Отправляем клиенту JSON-ответ с кодом 500 и сообщением об ошибке.
				utils.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}
		}()

		// Передаём управление следующему обработчику в цепочке middleware.
		next.ServeHTTP(w, r)
	})
}
