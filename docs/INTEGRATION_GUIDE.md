# Интеграция CDEK Go Client в ERP/CRM системы

Полный гайд по интеграции cdek-go библиотеки в корпоративные системы учета.

## Содержание

- [Архитектура интеграции](#архитектура-интеграции)
- [Установка и настройка](#установка-и-настройка)
- [Интеграция в ERP (на примере ERPgo)](#интеграция-в-erp)
- [Интеграция в CRM](#интеграция-в-crm)
- [Типовые сценарии](#типовые-сценарии)
- [Production best practices](#production-best-practices)
- [Мониторинг и отладка](#мониторинг-и-отладка)

---

## Архитектура интеграции

### Общая схема

```
ERP/CRM Система
  ↓
[Integration Layer]
  ├─ Credential Storage (БД)
  ├─ Webhook Handler (HTTP)
  └─ CDEK Service Wrapper
      ↓
    cdek-go Library
      ├─ Service API (high-level)
      ├─ Circuit Breaker
      ├─ OAuth2 Client
      └─ HTTP Client
          ↓
        CDEK API v2
```

### Компоненты интеграции

1. **Credential Storage** - Хранение API credentials в БД
2. **Integration Service** - Бизнес-логика работы с CDEK
3. **Webhook Handler** - Обработка уведомлений от CDEK
4. **Background Jobs** - Синхронизация статусов, печать документов
5. **Admin UI** - Настройка интеграции, мониторинг

---

## Установка и настройка

### 1. Установка библиотеки

```bash
cd /path/to/your-erp
go get github.com/metrica-pro/cdek-go
```

### 2. Структура проекта

```
your-erp/
├── internal/
│   ├── integrations/
│   │   ├── credential_types/
│   │   │   └── cdek.go              # Тип credentials
│   │   ├── integration_types/
│   │   │   └── cdek.go              # Тип интеграции
│   │   └── clients/
│   │       └── cdek/
│   │           ├── client.go        # Обертка над cdek-go
│   │           ├── webhook.go       # Webhook handler
│   │           └── types.go         # DTO для ERP
│   ├── service/
│   │   └── delivery_service.go      # Бизнес-логика доставок
│   ├── repository/
│   │   ├── delivery.go              # Repository для доставок
│   │   └── credential.go            # Repository для credentials
│   └── http/
│       └── handler/
│           └── delivery_handler.go  # HTTP handlers
└── migrations/
    └── 20260215_add_deliveries.sql  # Миграции БД
```

---

## Интеграция в ERP

### Шаг 1: Создание Credential Type

```go
// internal/integrations/credential_types/cdek.go

package credential_types

import "your-erp/internal/domain"

func NewCDEKCredentialType() *domain.CredentialType {
    return &domain.CredentialType{
        Slug: "cdekApi",
        Name: "CDEK API",
        Properties: []domain.Property{
            {
                Name:        "client_id",
                Type:        "string",
                Required:    true,
                Description: "Client ID из личного кабинета CDEK",
            },
            {
                Name:        "client_secret",
                Type:        "password",
                Required:    true,
                Description: "Client Secret из личного кабинета CDEK",
            },
            {
                Name:        "warehouse_location",
                Type:        "string",
                Required:    false,
                Description: "Код города склада по умолчанию (например, 44 для Москвы)",
            },
        },
    }
}
```

### Шаг 2: Создание Integration Type

```go
// internal/integrations/integration_types/cdek.go

package integration_types

import "your-erp/internal/domain"

func NewCDEKIntegrationType() *domain.IntegrationType {
    return &domain.IntegrationType{
        Slug: "cdek",
        Name: "CDEK Доставка",
        Description: "Интеграция с службой доставки CDEK для расчета стоимости, создания заказов и отслеживания",
        Category: "delivery",
        RequiredCredentials: []string{"cdekApi"},
        Operations: []domain.Operation{
            {Name: "calculateCost", Description: "Расчет стоимости доставки"},
            {Name: "createOrder", Description: "Создание заказа на доставку"},
            {Name: "trackOrder", Description: "Отслеживание статуса заказа"},
            {Name: "printDocuments", Description: "Печать накладных и штрих-кодов"},
            {Name: "listDeliveryPoints", Description: "Получение списка ПВЗ"},
        },
    }
}
```

### Шаг 3: CDEK Client Wrapper

```go
// internal/integrations/clients/cdek/client.go

package cdek

import (
    "context"
    "fmt"
    "time"

    "github.com/metrica-pro/cdek-go/pkg/cdek"
    "github.com/rs/zerolog"
    "github.com/sony/gobreaker/v2"
)

type Client struct {
    service *cdek.Service
    breaker *gobreaker.CircuitBreaker
    logger  zerolog.Logger

    // Настройки из ERP
    warehouseCityCode int32
}

type Config struct {
    ClientID          string
    ClientSecret      string
    WarehouseCityCode int32
    Logger            *zerolog.Logger
}

func NewClient(cfg *Config) (*Client, error) {
    // 1. Создаем конфигурацию cdek-go
    cdekConfig := cdek.DefaultConfig(cfg.ClientID, cfg.ClientSecret)

    // 2. Создаем менеджер
    manager, err := cdek.NewManager(cdekConfig)
    if err != nil {
        return nil, fmt.Errorf("create manager: %w", err)
    }

    // 3. Получаем клиент
    client, _ := manager.GetDefaultClient()

    // 4. Создаем service с логированием
    var logger zerolog.Logger
    if cfg.Logger != nil {
        logger = *cfg.Logger
    } else {
        logger = zerolog.Nop()
    }

    serviceConfig := &cdek.ServiceConfig{
        BreakerName:        "cdek-api",
        BreakerMaxRequests: 5,
        BreakerInterval:    30 * time.Second,
        BreakerTimeout:     60 * time.Second,
        Logger:             &logger,
    }

    service := cdek.NewService(client, serviceConfig)

    return &Client{
        service:           service,
        logger:            logger,
        warehouseCityCode: cfg.WarehouseCityCode,
    }, nil
}

// CalculateDeliveryCost рассчитывает стоимость доставки для ERP заказа
func (c *Client) CalculateDeliveryCost(ctx context.Context, req *CalculateDeliveryCostRequest) (*DeliveryCostResponse, error) {
    c.logger.Info().
        Int32("from", c.warehouseCityCode).
        Int32("to", req.ToCityCode).
        Int("packages", len(req.Packages)).
        Msg("calculating delivery cost")

    // Преобразуем ERP request в CDEK request
    cdekReq := &cdek.CostRequest{
        FromCityCode: c.warehouseCityCode,
        ToCityCode:   req.ToCityCode,
        Packages:     make([]cdek.Package, len(req.Packages)),
    }

    for i, pkg := range req.Packages {
        cdekReq.Packages[i] = cdek.Package{
            Weight: pkg.Weight,
            Length: pkg.Length,
            Width:  pkg.Width,
            Height: pkg.Height,
        }
    }

    // Вызываем CDEK API
    cost, err := c.service.CalculateCost(ctx, cdekReq)
    if err != nil {
        c.logger.Error().Err(err).Msg("failed to calculate cost")
        return nil, fmt.Errorf("cdek calculate cost: %w", err)
    }

    // Преобразуем CDEK response в ERP response
    tariffs := make([]TariffOption, len(cost.Tariffs))
    for i, t := range cost.Tariffs {
        tariffs[i] = TariffOption{
            Code:         t.TariffCode,
            Name:         t.TariffName,
            Cost:         t.DeliverySum,
            DeliveryDays: fmt.Sprintf("%d-%d", t.PeriodMin, t.PeriodMax),
        }
    }

    return &DeliveryCostResponse{
        Tariffs: tariffs,
    }, nil
}

// CreateDeliveryOrder создает заказ доставки в CDEK
func (c *Client) CreateDeliveryOrder(ctx context.Context, req *CreateDeliveryOrderRequest) (*DeliveryOrderResponse, error) {
    c.logger.Info().
        Str("erp_order_id", req.ERPOrderID).
        Int("tariff", req.TariffCode).
        Msg("creating delivery order")

    // Преобразуем данные получателя из ERP в CDEK формат
    recipient := cdek.Recipient{
        Contact: cdek.Contact{
            Name:   req.RecipientName,
            Phones: []cdek.Phone{{Number: req.RecipientPhone}},
        },
    }

    // Если получатель - юрлицо
    if req.RecipientCompany != nil {
        recipient.Company = req.RecipientCompany
        recipient.TIN = req.RecipientTIN
    }

    // Если получатель - физлицо
    if req.RecipientPassport != nil {
        recipient.PassportSeries = &req.RecipientPassport.Series
        recipient.PassportNumber = &req.RecipientPassport.Number
    }

    // Создаем заказ в CDEK
    orderReq := &cdek.OrderRequest{
        Type:       "1", // интернет-магазин
        TariffCode: req.TariffCode,
        Sender: cdek.Recipient{
            Contact: cdek.Contact{
                Company: ptrString("ООО Ваша Компания"),
                Name:    "Отдел доставки",
                Phones:  []cdek.Phone{{Number: "+74951234567"}},
            },
        },
        Recipient: recipient,
        FromLocation: cdek.Location{
            Code: &c.warehouseCityCode,
        },
        ToLocation: cdek.Location{
            Code: &req.ToCityCode,
        },
        Packages: convertPackages(req.Packages),
    }

    order, err := c.service.CreateOrder(ctx, orderReq)
    if err != nil {
        c.logger.Error().Err(err).Str("erp_order", req.ERPOrderID).Msg("failed to create order")
        return nil, fmt.Errorf("cdek create order: %w", err)
    }

    c.logger.Info().
        Str("cdek_uuid", order.UUID).
        Str("erp_order", req.ERPOrderID).
        Msg("delivery order created")

    return &DeliveryOrderResponse{
        CDEKUUID:   order.UUID,
        CDEKNumber: order.Number,
    }, nil
}

// TrackOrder получает статус заказа
func (c *Client) TrackOrder(ctx context.Context, cdekUUID string) (*TrackingResponse, error) {
    tracking, err := c.service.TrackOrder(ctx, cdekUUID)
    if err != nil {
        return nil, fmt.Errorf("cdek track order: %w", err)
    }

    return &TrackingResponse{
        Status:          tracking.CurrentStatus.Name,
        StatusCode:      tracking.CurrentStatus.Code,
        StatusDateTime:  tracking.CurrentStatus.DateTime,
        EstimatedDelivery: tracking.EstimatedDelivery,
        ActualDelivery:    tracking.ActualDelivery,
    }, nil
}

func ptrString(s string) *string { return &s }

func convertPackages(packages []PackageRequest) []cdek.OrderPackage {
    result := make([]cdek.OrderPackage, len(packages))
    for i, pkg := range packages {
        result[i] = cdek.OrderPackage{
            Number: fmt.Sprintf("%d", i+1),
            Weight: pkg.Weight,
            Items: []cdek.Item{
                {
                    Name:    pkg.ItemName,
                    WareKey: pkg.ItemSKU,
                    Cost:    pkg.ItemCost,
                    Weight:  pkg.Weight,
                    Amount:  pkg.ItemAmount,
                },
            },
        }
    }
    return result
}
```

### Шаг 4: DTO для ERP

```go
// internal/integrations/clients/cdek/types.go

package cdek

// Запросы от ERP к CDEK

type CalculateDeliveryCostRequest struct {
    ToCityCode int32
    Packages   []PackageRequest
}

type PackageRequest struct {
    Weight     int32
    Length     int32
    Width      int32
    Height     int32
    ItemName   string
    ItemSKU    string
    ItemCost   float64
    ItemAmount int32
}

type CreateDeliveryOrderRequest struct {
    ERPOrderID        string
    TariffCode        int
    ToCityCode        int32
    RecipientName     string
    RecipientPhone    string
    RecipientCompany  *string
    RecipientTIN      *string
    RecipientPassport *PassportData
    Packages          []PackageRequest
}

type PassportData struct {
    Series string
    Number string
}

// Ответы от CDEK в ERP

type DeliveryCostResponse struct {
    Tariffs []TariffOption
}

type TariffOption struct {
    Code         int
    Name         string
    Cost         float64
    DeliveryDays string
}

type DeliveryOrderResponse struct {
    CDEKUUID   string
    CDEKNumber *string
}

type TrackingResponse struct {
    Status            string
    StatusCode        string
    StatusDateTime    string
    EstimatedDelivery *string
    ActualDelivery    *string
}
```

### Шаг 5: Service Layer в ERP

```go
// internal/service/delivery_service.go

package service

import (
    "context"
    "fmt"

    "your-erp/internal/integrations/clients/cdek"
    "your-erp/internal/repository"
    "your-erp/internal/domain"
)

type DeliveryService struct {
    credentialRepo repository.CredentialRepository
    deliveryRepo   repository.DeliveryRepository
    cdekClients    map[string]*cdek.Client
}

func NewDeliveryService(
    credentialRepo repository.CredentialRepository,
    deliveryRepo repository.DeliveryRepository,
) *DeliveryService {
    return &DeliveryService{
        credentialRepo: credentialRepo,
        deliveryRepo:   deliveryRepo,
        cdekClients:    make(map[string]*cdek.Client),
    }
}

// CalculateDeliveryCost рассчитывает стоимость доставки для заказа
func (s *DeliveryService) CalculateDeliveryCost(ctx context.Context, req *CalculateDeliveryCostDTO) (*DeliveryCostDTO, error) {
    // 1. Получаем CDEK credentials из БД
    cred, err := s.credentialRepo.GetByType(ctx, "cdekApi")
    if err != nil {
        return nil, fmt.Errorf("get credentials: %w", err)
    }

    // 2. Получаем или создаем CDEK клиент
    client, err := s.getCDEKClient(cred)
    if err != nil {
        return nil, fmt.Errorf("get cdek client: %w", err)
    }

    // 3. Вызываем расчет стоимости
    cost, err := client.CalculateDeliveryCost(ctx, &cdek.CalculateDeliveryCostRequest{
        ToCityCode: req.ToCityCode,
        Packages:   convertToPackageRequest(req.Items),
    })
    if err != nil {
        return nil, fmt.Errorf("calculate cost: %w", err)
    }

    return &DeliveryCostDTO{
        Tariffs: cost.Tariffs,
    }, nil
}

// CreateDelivery создает доставку для заказа ERP
func (s *DeliveryService) CreateDelivery(ctx context.Context, req *CreateDeliveryDTO) (*domain.Delivery, error) {
    // 1. Получаем клиент
    cred, _ := s.credentialRepo.GetByType(ctx, "cdekApi")
    client, _ := s.getCDEKClient(cred)

    // 2. Создаем заказ в CDEK
    order, err := client.CreateDeliveryOrder(ctx, &cdek.CreateDeliveryOrderRequest{
        ERPOrderID:     req.OrderID,
        TariffCode:     req.TariffCode,
        ToCityCode:     req.ToCityCode,
        RecipientName:  req.RecipientName,
        RecipientPhone: req.RecipientPhone,
        Packages:       convertToPackageRequest(req.Items),
    })
    if err != nil {
        return nil, fmt.Errorf("create cdek order: %w", err)
    }

    // 3. Сохраняем доставку в БД
    delivery := &domain.Delivery{
        OrderID:         req.OrderID,
        Provider:        "cdek",
        CDEKOrderUUID:   order.CDEKUUID,
        CDEKOrderNumber: order.CDEKNumber,
        Status:          "created",
        TariffCode:      req.TariffCode,
    }

    if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
        return nil, fmt.Errorf("save delivery: %w", err)
    }

    return delivery, nil
}

// UpdateDeliveryStatus обновляет статус доставки (вызывается из webhook или background job)
func (s *DeliveryService) UpdateDeliveryStatus(ctx context.Context, cdekUUID string) error {
    // 1. Получаем доставку из БД
    delivery, err := s.deliveryRepo.FindByCDEKUUID(ctx, cdekUUID)
    if err != nil {
        return fmt.Errorf("find delivery: %w", err)
    }

    // 2. Получаем статус из CDEK
    cred, _ := s.credentialRepo.GetByType(ctx, "cdekApi")
    client, _ := s.getCDEKClient(cred)

    tracking, err := client.TrackOrder(ctx, cdekUUID)
    if err != nil {
        return fmt.Errorf("track order: %w", err)
    }

    // 3. Обновляем статус в БД
    delivery.Status = tracking.StatusCode
    delivery.StatusName = tracking.Status
    delivery.StatusUpdatedAt = tracking.StatusDateTime

    if err := s.deliveryRepo.Update(ctx, delivery); err != nil {
        return fmt.Errorf("update delivery: %w", err)
    }

    return nil
}

func (s *DeliveryService) getCDEKClient(cred *domain.Credential) (*cdek.Client, error) {
    // Кешируем клиент по credential ID
    if client, ok := s.cdekClients[cred.ID]; ok {
        return client, nil
    }

    // Создаем новый клиент
    client, err := cdek.NewClient(&cdek.Config{
        ClientID:          cred.Properties["client_id"],
        ClientSecret:      cred.Properties["client_secret"],
        WarehouseCityCode: parseInt32(cred.Properties["warehouse_location"]),
    })
    if err != nil {
        return nil, err
    }

    s.cdekClients[cred.ID] = client
    return client, nil
}
```

### Шаг 6: HTTP Handlers

```go
// internal/http/handler/delivery_handler.go

package handler

import (
    "net/http"

    "github.com/labstack/echo/v4"
    "your-erp/internal/service"
)

type DeliveryHandler struct {
    deliveryService *service.DeliveryService
}

func NewDeliveryHandler(deliveryService *service.DeliveryService) *DeliveryHandler {
    return &DeliveryHandler{
        deliveryService: deliveryService,
    }
}

// POST /api/delivery/calculate
func (h *DeliveryHandler) CalculateCost(c echo.Context) error {
    var req CalculateDeliveryCostRequest
    if err := c.Bind(&req); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }

    cost, err := h.deliveryService.CalculateDeliveryCost(c.Request().Context(), &req)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

    return c.JSON(http.StatusOK, cost)
}

// POST /api/delivery/create
func (h *DeliveryHandler) CreateDelivery(c echo.Context) error {
    var req CreateDeliveryRequest
    if err := c.Bind(&req); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }

    delivery, err := h.deliveryService.CreateDelivery(c.Request().Context(), &req)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

    return c.JSON(http.StatusCreated, delivery)
}

// POST /webhook/cdek (обработка webhook от CDEK)
func (h *DeliveryHandler) CDEKWebhook(c echo.Context) error {
    var webhook CDEKWebhookPayload
    if err := c.Bind(&webhook); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }

    // Обновляем статус доставки
    if err := h.deliveryService.UpdateDeliveryStatus(c.Request().Context(), webhook.OrderUUID); err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

    return c.NoContent(http.StatusOK)
}
```

### Шаг 7: Database Schema

```sql
-- migrations/20260215_add_deliveries.sql

CREATE TABLE IF NOT EXISTS erp.deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Связь с заказом ERP
    order_id UUID NOT NULL REFERENCES erp.orders(id),

    -- CDEK данные
    provider VARCHAR(50) NOT NULL DEFAULT 'cdek',
    cdek_order_uuid UUID,
    cdek_order_number TEXT,

    -- Статус
    status VARCHAR(100) NOT NULL,
    status_name VARCHAR(255),
    status_updated_at TIMESTAMPTZ,

    -- Тариф и стоимость
    tariff_code INTEGER,
    delivery_cost DECIMAL(10, 2),

    -- Tracking
    estimated_delivery_date DATE,
    actual_delivery_date DATE,

    -- Метаданные
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_deleted BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_deliveries_order_id ON erp.deliveries(order_id) WHERE NOT is_deleted;
CREATE INDEX idx_deliveries_cdek_uuid ON erp.deliveries(cdek_order_uuid) WHERE NOT is_deleted;
CREATE INDEX idx_deliveries_status ON erp.deliveries(status) WHERE NOT is_deleted;

-- Таблица истории статусов
CREATE TABLE IF NOT EXISTS erp.delivery_status_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id UUID NOT NULL REFERENCES erp.deliveries(id),

    status_code VARCHAR(100) NOT NULL,
    status_name VARCHAR(255) NOT NULL,
    city VARCHAR(255),
    occurred_at TIMESTAMPTZ NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_delivery_status_history_delivery_id ON erp.delivery_status_history(delivery_id);
```

### Шаг 8: Background Jobs (опционально)

```go
// internal/jobs/sync_delivery_statuses.go

package jobs

import (
    "context"
    "time"

    "your-erp/internal/service"
)

type DeliveryStatusSyncJob struct {
    deliveryService *service.DeliveryService
}

func (j *DeliveryStatusSyncJob) Run(ctx context.Context) error {
    // Получить все активные доставки
    deliveries, _ := j.deliveryService.GetActiveDeliveries(ctx)

    // Обновить статус каждой доставки
    for _, delivery := range deliveries {
        if err := j.deliveryService.UpdateDeliveryStatus(ctx, delivery.CDEKOrderUUID); err != nil {
            // Логируем ошибку, но продолжаем
            continue
        }

        // Задержка чтобы не превысить rate limit
        time.Sleep(100 * time.Millisecond)
    }

    return nil
}

// Запуск каждые 5 минут
func StartDeliveryStatusSync(deliveryService *service.DeliveryService) {
    job := &DeliveryStatusSyncJob{deliveryService: deliveryService}
    ticker := time.NewTicker(5 * time.Minute)

    go func() {
        for range ticker.C {
            job.Run(context.Background())
        }
    }()
}
```

---

## Интеграция в CRM

Для CRM систем логика аналогична, но фокусируется на:
- Отслеживании статусов доставки клиентам
- Автоматическом обновлении статусов сделок
- Уведомлениях менеджерам о проблемах с доставкой

```go
// Пример для CRM
func (s *CRMService) OnOrderCreated(ctx context.Context, dealID string) error {
    // 1. Создать доставку в CDEK
    // 2. Обновить deal с tracking link
    // 3. Создать задачу менеджеру "Отследить доставку"
}

func (s *CRMService) OnDeliveryStatusChanged(ctx context.Context, cdekUUID string, status string) error {
    // 1. Найти deal по CDEK UUID
    // 2. Обновить статус deal
    // 3. Отправить уведомление клиенту (email/sms)
    // 4. Если "Доставлен" - закрыть deal как "Выполнен"
}
```

---

## Типовые сценарии

### 1. Расчет стоимости при создании заказа

```go
func (h *OrderHandler) CreateOrder(c echo.Context) error {
    order := parseOrder(c)

    // Рассчитать стоимость доставки
    cost, _ := deliveryService.CalculateDeliveryCost(ctx, &CalculateCostDTO{
        ToCityCode: order.ShippingCity,
        Items:      order.Items,
    })

    // Показать клиенту варианты доставки
    order.DeliveryOptions = cost.Tariffs

    return c.JSON(200, order)
}
```

### 2. Создание доставки после оплаты

```go
func (h *PaymentHandler) OnPaymentSuccess(ctx context.Context, orderID string) error {
    order, _ := orderService.GetOrder(ctx, orderID)

    // Создать доставку в CDEK
    delivery, _ := deliveryService.CreateDelivery(ctx, &CreateDeliveryDTO{
        OrderID:     orderID,
        TariffCode:  order.SelectedTariff,
        ToCityCode:  order.ShippingCity,
        Recipient:   order.Customer,
        Items:       order.Items,
    })

    // Отправить email с tracking link
    emailService.SendDeliveryConfirmation(order.Customer.Email, delivery.TrackingURL)

    return nil
}
```

### 3. Автоматическая печать документов

```go
func (j *PrintDocumentsJob) Run(ctx context.Context) error {
    // Получить заказы готовые к отправке
    deliveries, _ := deliveryRepo.FindReadyForShipment(ctx)

    if len(deliveries) == 0 {
        return nil
    }

    // Собрать UUID заказов
    orderUUIDs := make([]string, len(deliveries))
    for i, d := range deliveries {
        orderUUIDs[i] = d.CDEKOrderUUID
    }

    // Создать задание на печать накладных
    printJob, _ := cdekService.PrintWaybill(ctx, &cdek.PrintRequest{
        Orders: orderUUIDs,
        Format: cdek.FormatA4,
    })

    // Подождать готовности
    time.Sleep(5 * time.Second)

    // Скачать PDF
    pdf, _ := cdekService.DownloadWaybill(ctx, printJob.UUID)

    // Сохранить в S3
    s3Service.Upload("deliveries/waybills/"+printJob.UUID+".pdf", pdf)

    return nil
}
```

---

## Production Best Practices

### 1. Обработка ошибок

```go
order, err := cdekService.CreateOrder(ctx, req)
if err != nil {
    // Circuit breaker открыт - API недоступен
    if errors.Is(err, gobreaker.ErrOpenState) {
        // Сохранить заказ в очередь для retry
        queue.Enqueue("cdek_orders", req)
        // Уведомить админа
        alertService.Alert("CDEK API недоступен")
        return ErrServiceUnavailable
    }

    // CDEK вернул ошибку валидации
    if cdekErr, ok := err.(*cdek.ErrorResponse); ok {
        // Логировать детали для отладки
        logger.Error().
            Str("code", cdekErr.Code).
            Str("message", cdekErr.Message).
            Msg("cdek validation error")

        // Показать пользователю понятную ошибку
        return fmt.Errorf("не удалось создать доставку: %s", cdekErr.Message)
    }

    return err
}
```

### 2. Кеширование справочников

```go
type CityCache struct {
    cache map[string]*cdek.City
    mu    sync.RWMutex
    ttl   time.Duration
}

func (c *CityCache) GetCity(ctx context.Context, code string) (*cdek.City, error) {
    // Проверить кеш
    c.mu.RLock()
    if city, ok := c.cache[code]; ok {
        c.mu.RUnlock()
        return city, nil
    }
    c.mu.RUnlock()

    // Загрузить из API
    cities, _ := cdekService.ListCities(ctx, &cdek.CitiesRequest{
        Code: &code,
    })

    if len(cities) > 0 {
        c.mu.Lock()
        c.cache[code] = &cities[0]
        c.mu.Unlock()
        return &cities[0], nil
    }

    return nil, ErrCityNotFound
}
```

### 3. Retry для временных ошибок

```go
func (s *DeliveryService) CreateOrderWithRetry(ctx context.Context, req *CreateOrderDTO) (*Delivery, error) {
    var lastErr error

    for attempt := 0; attempt < 3; attempt++ {
        delivery, err := s.CreateOrder(ctx, req)
        if err == nil {
            return delivery, nil
        }

        // Не retry для validation errors
        if _, ok := err.(*cdek.ErrorResponse); ok {
            return nil, err
        }

        lastErr = err

        // Exponential backoff
        backoff := time.Duration(1<<uint(attempt)) * time.Second
        time.Sleep(backoff)
    }

    return nil, fmt.Errorf("failed after 3 attempts: %w", lastErr)
}
```

---

## Мониторинг и отладка

### Метрики

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    cdekRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cdek_requests_total",
            Help: "Total CDEK API requests",
        },
        []string{"method", "status"},
    )

    cdekRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "cdek_request_duration_seconds",
            Help: "CDEK API request duration",
        },
        []string{"method"},
    )
)

func (c *Client) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    timer := prometheus.NewTimer(cdekRequestDuration.WithLabelValues("create_order"))
    defer timer.ObserveDuration()

    order, err := c.service.CreateOrder(ctx, req)

    status := "success"
    if err != nil {
        status = "error"
    }
    cdekRequestsTotal.WithLabelValues("create_order", status).Inc()

    return order, err
}
```

### Логирование

Используйте structured logging:

```go
logger.Info().
    Str("method", "CreateOrder").
    Str("erp_order", req.ERPOrderID).
    Str("cdek_uuid", order.UUID).
    Int("tariff", order.TariffCode).
    Dur("duration", time.Since(start)).
    Msg("order created successfully")

logger.Error().
    Err(err).
    Str("method", "CreateOrder").
    Str("erp_order", req.ERPOrderID).
    Msg("failed to create order")
```

---

## Поддержка

- 📘 [CDEK API Documentation](https://api.cdek.ru/v2/)
- 📖 [API Endpoints](API_ENDPOINTS.md)
- 🚀 [Deployment Guide](DEPLOYMENT.md)
- 🐛 [GitHub Issues](https://github.com/metrica-pro/cdek-go/issues)
