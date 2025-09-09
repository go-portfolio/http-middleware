package utils

import (
    "encoding/json"
    "net/http"
)

type ResponseWriter struct {
    http.ResponseWriter
    Status int
}

func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
    return &ResponseWriter{ResponseWriter: w, Status: http.StatusOK}
}

func (rw *ResponseWriter) WriteHeader(status int) {
    rw.Status = status
    rw.ResponseWriter.WriteHeader(status)
}

func JSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}
