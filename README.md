# CDEK Go Client v0.2.1

[![Go Reference](https://pkg.go.dev/badge/github.com/metrica-pro/cdek-go.svg)](https://pkg.go.dev/github.com/metrica-pro/cdek-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/metrica-pro/cdek-go)](https://goreportcard.com/report/github.com/metrica-pro/cdek-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Production-ready Go клиент для работы с [CDEK API v2](https://api.cdek.ru/v2/).

## Возможности

- ✅ **16 High-level Service методов** для упрощенной работы с CDEK API
- ✅ **OAuth2** авторизация с автоматическим обновлением и кешированием токенов
- ✅ **Circuit Breaker** (sony/gobreaker) защита от каскадных сбоев
- ✅ **Мультиаккаунт** поддержка (несколько аккаунтов CDEK одновременно)
- ✅ **Поддержка юрлиц и ИП** - получатели/отправители с ИНН
- ✅ **Поддержка третьих лиц (Seller)** - для интернет-магазинов
- ✅ **Thread-safe** реализация для многопоточных приложений
- ✅ **Structured Logging** (zerolog) для мониторинга и отладки
- ✅ **70%+ test coverage** + 16 интеграционных тестов
- ✅ Автогенерированный клиент из **OpenAPI 3.0** (40+ endpoints)

## Установка

```bash
go get github.com/metrica-pro/cdek-go
```

## Быстрый старт

### Простой пример

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/metrica-pro/cdek-go/pkg/cdek"
)

func main() {
    // 1. Создание конфигурации
    config := cdek.DefaultConfig(
        "your-client-id",
        "your-client-secret",
    )

    // 2. Создание менеджера
    manager, err := cdek.NewManager(config)
    if err != nil {
        log.Fatal(err)
    }

    // 3. Получение клиента
    client, _ := manager.GetDefaultClient()

    // 4. Создание высокоуровневого сервиса
    service := cdek.NewService(client, nil) // Circuit Breaker включен по умолчанию
    ctx := context.Background()

    // 5. Расчет стоимости доставки
    cost, err := service.CalculateCost(ctx, &cdek.CostRequest{
        FromCityCode: 44,  // Москва
        ToCityCode:   137, // Санкт-Петербург
        Packages: []cdek.Package{
            {Weight: 1000, Length: 20, Width: 15, Height: 10},
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Найдено %d тарифов:\n", len(cost.Tariffs))
    for _, t := range cost.Tariffs {
        fmt.Printf("  %s: %.2f руб (%d-%d дней)\n",
            t.TariffName, t.DeliverySum, t.PeriodMin, t.PeriodMax)
    }
}
```

## Service API - Все методы (16 штук)

### 📦 Расчет стоимости и заказы

#### 1. CalculateCost - Расчет стоимости доставки

```go
cost, err := service.CalculateCost(ctx, &cdek.CostRequest{
    FromCityCode: 44,
    ToCityCode:   137,
    Packages: []cdek.Package{
        {Weight: 1000, Length: 20, Width: 15, Height: 10},
    },
})
```

#### 2. CreateOrder - Создание заказа

**Физическое лицо (с паспортом):**

```go
order, err := service.CreateOrder(ctx, &cdek.OrderRequest{
    Type:       "1", // интернет-магазин
    TariffCode: 136,
    Sender: cdek.Recipient{ // Отправитель теперь тоже Recipient!
        Contact: cdek.Contact{
            Name:   "Иван Иванов",
            Phones: []cdek.Phone{{Number: "+79001234567"}},
        },
        PassportSeries: ptrString("1234"),
        PassportNumber: ptrString("567890"),
    },
    Recipient: cdek.Recipient{
        Contact: cdek.Contact{
            Name:   "Петр Петров",
            Email:  ptrString("test@example.com"),
            Phones: []cdek.Phone{{Number: "+79007654321"}},
        },
    },
    FromLocation: cdek.Location{
        Code: ptrInt32(44), // Москва
    },
    ToLocation: cdek.Location{
        Code: ptrInt32(137), // СПб
    },
    Packages: []cdek.OrderPackage{
        {
            Number: "1",
            Weight: 1000,
            Items: []cdek.Item{
                {
                    Name:    "Товар",
                    WareKey: "SKU-001",
                    Cost:    1000,
                    Weight:  1000,
                    Amount:  1,
                },
            },
        },
    },
})
```

**Юридическое лицо (с ИНН):**

```go
order, err := service.CreateOrder(ctx, &cdek.OrderRequest{
    Type:       "1",
    TariffCode: 136,
    Sender: cdek.Recipient{
        Contact: cdek.Contact{
            Company: ptrString("ООО Отправитель"),
            Name:    "Иван Иванов", // Контактное лицо
            Phones:  []cdek.Phone{{Number: "+79001234567"}},
        },
        TIN: ptrString("7707083893"), // ИНН 10 цифр (юрлицо) или 12 (ИП)
    },
    Recipient: cdek.Recipient{
        Contact: cdek.Contact{
            Company: ptrString("ООО Получатель"),
            Name:    "Петр Петров", // Контактное лицо
            Email:   ptrString("contact@company.ru"),
            Phones:  []cdek.Phone{{Number: "+79007654321"}},
        },
        TIN: ptrString("7709033880"), // ИНН получателя
    },
    // ... остальные поля
})
```

**Интернет-магазин с третьим лицом (Seller):**

```go
order, err := service.CreateOrder(ctx, &cdek.OrderRequest{
    Type:       "1", // обязательно "1" для интернет-магазина
    TariffCode: 136,
    Sender: cdek.Recipient{
        Contact: cdek.Contact{
            Company: ptrString("ООО Маркетплейс"),
            Name:    "Служба доставки",
            Phones:  []cdek.Phone{{Number: "+74951234567"}},
        },
        TIN: ptrString("7707083893"),
    },
    Recipient: cdek.Recipient{
        Contact: cdek.Contact{
            Name:   "Покупатель",
            Phones: []cdek.Phone{{Number: "+79001234567"}},
        },
    },
    Seller: &cdek.Seller{ // Истинный продавец (третье лицо)
        Name:          ptrString("ИП Настоящий Продавец"),
        INN:           ptrString("123456789012"), // ИНН продавца
        Phone:         ptrString("+79991234567"),
        OwnershipForm: ptrInt(1), // Форма собственности
        Address:       ptrString("Москва, ул. Продавца, 1"),
    },
    // ... остальные поля
})
```

#### 3. GetOrder - Получение информации о заказе

```go
orderInfo, err := service.GetOrder(ctx, orderUUID)
fmt.Printf("Заказ %s, тариф %d\n", orderInfo.UUID, orderInfo.TariffCode)
fmt.Printf("Получатель: %s (%s)\n", orderInfo.Recipient.Name, *orderInfo.Recipient.Company)
if orderInfo.Recipient.TIN != nil {
    fmt.Printf("ИНН: %s\n", *orderInfo.Recipient.TIN)
}
```

#### 4. UpdateOrder - Обновление заказа

```go
updated, err := service.UpdateOrder(ctx, &cdek.UpdateOrderRequest{
    OrderUUID: orderUUID,
    Recipient: &cdek.Recipient{
        Contact: cdek.Contact{
            Company: ptrString("ООО Новая Компания"),
            Name:    "Новое Контактное Лицо",
            Phones:  []cdek.Phone{{Number: "+79998887766"}},
        },
        TIN: ptrString("7709033880"),
    },
})
```

#### 5. CancelOrder - Отмена заказа

```go
err = service.CancelOrder(ctx, orderUUID)
```

#### 6. TrackOrder - Отслеживание заказа

```go
tracking, err := service.TrackOrder(ctx, orderUUID)
fmt.Printf("Текущий статус: %s\n", tracking.CurrentStatus.Name)
fmt.Printf("История: %d событий\n", len(tracking.StatusHistory))
for _, event := range tracking.StatusHistory {
    fmt.Printf("  %s: %s (%s)\n", event.DateTime, event.Name, event.Code)
}
```

### 📄 Печать документов

#### 7. PrintBarcode - Печать штрих-кодов

```go
printJob, err := service.PrintBarcode(ctx, &cdek.PrintRequest{
    Orders: []cdek.PrintOrder{
        {OrderUUID: orderUUID1},
        {OrderUUID: orderUUID2},
    },
    Format: cdek.FormatA4, // A4, A5, A6
})
fmt.Printf("Задание создано: %s\n", printJob.UUID)
```

#### 8. DownloadBarcode - Скачивание PDF штрих-кодов

```go
pdfData, err := service.DownloadBarcode(ctx, printJobUUID)
if err != nil {
    // Задание еще не готово, повторить позже
}
os.WriteFile("barcodes.pdf", pdfData, 0644)
```

#### 9. PrintWaybill - Печать накладных

```go
waybillJob, err := service.PrintWaybill(ctx, &cdek.PrintRequest{
    Orders: []cdek.PrintOrder{{OrderUUID: orderUUID}},
    Format: cdek.FormatA4,
})
```

#### 10. DownloadWaybill - Скачивание PDF накладных

```go
pdfData, err := service.DownloadWaybill(ctx, waybillJobUUID)
os.WriteFile("waybill.pdf", pdfData, 0644)
```

### 📍 Справочники

#### 11. ListDeliveryPoints - Список ПВЗ

```go
points, err := service.ListDeliveryPoints(ctx, &cdek.DeliveryPointsRequest{
    CityCode: "137", // Санкт-Петербург
    Type:     "PVZ", // или "POSTAMAT"
})
for _, p := range points {
    fmt.Printf("%s: %s\n", p.Code, p.Location.Address)
    fmt.Printf("  Режим работы: %s\n", p.WorkTime)
    fmt.Printf("  Координаты: %.6f, %.6f\n", p.Location.Latitude, p.Location.Longitude)
}
```

#### 12. ListCities - Поиск городов

```go
cities, err := service.ListCities(ctx, &cdek.CitiesRequest{
    City: ptrString("Москва"),
    Size: ptrInt32(10),
})
```

#### 13. ListRegions - Список регионов

```go
regions, err := service.ListRegions(ctx, &cdek.RegionsRequest{
    CountryCodes: []string{"RU"},
    Size:         ptrInt32(50),
})
```

### 🚚 Заявки на забор груза

#### 14. CreateIntake - Создание заявки на забор

```go
intake, err := service.CreateIntake(ctx, &cdek.IntakeRequest{
    IntakeDate:     "2026-02-20",
    IntakeTimeFrom: "10:00",
    IntakeTimeTo:   "18:00",
    Comment:        ptrString("Звонить за час"),
    Sender: cdek.Contact{
        Name:   "Иван Иванов",
        Phones: []cdek.Phone{{Number: "+79001234567"}},
    },
    FromLocation: cdek.Location{
        Code:    ptrInt32(44),
        Address: ptrString("Москва, ул. Ленина, 1"),
    },
    Orders: []cdek.IntakeOrder{
        {OrderUUID: orderUUID},
    },
})
```

#### 15. GetIntake - Информация о заявке на забор

```go
intakeInfo, err := service.GetIntake(ctx, intakeUUID)
```

#### 16. DeleteIntake - Отмена заявки на забор

```go
err = service.DeleteIntake(ctx, intakeUUID)
```

### 🔔 Webhooks (бонус - в плане)

```go
// Создание webhook (в разработке)
webhook, err := service.CreateWebhook(ctx, &cdek.WebhookRequest{
    URL:  "https://example.com/webhook/cdek",
    Type: "ORDER_STATUS",
})

// Список webhooks
webhooks, err := service.ListWebhooks(ctx)
```

## Вспомогательные функции

```go
func ptrString(s string) *string { return &s }
func ptrInt32(i int32) *int32 { return &i }
func ptrInt(i int) *int { return &i }
```

## Мультиаккаунт режим

```go
config := &cdek.Config{
    Accounts: []cdek.AccountConfig{
        {
            Name:         "warehouse-moscow",
            ClientID:     "msk-client-id",
            ClientSecret: "msk-secret",
            BaseURL:      cdek.URLProduction,
        },
        {
            Name:         "warehouse-spb",
            ClientID:     "spb-client-id",
            ClientSecret: "spb-secret",
            BaseURL:      cdek.URLProduction,
        },
    },
    DefaultAccount: "warehouse-moscow",
}

manager, _ := cdek.NewManager(config)

// Работа с разными аккаунтами
mskClient, _ := manager.GetClient("warehouse-moscow")
spbClient, _ := manager.GetClient("warehouse-spb")

// Health check всех аккаунтов параллельно
results := manager.HealthCheck(ctx)
for name, err := range results {
    if err != nil {
        log.Printf("❌ %s: %v", name, err)
    } else {
        log.Printf("✅ %s: OK", name)
    }
}
```

## Circuit Breaker защита

Circuit Breaker автоматически включен в Service и защищает от каскадных сбоев:

- **Открывается** при >= 60% ошибок (минимум 3 запроса)
- **Half-open** режим через 60 секунд
- **Max requests** в half-open: 5
- **Reset interval**: 30 секунд

Настройка:

```go
config := &cdek.ServiceConfig{
    BreakerName:        "cdek-api",
    BreakerMaxRequests: 5,
    BreakerInterval:    30 * time.Second,
    BreakerTimeout:     60 * time.Second,
}
service := cdek.NewService(client, config)
```

## Структурированное логирование

```go
import "github.com/rs/zerolog"

logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

config := &cdek.ServiceConfig{
    Logger: &logger,
}
service := cdek.NewService(client, config)

// Теперь все операции логируются:
// {"level":"info","method":"CalculateCost","from":44,"to":137,"time":"...","message":"calculating delivery cost"}
```

## Обработка ошибок

```go
import (
    "errors"
    "github.com/sony/gobreaker/v2"
)

order, err := service.CreateOrder(ctx, req)
if err != nil {
    // Circuit breaker открыт
    if errors.Is(err, gobreaker.ErrOpenState) {
        log.Println("CDEK API временно недоступен")
        return
    }

    // Проверка типа ошибки CDEK
    if cdekErr, ok := err.(*cdek.ErrorResponse); ok {
        log.Printf("CDEK API error [%s]: %s", cdekErr.Code, cdekErr.Message)
        return
    }

    // Другая ошибка
    log.Printf("Unexpected error: %v", err)
}
```

## Архитектура

```
User Code
  ↓
Service API (16 методов)
  ├─ Circuit Breaker (sony/gobreaker)
  ├─ Structured Logging (zerolog)
  └─ DTO Mapper (map[string]interface{} ↔ Service types)
      ↓
AuthenticatedClient
  ├─ OAuth2 Token Management
  └─ Thread-safe Token Caching
      ↓
Client (автогенерированный из OpenAPI 3.0)
  ↓
CDEK API v2 (https://api.cdek.ru)
```

## Документация

- [API Endpoints](docs/API_ENDPOINTS.md) - Полный список endpoints
- [Integration Guide](docs/INTEGRATION_GUIDE.md) - Интеграция в ERP/CRM
- [Deployment](docs/DEPLOYMENT.md) - Развертывание и настройка
- [CHANGELOG](CHANGELOG.md) - История изменений

## Требования

- Go 1.22+
- Аккаунт CDEK с API credentials

## Получение credentials

1. Зарегистрируйтесь на [CDEK](https://cdek.ru/)
2. Перейдите в личный кабинет → Интеграции → API
3. Создайте новое приложение
4. Получите `Client ID` и `Client Secret`

**Production URL:** `https://api.cdek.ru`

## Тестирование

```bash
# Unit тесты
go test -v ./pkg/cdek/...

# С race detector
go test -v -race ./pkg/cdek/...

# Coverage
go test -coverprofile=coverage.out ./pkg/cdek/...
go tool cover -html=coverage.out

# Интеграционные тесты (требуют credentials)
export CDEK_TEST_CLIENT_ID=your-client-id
export CDEK_TEST_CLIENT_SECRET=your-client-secret
go test -tags=integration -v ./pkg/cdek/...
```

## Примеры

См. директорию [examples/](examples/):
- [basic](examples/basic/) - Базовое использование
- [multi-account](examples/multi-account/) - Мультиаккаунт режим
- [erp-integration](examples/erp-integration/) - Интеграция в ERP систему

## Лицензия

MIT License - см. [LICENSE](LICENSE)

## Поддержка

- 🐛 [GitHub Issues](https://github.com/metrica-pro/cdek-go/issues)
- 📧 Email: support@metrica.pro
- 📘 [CDEK API Documentation](https://api.cdek.ru/v2/)

---

Made with ❤️ by [Metrica Pro](https://metrica.pro)
