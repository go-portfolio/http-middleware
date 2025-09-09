package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-portfolio/http-middleware/internal/utils"
	"golang.org/x/time/rate"
)

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var clients = make(map[string]*client)
var mu sync.Mutex

func getClientLimiter(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()
	c, exists := clients[ip]
	if !exists {
		limiter := rate.NewLimiter(1, 5) // 1 req/sec, burst 5 — настройте под себя
		clients[ip] = &client{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	c.lastSeen = time.Now()
	return c.limiter
}

func cleanupClients() {
	for {
		time.Sleep(time.Minute)
		mu.Lock()
		for ip, c := range clients {
			if time.Since(c.lastSeen) > 3*time.Minute {
				delete(clients, ip)
			}
		}
		mu.Unlock()
	}
}

func RateLimit(next http.Handler) http.Handler {
	go cleanupClients()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		limiter := getClientLimiter(ip)
		if !limiter.Allow() {
			utils.JSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
