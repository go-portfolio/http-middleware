package utils

import (
    "encoding/json"
    "net/http"
)

// ResponseWriter — обёртка над http.ResponseWriter, которая сохраняет статус ответа.
type ResponseWriter struct {
    http.ResponseWriter  // встроенный оригинальный ResponseWriter
    Status int           // поле для хранения кода статуса HTTP-ответа
}

// NewResponseWriter создаёт новый экземпляр ResponseWriter с дефолтным статусом 200 OK.
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
    return &ResponseWriter{ResponseWriter: w, Status: http.StatusOK}
}

// WriteHeader перехватывает вызов записи заголовка и сохраняет статус в структуре.
func (rw *ResponseWriter) WriteHeader(status int) {
    rw.Status = status              // сохраняем статус в поле структуры
    rw.ResponseWriter.WriteHeader(status)  // вызываем оригинальный метод записи заголовка
}

// JSON — вспомогательная функция для отправки JSON-ответа с заданным статусом и телом v.
func JSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json") // устанавливаем заголовок Content-Type
    w.WriteHeader(status)                              // устанавливаем HTTP статус ответа
    json.NewEncoder(w).Encode(v)                       // кодируем объект v в JSON и пишем в ответ
}
