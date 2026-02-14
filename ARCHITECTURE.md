# Архитектура CDEK Go Client

## Обзор

CDEK Go Client построен на принципах **чистой архитектуры** с соблюдением регламента Metrica Pro (go_project_rules_v_1_3.5.md).

## Основные принципы

### 1. Именование (раздел 3.1 регламента)

- **Пакеты:** lowercase (`cdek`)
- **Go-файлы:** snake_case (`auth.go`, `components.go`)
- **Типы (экспорт):** PascalCase (`Manager`, `Service`, `Config`)
- **Типы (приватные):** camelCase (`tokenCache`, `costCalculator`)
- **Функции (экспорт):** PascalCase (`NewManager`, `GetClient`)
- **Функции (приватные):** camelCase (`parseResponse`, `refreshToken`)
- **Компоненты:** camelCase (`tokenCache`, `requestBuilder`, `responseParser`)
- **Константы:** UPPER_SNAKE_CASE (`URLProduction`, `URLSandbox`)

### 2. Компоненты (раздел 4.6 регламента)

Повторно используемая бизнес-логика вынесена в компоненты с именами в `camelCase`:

```go
// components.go

type tokenCache struct { ... }      // Кеширование OAuth2 токенов
type requestBuilder struct { ... }  // Построение HTTP запросов
type responseParser struct { ... }  // Парсинг API ответов
type orderValidator struct { ... }  // Валидация заказов
type costCalculator struct { ... }  // Расчет стоимости
```

## Архитектура слоев

```
┌─────────────────────────────────────────────────────────────┐
│                       User Code                             │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│  Service Layer (service.go) - High-level API                │
│  - Удобный интерфейс                                        │
│  - Валидация бизнес-логики                                  │
│  - Обработка ошибок                                         │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│  Components (components.go) - Повторная логика              │
│  ├─ costCalculator: расчет стоимости                        │
│  ├─ orderValidator: валидация заказов                       │
│  └─ responseParser: парсинг ответов                         │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│  Auth Layer (auth.go) - OAuth2 + кеширование               │
│  ├─ tokenCache: кеш токенов (thread-safe)                  │
│  ├─ requestBuilder: построение запросов                     │
│  └─ GetToken(): автоматическое обновление                   │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│  Client Layer (client.go) - Низкоуровневый HTTP             │
│  - Автогенерирован из OpenAPI                               │
│  - 40+ endpoints CDEK API                                   │
│  - НЕ РЕДАКТИРОВАТЬ ВРУЧНУЮ                                 │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                      CDEK API                                │
│  https://api.cdek.ru (production)                           │
│  https://api.edu.cdek.ru (sandbox)                          │
└─────────────────────────────────────────────────────────────┘
```

## Компоненты системы

### 1. Config (`config.go`)

**Назначение:** Управление конфигурацией для множественных аккаунтов CDEK.

**Ключевые типы:**
```go
type AccountConfig struct {
    Name         string        // Уникальное имя аккаунта
    ClientID     string        // OAuth2 client ID
    ClientSecret string        // OAuth2 client secret
    BaseURL      string        // URLProduction или URLSandbox
    Timeout      time.Duration // HTTP timeout
    MaxRetries   int           // Количество повторов
}

type Config struct {
    Accounts       []AccountConfig // Список аккаунтов
    DefaultAccount string          // Имя аккаунта по умолчанию
}
```

**Основные методы:**
- `Validate()` - валидация конфигурации
- `GetAccount(name)` - получение конфигурации аккаунта
- `DefaultConfig(clientID, secret)` - создание дефолтной конфигурации

### 2. Auth (`auth.go`)

**Назначение:** OAuth2 клиент с автоматическим обновлением и кешированием токенов.

**Thread-safety:** Использует `sync.RWMutex` через компонент `tokenCache`.

**Ключевые типы:**
```go
type AuthenticatedClient struct {
    config     *AccountConfig
    client     *Client        // Автогенерированный клиент
    httpClient *http.Client

    // Компоненты (camelCase по регламенту 4.6)
    cache   *tokenCache      // Кеширование токенов
    parser  *responseParser  // Парсинг ответов
    builder *requestBuilder  // Построение запросов
}
```

**Основные методы:**
- `GetToken(ctx)` - получение токена (с кешированием)
- `Do(ctx, method, path, body)` - HTTP запрос с авторизацией
- `DoWithResponse(ctx, method, path, body, result)` - запрос + парсинг

**Кеширование токенов:**
```go
// tokenCache - thread-safe кеш
type tokenCache struct {
    mu          sync.RWMutex
    accessToken string
    expiresAt   time.Time
}

func (tc *tokenCache) get() (string, bool)
func (tc *tokenCache) set(token string, expiresIn int)
```

### 3. Manager (`manager.go`)

**Назначение:** Менеджер для работы с несколькими аккаунтами одновременно.

**Thread-safety:** Использует `sync.RWMutex` для защиты доступа к клиентам.

**Ключевые методы:**
- `NewManager(config)` - создание менеджера
- `GetClient(name)` - получение клиента по имени
- `GetDefaultClient()` - получение дефолтного клиента
- `ListAccounts()` - список аккаунтов
- `AddAccount(config)` - добавление аккаунта в runtime
- `RemoveAccount(name)` - удаление аккаунта
- `HealthCheck(ctx)` - параллельная проверка всех аккаунтов

**Параллельный HealthCheck:**
```go
func (m *Manager) HealthCheck(ctx context.Context) map[string]error {
    // Проверяет все аккаунты параллельно с помощью goroutines
    // Возвращает map[accountName]error
}
```

### 4. Components (`components.go`)

**Назначение:** Повторно используемые компоненты бизнес-логики (по регламенту 4.6).

**Компоненты:**

#### tokenCache (camelCase)
```go
type tokenCache struct {
    mu          sync.RWMutex
    accessToken string
    expiresAt   time.Time
}
```
- Thread-safe кеш OAuth2 токенов
- Автоматическая проверка срока действия
- Запас 1 минута для обновления

#### requestBuilder (camelCase)
```go
type requestBuilder struct {
    baseURL string
    headers map[string]string
}
```
- Построение HTTP запросов
- Автоматическое добавление заголовков
- Fluent interface: `builder.withAuthorization().build()`

#### responseParser (camelCase)
```go
type responseParser struct{}
```
- Парсинг JSON ответов
- Обработка HTTP ошибок
- Поддержка binary данных (PDF, изображения)

#### orderValidator (camelCase)
```go
type orderValidator struct{}
```
- Валидация данных заказов
- Проверка обязательных полей
- Generic validation

#### costCalculator (camelCase)
```go
type costCalculator struct {
    client *AuthenticatedClient
    parser *responseParser
}
```
- Расчет стоимости доставки
- Обертка над низкоуровневым API

### 5. Service (`service.go`)

**Назначение:** High-level API обертка для упрощения использования.

**Структура:**
```go
type Service struct {
    client *AuthenticatedClient

    // Компоненты (camelCase по регламенту 4.6)
    costCalculator *costCalculator
    orderValidator *orderValidator
    parser         *responseParser
}
```

**Методы:**
- `NewService(client)` - создание сервиса
- `GetClient()` - получение базового клиента
- `HealthCheck(ctx)` - проверка доступности API

**Будущие методы** (после завершения service layer):
- `CalculateCost(ctx, req)` - расчет стоимости
- `CreateOrder(ctx, req)` - создание заказа
- `GetOrder(ctx, uuid)` - получение информации о заказе
- `TrackOrder(ctx, uuid)` - отслеживание статуса
- `PrintWaybill(ctx, uuid)` - печать накладной
- `PrintBarcode(ctx, uuid)` - печать этикетки
- `ListDeliveryPoints(ctx, req)` - список ПВЗ

### 6. Errors (`errors.go`)

**Назначение:** Типизированные ошибки для удобной обработки.

**Стандартные ошибки:**
```go
var (
    ErrNotFound          = fmt.Errorf("cdek: not found")
    ErrUnauthorized      = fmt.Errorf("cdek: unauthorized")
    ErrInvalidRequest    = fmt.Errorf("cdek: invalid request")
    ErrRateLimitExceeded = fmt.Errorf("cdek: rate limit exceeded")
    ErrServerError       = fmt.Errorf("cdek: server error")
)
```

**ErrorResponse:**
```go
type ErrorResponse struct {
    Errors []ErrorDetail `json:"errors"`
}

type ErrorDetail struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

**HTTP статусы → Typed errors:**
- 401, 403 → `ErrUnauthorized`
- 404 → `ErrNotFound`
- 400, 422 → `ErrInvalidRequest`
- 429 → `ErrRateLimitExceeded`
- 500, 502, 503 → `ErrServerError`

### 7. Client (`client.go`)

**Назначение:** Низкоуровневый HTTP клиент, автогенерированный из OpenAPI спецификации.

**⚠️ НЕ РЕДАКТИРОВАТЬ ВРУЧНУЮ!**

**Генерация:**
```bash
oapi-codegen -config oapi-codegen-config.yaml api/cdek-api.yaml
```

**Содержит:**
- 40+ методов для всех CDEK endpoints
- 178+ типов данных (DTO)
- Request/Response обертки
- Валидация параметров

**Использование:**
```go
client, _ := NewClient(baseURL, WithHTTPClient(httpClient))
resp, err := client.GetDeliverypointsWithResponse(ctx, params)
```

## Схема работы

### Создание заказа (пример)

```
User Code
   ↓
service.CreateOrder(ctx, req)
   ↓
orderValidator.validate(req)  ← Валидация
   ↓
client.GetToken(ctx)  ← Получение токена (с кешем)
   ↓
requestBuilder.build()  ← Построение запроса
   ↓
httpClient.Do(req)  ← HTTP запрос
   ↓
responseParser.parse()  ← Парсинг ответа
   ↓
return Order
```

## Thread-Safety

### tokenCache
- Использует `sync.RWMutex`
- Read lock для `get()`
- Write lock для `set()` и `clear()`

### Manager
- Использует `sync.RWMutex`
- Read lock для `GetClient()`, `ListAccounts()`
- Write lock для `AddAccount()`, `RemoveAccount()`
- Параллельный `HealthCheck()` с `sync.WaitGroup`

## Тестирование

### Структура тестов

```
pkg/cdek/
├── config_test.go       # Тесты конфигурации
├── auth_test.go         # Тесты авторизации
├── manager_test.go      # Тесты менеджера
├── components_test.go   # Тесты компонентов
├── service_test.go      # Тесты сервисного слоя
└── errors_test.go       # Тесты обработки ошибок
```

### Покрытие

- **Общее покрытие:** 93.6% (вручную написанный код)
- **Требование регламента:** >= 70% для domain кода ✅

### Запуск тестов

```bash
# Все тесты
go test -v ./pkg/cdek/...

# С race detector
go test -race ./pkg/cdek/...

# С покрытием
go test -coverprofile=coverage.out ./pkg/cdek/...
go tool cover -html=coverage.out
```

## Регенерация клиента

### Процесс

1. Обновление OpenAPI спецификации: `api/cdek-api.yaml`
2. Запуск генератора: `oapi-codegen -config oapi-codegen-config.yaml api/cdek-api.yaml`
3. Проверка компиляции: `go build ./pkg/cdek/...`
4. Запуск тестов: `go test ./pkg/cdek/...`
5. Линтинг: `golangci-lint run ./pkg/cdek/...`

### Конфигурация генератора

```yaml
# oapi-codegen-config.yaml
package: cdek
generate:
  models: true
  client: true
  embedded-spec: false
output: pkg/cdek/client.go
output-options:
  skip-prune: true
```

## Соответствие регламенту

### ✅ go_project_rules_v_1_3.5.md

| Требование | Статус | Реализация |
|------------|--------|------------|
| Go >= 1.25 | ✅ | Go 1.26 |
| Именование: camelCase для компонентов | ✅ | tokenCache, costCalculator, etc. |
| Именование: PascalCase для экспорта | ✅ | Manager, Service, Config |
| Test coverage >= 40% | ✅ | 93.6% |
| Domain coverage >= 70% | ✅ | 93.6% |
| golangci-lint | ✅ | 0 issues |
| Thread-safe | ✅ | sync.RWMutex в cache и manager |
| OpenAPI source of truth | ✅ | auto-generated client.go |
| Чистая архитектура | ✅ | Layered architecture |

---

**Последнее обновление:** 2026-02-15
