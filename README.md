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

ASCII-схема, которая показывает поток данных  

                   ┌───────────────────────┐
                   │       Client          │
                   │   (браузер/curl)      │
                   └───────────┬───────────┘
                               │ HTTP-запрос
                               ▼
                   ┌───────────────────────┐
                   │  RecoveryMiddleware    │
                   │ (ловит панику, 500)    │
                   └───────────┬───────────┘
                               │
                               ▼
                   ┌───────────────────────┐
                   │  LoggingMiddleware     │
                   │ (логирует метод, путь, │
                   │ статус, время)         │
                   └───────────┬───────────┘
                               │
                               ▼
                   ┌───────────────────────┐
                   │   AuthMiddleware       │
                   │ (проверяет токен в     │
                   │ Authorization header)  │
                   └───────────┬───────────┘
                               │
                               ▼
                   ┌───────────────────────┐
                   │ RateLimitMiddleware    │
                   │ (ограничение частоты   │
                   │ запросов на IP)        │
                   └───────────┬───────────┘
                               │
                               ▼
                   ┌───────────────────────┐
                   │      Handlers          │
                   │   /ping   /secure      │
                   └───────────┬───────────┘
                               │
                               ▼
                   ┌───────────────────────┐
                   │     Response JSON      │
                   │ {"message": "..."}     │
                   └───────────────────────┘


👉 У маршрута /ping цепочка короче — там нет Auth и RateLimit, только Recovery + Logging.
👉 У /secure подключены все middleware: Recovery → Logging → Auth → RateLimit → Handler.

# HTTP Middleware с Redis

Этот проект содержит набор полезных HTTP middleware на Go, использующих Redis для реализации различных сценариев ограничения запросов, управления сессиями, очередей и атомарных операций.  

## Содержание

- [Simple Rate Limiting](#simple-rate-limiting)  
- [Sliding Window Rate Limiting](#sliding-window-rate-limiting)  
- [Distributed Lock Middleware](#distributed-lock-middleware)  
- [Page Counter Middleware](#page-counter-middleware)  
- [Queue Processing Middleware](#queue-processing-middleware)  
- [Session TTL Middleware](#session-ttl-middleware)  
- [Atomic State Update Middleware](#atomic-state-update-middleware)  

---

## Simple Rate Limiting

**Пакет:** `ratelimit`

**Описание:**  
Middleware для простого ограничения количества запросов на основе IP с фиксированным окном времени.  

**Принцип работы:**  
- Lua скрипт хранит счётчик запросов в Redis с TTL.  
- Если количество запросов за окно превышает лимит — возвращается `429 Too Many Requests`.  
- Используется подход "fail-open": при ошибках Redis запрос пропускается.  

**Использование:**

```go
mux.Handle("/api", ratelimit.RateLimit(http.HandlerFunc(MyHandler)))
```
## Sliding Window Rate Limiting

**Пакет:** `slidingwindow`

**Описание:**  
Ограничение частоты запросов с использованием скользящего окна.

**Принцип работы:**  
- В Redis хранится отсортированное множество меток времени запросов.  
- Удаляются старые записи за пределами окна.  
- Если количество запросов меньше лимита → разрешён запрос.

**Пример использования:**

```go
mux.Handle("/api", slidingwindow.SlidingWindow(5, 1000)(http.HandlerFunc(MyHandler)))
```

## Distributed Lock Middleware

**Пакет:** `distributedlock`

**Описание:**  
Middleware для реализации распределённой блокировки через Redis. Полезно для защиты ресурсов от одновременного выполнения (например, заказов или обработки платежей).

**Принцип работы:**  
- Lua скрипт устанавливает ключ блокировки с TTL, если он ещё не занят.  
- Если блокировка занята — возвращается `429 Too Many Requests`.

**Использование:**

```go
mux.Handle("/order", distributedlock.RedisLockMiddleware("lock:order:123", 5000)(http.HandlerFunc(OrderHandler)))
```
## Page Counter Middleware

**Пакет:** `pagecounter`

**Описание:**  
Middleware для подсчёта посещений страниц с хранением счётчика в Redis.

**Принцип работы:**  
- Lua скрипт инкрементирует счётчик и устанавливает TTL для автоматической очистки.  
- Текущее значение счётчика добавляется в заголовок ответа `X-Counter`.

**Использование:**

```go
mux.Handle("/page", pagecounter.CounterMiddleware("counter:page_view", 60)(http.HandlerFunc(PageHandler)))
```

## Queue Processing Middleware

**Пакет:** `queue`

**Описание:**  
Middleware для обработки очередей задач через Redis. Перемещает элемент из исходной очереди в очередь обработки (in-progress) с помощью команды `RPOPLPUSH`.

**Принцип работы:**  
- Берём элемент с конца исходной очереди.  
- Добавляем его в очередь обработки.  
- Заголовок `X-Queue-Item` содержит текущий элемент.  
- Если очередь пуста — возвращается `204 No Content`.

**Использование:**

```go
mux.Handle("/task", queue.QueueMiddleware("queue:tasks", "queue:processing")(http.HandlerFunc(TaskHandler)))
```
## Session TTL Middleware

**Пакет:** `session`

**Описание:**  
Middleware для управления сессиями: продление TTL ключа в Redis при каждом запросе.

**Принцип работы:**  
- Извлекает `session_id` из cookie.  
- Lua скрипт проверяет наличие ключа и продлевает TTL.  
- Если сессия не найдена — возвращается `401 Unauthorized`.

**Использование:**

```go
mux.Handle("/secure", session.SessionMiddleware(10)(http.HandlerFunc(SecureHandler)))
```



## Atomic State Update Middleware

**Пакет:** `stateupdate`

**Описание:**  
Middleware для атомарного обновления значения ключа в Redis по принципу compare-and-set.

**Принцип работы:**  
- Lua скрипт сравнивает текущее значение ключа с ожидаемым.  
- Если совпадает — устанавливает новое значение.  
- Если не совпадает — возвращает `409 Conflict`.

**Использование:**

```go
mux.Handle("/update", stateupdate.StateUpdateMiddleware("state:item123", "old", "new")(http.HandlerFunc(UpdateHandler)))
```

## Как запускать

1. Установите Redis локально или используйте Docker:

```bash
docker run -p 6379:6379 redis
```
Настройте подключения к Redis в каждом middleware.

Запустите сервер:

```bash
go run main.go
```

## Пример использования middleware для защищённого маршрута

```go
mux.Handle("/secure", Chain(
    http.HandlerFunc(handlers.Secure),
    recovery.Recovery,   // ловим паники и возвращаем 500
    logging.Logging,     // структурированное логирование запроса
    metrics.Metrics,     // сбор метрик для Prometheus
    auth.Auth,           // проверка заголовка Authorization
    ratelimit.RateLimit, // ограничение частоты запросов по IP
))
```
Пояснение порядка middleware:

Recovery — первым, чтобы перехватывать любые паники и возвращать корректный HTTP-ответ.

Logging — сразу после Recovery, чтобы фиксировать все запросы, включая те, где произошла ошибка.

Metrics — собирает статистику по запросам.

Auth — проверяет авторизацию.

RateLimit — ограничивает частоту запросов.

Handler (handlers.Secure) — выполняется в самом конце, когда все проверки и обёртки пройдены.

## Structured Logging Middleware

**Пакет:** `logging`

**Описание:**  
Middleware для структурированного логирования HTTP-запросов с использованием [`zerolog`](https://github.com/rs/zerolog). Позволяет удобно собирать логи с полями `method`, `path`, `status`, `duration`, `user_id` и другими для последующего анализа.

## Метрики и Prometheus

Сервер автоматически собирает метрики с помощью **OpenTelemetry** и отдаёт их в формате **Prometheus**.

- Эндпоинт `/metrics` используется Prometheus для сбора метрик:

```bash
curl http://localhost:8080/metrics

## Сбор метрик через Prometheus

Сервер автоматически собирает метрики с помощью OpenTelemetry и отдаёт их на эндпоинт `/metrics`.  

Prometheus опрашивает сервер по конфигурации `prometheus.yml`

## Сбор метрик и порядок middleware

Метрики собираются через `metrics.Metrics` middleware и отдаются на эндпоинт `/metrics` для Prometheus.

**Важно:** порядок middleware влияет на метрики:

- `metrics.Metrics` должен располагаться **после middleware**, которые могут менять результат запроса (например, `Recovery` или `Logging`).  
- Тогда в метриках будут корректные значения статуса, длительности и ошибок.  
- Middleware, расположенные **после `metrics.Metrics`**, не будут учитываться в метриках.  

Пример правильного порядка:

```go
Chain(handler,
    recovery.Recovery,
    logging.Logging,
    metrics.Metrics,
)

## Метрики паник (`Recovery` middleware)

- Middleware `Recovery` перехватывает паники в хэндлерах, **которые она оборачивает**, и возвращает клиенту JSON с ошибкой 500.  
- Для каждой перехваченной паники увеличивается метрика `recovery_panic_total`. Также ведётся учёт успешно обработанных запросов (`recovery_success_total`) и времени выполнения (`recovery_duration_seconds`).  
- **Важно:** если внутри хэндлера реализован собственный `recover`, паника **не дойдёт до middleware**, и метрика `recovery_panic_total` **не увеличится**.  
- Чтобы отслеживать все паники глобально, можно обернуть весь `mux` в `Recovery`.



