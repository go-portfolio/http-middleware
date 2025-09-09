package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
)

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := utils.NewResponseWriter(w)
		next.ServeHTTP(rw, r)
		duration := time.Since(start)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.Status, duration)
	})
}
