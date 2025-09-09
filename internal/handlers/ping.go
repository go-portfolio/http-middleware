package handlers

import (
	"net/http"

	"github.com/go-portfolio/http-middleware/internal/utils"
)

// Ping — простой "healthcheck"-обработчик.
// Используется для проверки, что сервер жив и отвечает.
// Возвращает JSON {"message": "pong"} со статусом 200 OK.
func Ping(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]string{"message": "pong"})
}

// Secure — обработчик для защищённого маршрута.
// Доступ к этому маршруту возможен только через AuthMiddleware (требует токен).
// Возвращает JSON {"message": "secure ok"} со статусом 200 OK.
func Secure(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]string{"message": "secure ok"})
}
