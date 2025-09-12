package logging

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// responseWriterWrapper оборачивает http.ResponseWriter,
// чтобы отслеживать HTTP статус.
type responseWriterWrapper struct {
	http.ResponseWriter
	status int
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Logging — middleware для структурированного логирования HTTP запросов
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapper := &responseWriterWrapper{ResponseWriter: w, status: http.StatusOK}

		// Перед обработкой запроса логируем базовую информацию
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Msg("incoming request")

		// Выполняем основной handler
		next.ServeHTTP(wrapper, r)

		// После обработки логируем результат
		event := log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Int("status", wrapper.status).
			Dur("duration", time.Since(start))

		// Если есть заголовок X-User-ID (или session), добавляем в лог
		if userID := r.Header.Get("X-User-ID"); userID != "" {
			event = event.Str("user_id", userID)
		}

		// Логи ошибок для статусов >= 400
		if wrapper.status >= 400 {
			event.Msg("request completed with error")
		} else {
			event.Msg("request completed successfully")
		}
	})
}
