package handlers

import (
	"net/http"

	"github.com/go-portfolio/http-middleware/internal/utils"
)

func Ping(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]string{"message": "pong"})
}

func Secure(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]string{"message": "secure ok"})
}
