# CDEK Go Client - Развертывание и настройка

Руководство по развертыванию cdek-go библиотеки в production окружении.

## Содержание

- [Требования](#требования)
- [Получение credentials](#получение-credentials)
- [Настройка окружения](#настройка-окружения)
- [Development](#development)
- [Staging](#staging)
- [Production](#production)
- [Мониторинг](#мониторинг)
- [Безопасность](#безопасность)
- [Troubleshooting](#troubleshooting)

---

## Требования

### Системные требования

- **Go:** 1.22+ (рекомендуется 1.23)
- **OS:** Linux, macOS, Windows
- **RAM:** Минимум 256MB для библиотеки
- **Network:** Доступ к https://api.cdek.ru

### Зависимости

```go
require (
    github.com/metrica-pro/cdek-go v0.2.0
    github.com/sony/gobreaker/v2 v2.0.0
    github.com/rs/zerolog v1.33.0
    // ... другие зависимости вашего проекта
)
```

---

## Получение credentials

### 1. Регистрация в CDEK

1. Перейдите на [cdek.ru](https://cdek.ru/)
2. Зарегистрируйтесь как юридическое лицо или ИП
3. Заключите договор с CDEK (онлайн или офлайн)

### 2. Создание API приложения

1. Войдите в [личный кабинет CDEK](https://lk.cdek.ru/)
2. Перейдите в **Настройки** → **Интеграции** → **API**
3. Нажмите **Создать приложение**
4. Заполните данные:
   - Название: "Ваша ERP/CRM система"
   - Описание: "Интеграция для автоматизации доставок"
   - Redirect URL: (не требуется для Client Credentials flow)
5. После создания получите:
   - **Client ID** (например: `Gh2kvD9jFHpP9DiRxE343ViJWlMVeMsx`)
   - **Client Secret** (например: `gq5itaXV6uD728jcaeFGRJadaC8xRXGs`)

### 3. Настройка прав доступа

Убедитесь что приложение имеет права на:
- ✅ Создание заказов
- ✅ Получение информации о заказах
- ✅ Печать документов
- ✅ Отслеживание статусов
- ✅ Работа со справочниками

---

## Настройка окружения

### Environment Variables

Создайте файл `.env` (НЕ коммитить в git!):

```bash
# CDEK API Credentials
CDEK_CLIENT_ID=your-client-id
CDEK_CLIENT_SECRET=your-client-secret

# Optional: Production URL (по умолчанию https://api.cdek.ru)
CDEK_BASE_URL=https://api.cdek.ru

# Optional: Warehouse city code
CDEK_WAREHOUSE_CITY=44  # Москва

# Optional: Circuit Breaker settings
CDEK_BREAKER_MAX_REQUESTS=5
CDEK_BREAKER_TIMEOUT=60s
```

### Загрузка переменных окружения

```go
package main

import (
    "log"
    "os"

    "github.com/joho/godotenv"
    "github.com/metrica-pro/cdek-go/pkg/cdek"
)

func main() {
    // Загрузить .env файл (только для development)
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found")
    }

    // Создать конфигурацию
    config := cdek.DefaultConfig(
        os.Getenv("CDEK_CLIENT_ID"),
        os.Getenv("CDEK_CLIENT_SECRET"),
    )

    // ... остальной код
}
```

### Конфигурация в коде

```go
// config/cdek.go

package config

import (
    "os"
    "time"

    "github.com/metrica-pro/cdek-go/pkg/cdek"
)

type CDEKConfig struct {
    ClientID          string
    ClientSecret      string
    BaseURL           string
    WarehouseCityCode int32

    // Circuit Breaker
    BreakerMaxRequests uint32
    BreakerTimeout     time.Duration
}

func LoadCDEKConfig() *CDEKConfig {
    return &CDEKConfig{
        ClientID:           getEnv("CDEK_CLIENT_ID", ""),
        ClientSecret:       getEnv("CDEK_CLIENT_SECRET", ""),
        BaseURL:            getEnv("CDEK_BASE_URL", "https://api.cdek.ru"),
        WarehouseCityCode:  getEnvInt32("CDEK_WAREHOUSE_CITY", 44),
        BreakerMaxRequests: getEnvUint32("CDEK_BREAKER_MAX_REQUESTS", 5),
        BreakerTimeout:     getEnvDuration("CDEK_BREAKER_TIMEOUT", 60*time.Second),
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

---

## Development

### Локальная разработка

```bash
# 1. Клонировать проект
git clone https://github.com/yourcompany/your-erp.git
cd your-erp

# 2. Установить зависимости
go mod download

# 3. Настроить .env
cp .env.example .env
# Отредактировать .env с вашими credentials

# 4. Запустить приложение
go run cmd/api/main.go
```

### Тестирование

```bash
# Unit тесты (без реальных API вызовов)
go test ./...

# Integration тесты (требуют credentials)
export CDEK_CLIENT_ID=your-test-credentials
export CDEK_CLIENT_SECRET=your-test-secret
go test -tags=integration ./...

# Race detector
go test -race ./...

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Docker для development

```dockerfile
# Dockerfile.dev

FROM golang:1.23-alpine

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN go build -o /app/bin/api cmd/api/main.go

# Run
CMD ["/app/bin/api"]
```

```bash
# docker-compose.yml для development
version: '3.8'

services:
  api:
    build:
      context: .
      dockerfile: Dockerfile.dev
    ports:
      - "8080:8080"
    environment:
      - CDEK_CLIENT_ID=${CDEK_CLIENT_ID}
      - CDEK_CLIENT_SECRET=${CDEK_CLIENT_SECRET}
      - DATABASE_URL=postgres://user:pass@db:5432/erp
    volumes:
      - .:/app
    depends_on:
      - db

  db:
    image: postgres:16-alpine
    environment:
      - POSTGRES_DB=erp
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
    ports:
      - "5432:5432"
```

---

## Staging

### Настройка Staging окружения

```bash
# .env.staging

CDEK_CLIENT_ID=staging-client-id
CDEK_CLIENT_SECRET=staging-secret
CDEK_BASE_URL=https://api.cdek.ru  # тот же URL

# Database
DATABASE_URL=postgres://user:pass@staging-db:5432/erp_staging

# Логирование (verbose для staging)
LOG_LEVEL=debug
```

### CI/CD Pipeline (GitHub Actions)

```yaml
# .github/workflows/deploy-staging.yml

name: Deploy to Staging

on:
  push:
    branches: [develop]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Run tests
        run: |
          go test -v -race ./...

      - name: Integration tests
        env:
          CDEK_CLIENT_ID: ${{ secrets.CDEK_TEST_CLIENT_ID }}
          CDEK_CLIENT_SECRET: ${{ secrets.CDEK_TEST_CLIENT_SECRET }}
        run: |
          go test -tags=integration -v ./...

  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Deploy to staging
        env:
          DEPLOY_KEY: ${{ secrets.STAGING_DEPLOY_KEY }}
        run: |
          # Ваш deploy скрипт
          ./scripts/deploy-staging.sh
```

---

## Production

### Production конфигурация

```go
// config/production.go

package config

import (
    "time"

    "github.com/metrica-pro/cdek-go/pkg/cdek"
    "github.com/rs/zerolog"
)

func NewProductionCDEKService(logger *zerolog.Logger) (*cdek.Service, error) {
    config := cdek.DefaultConfig(
        mustGetEnv("CDEK_CLIENT_ID"),
        mustGetEnv("CDEK_CLIENT_SECRET"),
    )

    manager, err := cdek.NewManager(config)
    if err != nil {
        return nil, err
    }

    client, _ := manager.GetDefaultClient()

    // Production-ready конфигурация
    serviceConfig := &cdek.ServiceConfig{
        BreakerName:        "cdek-api-production",
        BreakerMaxRequests: 10,          // Больше для production
        BreakerInterval:    30 * time.Second,
        BreakerTimeout:     60 * time.Second,
        Logger:             logger,
    }

    return cdek.NewService(client, serviceConfig), nil
}

func mustGetEnv(key string) string {
    value := os.Getenv(key)
    if value == "" {
        panic("Required env var not set: " + key)
    }
    return value
}
```

### Secrets Management

#### Kubernetes Secrets

```yaml
# k8s/secrets.yaml

apiVersion: v1
kind: Secret
metadata:
  name: cdek-credentials
  namespace: production
type: Opaque
stringData:
  client-id: "your-production-client-id"
  client-secret: "your-production-secret"
```

```yaml
# k8s/deployment.yaml

apiVersion: apps/v1
kind: Deployment
metadata:
  name: erp-api
spec:
  template:
    spec:
      containers:
      - name: api
        image: yourcompany/erp-api:latest
        env:
        - name: CDEK_CLIENT_ID
          valueFrom:
            secretKeyRef:
              name: cdek-credentials
              key: client-id
        - name: CDEK_CLIENT_SECRET
          valueFrom:
            secretKeyRef:
              name: cdek-credentials
              key: client-secret
```

#### AWS Secrets Manager

```go
import (
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/secretsmanager"
)

func getCDEKCredentialsFromAWS() (string, string, error) {
    sess := session.Must(session.NewSession())
    svc := secretsmanager.New(sess)

    result, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
        SecretId: aws.String("production/cdek/credentials"),
    })
    if err != nil {
        return "", "", err
    }

    var creds struct {
        ClientID     string `json:"client_id"`
        ClientSecret string `json:"client_secret"`
    }
    json.Unmarshal([]byte(*result.SecretString), &creds)

    return creds.ClientID, creds.ClientSecret, nil
}
```

### Health Checks

```go
// internal/health/cdek.go

package health

import (
    "context"
    "time"

    "github.com/metrica-pro/cdek-go/pkg/cdek"
)

type CDEKHealthCheck struct {
    service *cdek.Service
}

func (h *CDEKHealthCheck) Check(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    return h.service.HealthCheck(ctx)
}
```

HTTP endpoint:

```go
// GET /health/cdek
func (h *HealthHandler) CDEKHealth(c echo.Context) error {
    if err := cdekHealthCheck.Check(c.Request().Context()); err != nil {
        return c.JSON(503, map[string]string{
            "status": "unhealthy",
            "error":  err.Error(),
        })
    }

    return c.JSON(200, map[string]string{
        "status": "healthy",
    })
}
```

### Production Deployment

```bash
# scripts/deploy-production.sh

#!/bin/bash
set -e

echo "🚀 Deploying to production..."

# 1. Build
echo "📦 Building..."
go build -ldflags="-s -w" -o bin/api cmd/api/main.go

# 2. Run tests
echo "🧪 Running tests..."
go test -v ./...

# 3. Build Docker image
echo "🐳 Building Docker image..."
docker build -t yourcompany/erp-api:$(git rev-parse --short HEAD) .
docker tag yourcompany/erp-api:$(git rev-parse --short HEAD) yourcompany/erp-api:latest

# 4. Push to registry
echo "📤 Pushing to registry..."
docker push yourcompany/erp-api:$(git rev-parse --short HEAD)
docker push yourcompany/erp-api:latest

# 5. Deploy to Kubernetes
echo "☸️  Deploying to Kubernetes..."
kubectl apply -f k8s/secrets.yaml
kubectl apply -f k8s/deployment.yaml
kubectl rollout status deployment/erp-api -n production

echo "✅ Deployment complete!"
```

---

## Мониторинг

### Prometheus Metrics

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    cdekRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cdek_requests_total",
            Help: "Total number of CDEK API requests",
        },
        []string{"method", "status"},
    )

    cdekRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "cdek_request_duration_seconds",
            Help:    "CDEK API request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method"},
    )

    cdekCircuitBreakerState = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "cdek_circuit_breaker_state",
            Help: "CDEK circuit breaker state (0=closed, 1=half-open, 2=open)",
        },
        []string{"name"},
    )
)
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "CDEK Integration",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [{
          "expr": "rate(cdek_requests_total[5m])"
        }]
      },
      {
        "title": "Error Rate",
        "targets": [{
          "expr": "rate(cdek_requests_total{status=\"error\"}[5m]) / rate(cdek_requests_total[5m])"
        }]
      },
      {
        "title": "Request Duration (p95)",
        "targets": [{
          "expr": "histogram_quantile(0.95, cdek_request_duration_seconds_bucket)"
        }]
      },
      {
        "title": "Circuit Breaker State",
        "targets": [{
          "expr": "cdek_circuit_breaker_state"
        }]
      }
    ]
  }
}
```

### Alerting (Prometheus)

```yaml
# alerts/cdek.yml

groups:
  - name: cdek
    interval: 30s
    rules:
      - alert: CDEKHighErrorRate
        expr: |
          rate(cdek_requests_total{status="error"}[5m]) / rate(cdek_requests_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High CDEK API error rate"
          description: "{{ $value | humanizePercentage }} errors in last 5 minutes"

      - alert: CDEKCircuitBreakerOpen
        expr: cdek_circuit_breaker_state == 2
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "CDEK Circuit Breaker is OPEN"
          description: "CDEK API unavailable, orders cannot be created"

      - alert: CDEKSlowRequests
        expr: histogram_quantile(0.95, cdek_request_duration_seconds_bucket) > 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "CDEK API slow responses"
          description: "p95 latency: {{ $value }}s"
```

---

## Безопасность

### 1. Хранение Credentials

❌ **Плохо:**
```go
config := cdek.DefaultConfig(
    "Gh2kvD9jFHpP9DiRxE343ViJWlMVeMsx",  // Хардкод в коде!
    "gq5itaXV6uD728jcaeFGRJadaC8xRXGs",
)
```

✅ **Хорошо:**
```go
config := cdek.DefaultConfig(
    os.Getenv("CDEK_CLIENT_ID"),     // Из переменных окружения
    os.Getenv("CDEK_CLIENT_SECRET"),
)
```

### 2. .gitignore

```gitignore
# .gitignore

# Credentials
.env
.env.*
!.env.example

# Secrets
secrets/
*.secret
```

### 3. Rotate Credentials

Регулярно меняйте credentials (рекомендуется каждые 90 дней):

1. Создайте новое приложение в личном кабинете CDEK
2. Получите новые Client ID и Secret
3. Обновите secrets в production
4. Дождитесь deployment
5. Удалите старое приложение через 24 часа (для graceful transition)

### 4. Rate Limiting на вашей стороне

```go
import "golang.org/x/time/rate"

type RateLimitedCDEKService struct {
    service *cdek.Service
    limiter *rate.Limiter
}

func NewRateLimitedService(service *cdek.Service) *RateLimitedCDEKService {
    // 100 запросов в минуту
    limiter := rate.NewLimiter(rate.Every(time.Minute/100), 10) // burst 10

    return &RateLimitedCDEKService{
        service: service,
        limiter: limiter,
    }
}

func (s *RateLimitedCDEKService) CreateOrder(ctx context.Context, req *OrderRequest) (*Order, error) {
    if err := s.limiter.Wait(ctx); err != nil {
        return nil, err
    }

    return s.service.CreateOrder(ctx, req)
}
```

---

## Troubleshooting

### Проблема: Circuit Breaker постоянно открыт

**Причина:** CDEK API недоступен или слишком много ошибок

**Решение:**
1. Проверить status CDEK API: https://status.cdek.ru/
2. Проверить логи на ошибки 401 (неверные credentials)
3. Увеличить BreakerTimeout если API восстанавливается медленно
4. Проверить network connectivity к api.cdek.ru

### Проблема: 401 Unauthorized

**Причина:** Неверные credentials или истекший токен

**Решение:**
```bash
# Проверить credentials
curl -X POST https://api.cdek.ru/v2/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials&client_id=YOUR_ID&client_secret=YOUR_SECRET"

# Должен вернуть access_token
```

### Проблема: Медленные запросы

**Причина:** Большие payload или сетевые проблемы

**Решение:**
1. Проверить network latency к api.cdek.ru
2. Уменьшить размер batch операций (PrintBarcode - макс 100 заказов вместо 1000)
3. Увеличить timeout в конфигурации

### Проблема: Memory leak

**Причина:** Не закрываются HTTP соединения

**Решение:**
```go
// Библиотека автоматически управляет connections, но убедитесь:
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

order, err := service.CreateOrder(ctx, req)
```

### Логирование для debugging

```go
import "github.com/rs/zerolog/log"

logger := log.With().
    Str("service", "cdek").
    Logger()

serviceConfig := &cdek.ServiceConfig{
    Logger: &logger,
}

service := cdek.NewService(client, serviceConfig)

// Теперь все запросы логируются с деталями
```

---

## Поддержка

### Официальная документация

- 📘 [CDEK API Docs](https://api.cdek.ru/v2/)
- 🔧 [CDEK Status Page](https://status.cdek.ru/)
- 📞 [CDEK Support](https://cdek.ru/support/)

### cdek-go библиотека

- 🐛 [GitHub Issues](https://github.com/metrica-pro/cdek-go/issues)
- 📖 [API Reference](API_ENDPOINTS.md)
- 📚 [Integration Guide](INTEGRATION_GUIDE.md)
- 📧 Email: support@metrica.pro

---

Made with ❤️ by [Metrica Pro](https://metrica.pro)
