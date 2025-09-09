# План проекта: HTTP Middleware
## 1. Цель проекта

Создать HTTP-сервис на Go с кастомными middleware, которые:

Логируют запросы и ответы.

Проверяют аутентификацию по токену.

Ограничивают скорость запросов (rate limiting).

Обрабатывают ошибки в едином формате JSON.

## Тесты
```
go test ./... -v
```

# План по добавлению к проекту функционала
Добавить Prometheus метрики (middleware для метрик).

Распределённый rate-limit через Redis или внешнее решение.

Structured logging (zerolog / zap).

Tracing (OpenTelemetry).

Graceful shutdown (context + server.Shutdown).