package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-portfolio/http-middleware/internal/handlers"
	"github.com/go-portfolio/http-middleware/internal/middleware"
)

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func main() {
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	mux := http.NewServeMux()
	mux.Handle("/ping", Chain(http.HandlerFunc(handlers.Ping),
		middleware.Recovery, middleware.Logging))
	mux.Handle("/secure", Chain(http.HandlerFunc(handlers.Secure),
		middleware.Recovery, middleware.Logging, middleware.Auth, middleware.RateLimit))

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
