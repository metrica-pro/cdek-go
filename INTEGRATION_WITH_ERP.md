# Интеграция CDEK клиента в ERPGO

Руководство по интеграции CDEK Go Client в ERP систему на базе ERPGO.

## Обзор

CDEK клиент интегрируется в ERPGO через стандартную систему интеграций:
1. **Credential Type** - хранение учетных данных
2. **Integration Type** - описание операций
3. **HTTP Client** - реализация бизнес-логики
4. **Service Layer** - использование в сервисах
5. **Handler Layer** - API endpoints

## Установка

```bash
cd /Users/app/erpgo
go get github.com/metrica-pro/cdek-go/pkg/cdek@latest
```

## Шаг 1: Credential Type

Создайте credential type для хранения учетных данных CDEK.

**Файл:** `/Users/app/erpgo/internal/integrations/credential_types/services/cdek.go`

```go
package services

import (
    "github.com/google/uuid"
    "yourcompany/erpgo/internal/integrations/credential_types/domain"
)

func NewCDEKCredentialType() *domain.CredentialType {
    return &domain.CredentialType{
        ID:   uuid.New(),
        Slug: "cdekApi",
        Name: "CDEK API",
        Properties: []domain.Property{
            {
                Name:        "client_id",
                DisplayName: "Client ID",
                Type:        "string",
                Required:    true,
                Description: "Идентификатор клиента CDEK",
            },
            {
                Name:        "client_secret",
                DisplayName: "Client Secret",
                Type:        "password",
                Required:    true,
                Sensitive:   true,
                Description: "Секретный ключ клиента CDEK",
            },
            {
                Name:        "test_mode",
                DisplayName: "Тестовый режим",
                Type:        "boolean",
                Default:     false,
                Description: "Использовать sandbox API",
            },
            {
                Name:        "account_name",
                DisplayName: "Название аккаунта",
                Type:        "string",
                Default:     "default",
                Description: "Уникальное имя аккаунта (для мультиаккаунта)",
            },
        },
    }
}
```

## Шаг 2: Integration Type

Определите доступные операции интеграции.

**Файл:** `/Users/app/erpgo/internal/integrations/integration_types/services/cdek.go`

```go
package services

import (
    "github.com/google/uuid"
    "yourcompany/erpgo/internal/integrations/integration_types/domain"
)

func NewCDEKIntegrationType() *domain.IntegrationType {
    return &domain.IntegrationType{
        ID:   uuid.New(),
        Slug: "cdek",
        Name: "CDEK Доставка",
        Description: "Интеграция с CDEK для расчета стоимости, создания заказов и отслеживания доставки",
        Operations: []domain.Operation{
            {
                Slug:        "calculateCost",
                Name:        "Расчет стоимости",
                Description: "Расчет стоимости доставки",
            },
            {
                Slug:        "createOrder",
                Name:        "Создание заказа",
                Description: "Создание заказа доставки в CDEK",
            },
            {
                Slug:        "trackOrder",
                Name:        "Отслеживание заказа",
                Description: "Получение статуса доставки",
            },
            {
                Slug:        "printDocuments",
                Name:        "Печать документов",
                Description: "Печать накладных и этикеток",
            },
            {
                Slug:        "listDeliveryPoints",
                Name:        "Список ПВЗ",
                Description: "Получение списка пунктов выдачи заказов",
            },
        },
        CredentialTypeSlug: "cdekApi",
    }
}
```

## Шаг 3: HTTP Client

Реализуйте HTTP клиент с интеграцией CDEK SDK.

**Файл:** `/Users/app/erpgo/internal/integrations/clients/cdek/client.go`

```go
package cdek

import (
    "context"
    "fmt"
    "time"

    "github.com/sony/gobreaker"
    cdekapi "github.com/metrica-pro/cdek-go/pkg/cdek"
)

// Client - CDEK integration client
type Client struct {
    service *cdekapi.Service
    breaker *gobreaker.CircuitBreaker
    config  *Config
}

// Config - CDEK client configuration
type Config struct {
    ClientID     string
    ClientSecret string
    TestMode     bool
    AccountName  string
}

// NewClient создает новый CDEK клиент
func NewClient(config *Config) (*Client, error) {
    baseURL := cdekapi.URLProduction
    if config.TestMode {
        baseURL = cdekapi.URLSandbox
    }

    accountName := config.AccountName
    if accountName == "" {
        accountName = "default"
    }

    cdekConfig := &cdekapi.Config{
        Accounts: []cdekapi.AccountConfig{
            {
                Name:         accountName,
                ClientID:     config.ClientID,
                ClientSecret: config.ClientSecret,
                BaseURL:      baseURL,
                Timeout:      30 * time.Second,
                MaxRetries:   3,
            },
        },
        DefaultAccount: accountName,
    }

    manager, err := cdekapi.NewManager(cdekConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create CDEK manager: %w", err)
    }

    authClient, err := manager.GetDefaultClient()
    if err != nil {
        return nil, fmt.Errorf("failed to get CDEK client: %w", err)
    }

    service := cdekapi.NewService(authClient)

    // Circuit breaker для защиты от перегрузки
    breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
        Name:        "cdek-api",
        MaxRequests: 10,
        Interval:    30 * time.Second,
        Timeout:     60 * time.Second,
    })

    return &Client{
        service: service,
        breaker: breaker,
        config:  config,
    }, nil
}

// HealthCheck проверяет доступность API
func (c *Client) HealthCheck(ctx context.Context) error {
    result, err := c.breaker.Execute(func() (interface{}, error) {
        return nil, c.service.HealthCheck(ctx)
    })

    if err != nil {
        return err
    }

    if result != nil {
        return result.(error)
    }

    return nil
}

// CalculateCost рассчитывает стоимость доставки
func (c *Client) CalculateCost(ctx context.Context, req *CalculateCostRequest) (*CostResponse, error) {
    // TODO: Реализовать после завершения service layer в cdek-go
    // Пример:
    // result, err := c.breaker.Execute(func() (interface{}, error) {
    //     return c.service.CalculateCost(ctx, convertToCDEKRequest(req))
    // })
    return nil, fmt.Errorf("not implemented")
}

// CreateOrder создает заказ доставки
func (c *Client) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    // TODO: Реализовать после завершения service layer
    return nil, fmt.Errorf("not implemented")
}

// GetOrder получает информацию о заказе
func (c *Client) GetOrder(ctx context.Context, uuid string) (*Order, error) {
    // TODO: Реализовать после завершения service layer
    return nil, fmt.Errorf("not implemented")
}
```

**Файл:** `/Users/app/erpgo/internal/integrations/clients/cdek/types.go`

```go
package cdek

// CalculateCostRequest - запрос расчета стоимости
type CalculateCostRequest struct {
    FromCityCode int
    ToCityCode   int
    Weight       int
    Length       int
    Width        int
    Height       int
    TariffCode   int
}

// CostResponse - ответ с расчетом стоимости
type CostResponse struct {
    DeliverySum float64
    PeriodMin   int
    PeriodMax   int
    TariffCode  int
}

// CreateOrderRequest - запрос создания заказа
type CreateOrderRequest struct {
    TariffCode     int
    FromAddress    string
    ToAddress      string
    RecipientName  string
    RecipientPhone string
    Packages       []Package
}

// Package - посылка
type Package struct {
    Weight int
    Length int
    Width  int
    Height int
    Items  []Item
}

// Item - товар в посылке
type Item struct {
    Name   string
    Cost   float64
    Weight int
    Amount int
}

// Order - заказ доставки
type Order struct {
    UUID       string
    CDEKNumber string
    Status     string
    CreatedAt  string
}
```

## Шаг 4: Service Layer

Используйте CDEK клиент в сервисном слое ERP.

**Файл:** `/Users/app/erpgo/internal/service/delivery_service.go`

```go
package service

import (
    "context"
    "fmt"

    cdekClient "yourcompany/erpgo/internal/integrations/clients/cdek"
    "yourcompany/erpgo/internal/repository"
)

type DeliveryService struct {
    credentialRepo repository.CredentialRepository
    cdekClients    map[string]*cdekClient.Client
    mu             sync.RWMutex
}

func NewDeliveryService(credentialRepo repository.CredentialRepository) *DeliveryService {
    return &DeliveryService{
        credentialRepo: credentialRepo,
        cdekClients:    make(map[string]*cdekClient.Client),
    }
}

// getCDEKClient получает или создает CDEK клиент
func (s *DeliveryService) getCDEKClient(ctx context.Context) (*cdekClient.Client, error) {
    s.mu.RLock()
    if client, ok := s.cdekClients["default"]; ok {
        s.mu.RUnlock()
        return client, nil
    }
    s.mu.RUnlock()

    // Получаем credentials из БД
    cred, err := s.credentialRepo.GetByType(ctx, "cdekApi")
    if err != nil {
        return nil, fmt.Errorf("failed to get CDEK credentials: %w", err)
    }

    // Создаем клиент
    config := &cdekClient.Config{
        ClientID:     cred.Properties["client_id"],
        ClientSecret: cred.Properties["client_secret"],
        TestMode:     cred.Properties["test_mode"] == "true",
        AccountName:  cred.Properties["account_name"],
    }

    client, err := cdekClient.NewClient(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create CDEK client: %w", err)
    }

    // Кешируем клиент
    s.mu.Lock()
    s.cdekClients["default"] = client
    s.mu.Unlock()

    return client, nil
}

// CalculateDeliveryCost рассчитывает стоимость доставки
func (s *DeliveryService) CalculateDeliveryCost(ctx context.Context, req *CalculateDeliveryRequest) (*DeliveryCostResponse, error) {
    client, err := s.getCDEKClient(ctx)
    if err != nil {
        return nil, err
    }

    cost, err := client.CalculateCost(ctx, &cdekClient.CalculateCostRequest{
        FromCityCode: req.FromCity,
        ToCityCode:   req.ToCity,
        Weight:       req.Weight,
        Length:       req.Length,
        Width:        req.Width,
        Height:       req.Height,
        TariffCode:   req.TariffCode,
    })
    if err != nil {
        return nil, err
    }

    return &DeliveryCostResponse{
        DeliverySum: cost.DeliverySum,
        PeriodMin:   cost.PeriodMin,
        PeriodMax:   cost.PeriodMax,
    }, nil
}
```

## Шаг 5: Handler Layer

Создайте HTTP handler для API endpoints.

**Файл:** `/Users/app/erpgo/internal/http/handler/delivery_handler.go`

```go
package handler

import (
    "net/http"

    "github.com/labstack/echo/v4"
    "yourcompany/erpgo/internal/service"
)

type DeliveryHandler struct {
    deliveryService *service.DeliveryService
}

func NewDeliveryHandler(deliveryService *service.DeliveryService) *DeliveryHandler {
    return &DeliveryHandler{
        deliveryService: deliveryService,
    }
}

// CalculateCost рассчитывает стоимость доставки
func (h *DeliveryHandler) CalculateCost(c echo.Context) error {
    var req service.CalculateDeliveryRequest
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{
            "error": "invalid request",
        })
    }

    resp, err := h.deliveryService.CalculateDeliveryCost(c.Request().Context(), &req)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{
            "error": err.Error(),
        })
    }

    return c.JSON(http.StatusOK, resp)
}
```

## Шаг 6: Регистрация в main.go

Зарегистрируйте credential type и integration type.

**Файл:** `/Users/app/erpgo/cmd/api/main.go`

```go
import (
    cdekCredType "yourcompany/erpgo/internal/integrations/credential_types/services"
    cdekIntType "yourcompany/erpgo/internal/integrations/integration_types/services"
)

func main() {
    // ...

    // Регистрация CDEK credential type
    credentialTypeService.Register(cdekCredType.NewCDEKCredentialType())

    // Регистрация CDEK integration type
    integrationTypeService.Register(cdekIntType.NewCDEKIntegrationType())

    // ...
}
```

## Миграции (опционально)

Если требуется хранить данные доставки в БД:

**Файл:** `/Users/app/erpgo/migrations/20260215000000_add_delivery_tables.up.sql`

```sql
CREATE TABLE IF NOT EXISTS erp.deliveries (
    id UUID PRIMARY KEY DEFAULT pg_catalog.uuidv7(),
    document_id UUID REFERENCES erp.documents(id),
    cdek_order_uuid UUID,
    cdek_number TEXT,
    status TEXT NOT NULL,
    tracking_number TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_deleted BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_deliveries_document_id ON erp.deliveries(document_id) WHERE NOT is_deleted;
CREATE INDEX idx_deliveries_cdek_order_uuid ON erp.deliveries(cdek_order_uuid) WHERE NOT is_deleted;
CREATE INDEX idx_deliveries_tracking_number ON erp.deliveries(tracking_number) WHERE NOT is_deleted;
```

## Использование в бизнес-процессах

### Создание заказа при оформлении документа продажи

```go
// В обработчике создания документа продажи
func (s *SaleService) Create(ctx context.Context, req *CreateSaleRequest) (*Sale, error) {
    // ... создание документа продажи

    // Если требуется доставка CDEK
    if req.DeliveryType == "cdek" {
        cost, err := s.deliveryService.CalculateDeliveryCost(ctx, &CalculateDeliveryRequest{
            FromCity:   req.FromCity,
            ToCity:     req.ToCity,
            Weight:     req.TotalWeight,
            TariffCode: req.TariffCode,
        })
        if err != nil {
            return nil, err
        }

        sale.DeliveryCost = cost.DeliverySum
    }

    return sale, nil
}
```

## Тестирование

```go
// internal/integrations/clients/cdek/client_test.go

func TestClient_HealthCheck(t *testing.T) {
    config := &Config{
        ClientID:     os.Getenv("CDEK_CLIENT_ID"),
        ClientSecret: os.Getenv("CDEK_CLIENT_SECRET"),
        TestMode:     true,
    }

    client, err := NewClient(config)
    require.NoError(t, err)

    ctx := context.Background()
    err = client.HealthCheck(ctx)
    require.NoError(t, err)
}
```

## Примеры использования

### Расчет стоимости через API

```bash
curl -X POST http://localhost:8080/api/v1/delivery/calculate \
  -H "Content-Type: application/json" \
  -d '{
    "from_city": 44,
    "to_city": 137,
    "weight": 1000,
    "tariff_code": 136
  }'
```

### Ответ

```json
{
  "delivery_sum": 450.50,
  "period_min": 2,
  "period_max": 4
}
```

---

**Статус:** Готово к интеграции после завершения service layer в cdek-go
