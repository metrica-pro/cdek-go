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
passportSeries, passportNumber := "1234", "567890"
recipientEmail := "test@example.com"
fromCode, toCode := int32(44), int32(137) // Москва, СПб

order, err := service.CreateOrder(ctx, &cdek.OrderRequest{
    Type:       "1", // интернет-магазин
    TariffCode: 136,
    Sender: cdek.Recipient{ // Отправитель теперь тоже Recipient!
        Contact: cdek.Contact{
            Name:   "Иван Иванов",
            Phones: []cdek.Phone{{Number: "+79001234567"}},
        },
        PassportSeries: &passportSeries,
        PassportNumber: &passportNumber,
    },
    Recipient: cdek.Recipient{
        Contact: cdek.Contact{
            Name:  "Петр Петров",
            Email: &recipientEmail,
            Phones: []cdek.Phone{{Number: "+79007654321"}},
        },
    },
    FromLocation: cdek.Location{
        Code: &fromCode,
    },
    ToLocation: cdek.Location{
        Code: &toCode,
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
senderCompany, senderTIN := "ООО Отправитель", "7707083893" // ИНН 10 цифр (юрлицо) или 12 (ИП)
recipientCompany, recipientTIN := "ООО Получатель", "7709033880"
recipientEmail := "contact@company.ru"

order, err := service.CreateOrder(ctx, &cdek.OrderRequest{
    Type:       "1",
    TariffCode: 136,
    Sender: cdek.Recipient{
        Contact: cdek.Contact{
            Company: &senderCompany,
            Name:    "Иван Иванов", // Контактное лицо
            Phones:  []cdek.Phone{{Number: "+79001234567"}},
        },
        TIN: &senderTIN,
    },
    Recipient: cdek.Recipient{
        Contact: cdek.Contact{
            Company: &recipientCompany,
            Name:    "Петр Петров", // Контактное лицо
            Email:   &recipientEmail,
            Phones:  []cdek.Phone{{Number: "+79007654321"}},
        },
        TIN: &recipientTIN, // ИНН получателя
    },
    // ... остальные поля
})
```

**Интернет-магазин с третьим лицом (Seller):**

```go
marketplaceCompany, marketplaceTIN := "ООО Маркетплейс", "7707083893"
sellerName, sellerINN := "ИП Настоящий Продавец", "123456789012" // ИНН продавца
sellerPhone, sellerAddress := "+79991234567", "Москва, ул. Продавца, 1"
sellerOwnershipForm := 1 // Форма собственности

order, err := service.CreateOrder(ctx, &cdek.OrderRequest{
    Type:       "1", // обязательно "1" для интернет-магазина
    TariffCode: 136,
    Sender: cdek.Recipient{
        Contact: cdek.Contact{
            Company: &marketplaceCompany,
            Name:    "Служба доставки",
            Phones:  []cdek.Phone{{Number: "+74951234567"}},
        },
        TIN: &marketplaceTIN,
    },
    Recipient: cdek.Recipient{
        Contact: cdek.Contact{
            Name:   "Покупатель",
            Phones: []cdek.Phone{{Number: "+79001234567"}},
        },
    },
    Seller: &cdek.Seller{ // Истинный продавец (третье лицо)
        Name:          &sellerName,
        INN:           &sellerINN,
        Phone:         &sellerPhone,
        OwnershipForm: &sellerOwnershipForm,
        Address:       &sellerAddress,
    },
    // ... остальные поля
})
```

**Доставка в ПВЗ или между ПВЗ:**

```go
fromCode := int32(44)     // Москва (адрес курьером)
deliveryPoint := "SPB12"  // код ПВЗ из ListDeliveryPoints

// До пункта выдачи: DeliveryPoint задает сторону получателя.
// Одновременная передача DeliveryPoint и ToLocation — ошибка ErrInvalidRequest (XOR по CDEK)
order, err := service.CreateOrder(ctx, &cdek.OrderRequest{
    Type:       "1",
    TariffCode: 136, // тариф "склад-склад" или "дверь-склад"
    Recipient: cdek.Recipient{
        Contact: cdek.Contact{
            Name:   "Петр Петров",
            Phones: []cdek.Phone{{Number: "+79007654321"}},
        },
    },
    FromLocation:  cdek.Location{Code: &fromCode},
    DeliveryPoint: &deliveryPoint,
    Packages:      /* как выше */,
})

// ПВЗ → ПВЗ: оба пункта, адресные блоки не указываются
shipmentPoint := "MSK109" // сдача отправителем в ПВЗ
order, err = service.CreateOrder(ctx, &cdek.OrderRequest{
    Type:          "1",
    TariffCode:    136,
    Recipient:     /* как выше */,
    ShipmentPoint: &shipmentPoint,
    DeliveryPoint: &deliveryPoint,
    Packages:      /* как выше */,
})
```

`ShipmentPoint`/`DeliveryPoint` работают и в `UpdateOrder`. Код ПВЗ и адрес одной
стороны взаимоисключающие: `shipment_point`+`from_location` или `delivery_point`+`to_location`
в одном запросе — ошибка `ErrInvalidRequest` (CDEK их не принимает). Адрес с одной стороны
и ПВЗ с другой (например, `FromLocation` + `DeliveryPoint` для «дверь-склад») — валидно.

#### 3. GetOrder - Получение информации о заказе

```go
orderInfo, err := service.GetOrder(ctx, orderUUID)
fmt.Printf("Заказ %s, тариф %d\n", orderInfo.UUID, orderInfo.TariffCode)
fmt.Printf("Получатель: %s (%s)\n", orderInfo.Recipient.Name, *orderInfo.Recipient.Company)
if orderInfo.Recipient.TIN != nil {
    fmt.Printf("ИНН: %s\n", *orderInfo.Recipient.TIN)
}
if orderInfo.DeliveryPoint != "" {
    fmt.Printf("Доставка в ПВЗ: %s\n", orderInfo.DeliveryPoint)
}
```

#### 4. UpdateOrder - Обновление заказа

По спеке CDEK `OrderUpdateRequestDto` требует `type` и `recipient` — передавайте их в каждом PATCH.

```go
recipientCompany, recipientTIN := "ООО Новая Компания", "7709033880"

updated, err := service.UpdateOrder(ctx, &cdek.UpdateOrderRequest{
    OrderUUID: orderUUID,
    Type:      "1", // как в OrderRequest.Type ("1"/"online_store" или "2"/"delivery")
    Recipient: &cdek.Recipient{
        Contact: cdek.Contact{
            Company: &recipientCompany,
            Name:    "Новое Контактное Лицо",
            Phones:  []cdek.Phone{{Number: "+79998887766"}},
        },
        TIN: &recipientTIN,
    },
})
```

**Перевод заказа на другой ПВЗ:**

```go
deliveryPoint := "SPB34"

updated, err := service.UpdateOrder(ctx, &cdek.UpdateOrderRequest{
    OrderUUID: orderUUID,
    Type:      "1", // по спеке PATCH требует type и recipient
    Recipient: &cdek.Recipient{ /* как выше */ },
    DeliveryPoint: &deliveryPoint, // ToLocation в этом же запросе — ошибка (XOR)
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
haveCashless, allowedCOD := true, true
weightMax := int32(15) // кг

points, err := service.ListDeliveryPoints(ctx, &cdek.DeliveryPointsRequest{
    CityCode:     "137",         // Санкт-Петербург
    Type:         "PVZ",         // или "POSTAMAT", "ALL"
    HaveCashless: &haveCashless, // безналичная оплата
    AllowedCOD:   &allowedCOD,   // наложенный платеж
    WeightMax:    &weightMax,    // максимальный вес приема/выдачи, кг
})
for _, p := range points {
    fmt.Printf("%s: %s\n", p.Code, p.Location.AddressFull)
    fmt.Printf("  Наличные/безнал: %v/%v, наложенный платеж: %v\n",
        p.HaveCash, p.HaveCashless, p.AllowedCOD)
    if p.WeightMax != nil { // nil - поля нет в ответе
        fmt.Printf("  Максимальный вес: %.0f кг\n", *p.WeightMax)
    }
}
```

Вес в фильтрах и ответе — килограммы. Query-параметры кодируются через `net/url.Values`
(спецсимволы безопасны), `size`/`page` со значением <= 0 не отправляются. Полный набор фильтров:
`code`, `country_code`, `region_code`, `postal_code`, `fias_guid`, `have_cashless`, `have_cash`,
`allowed_cod`, `is_dressing_room`, `is_handout`, `is_reception`, `take_only`, `weight_max`,
`weight_min`, `size`, `page`.

#### 12. ListCities - Поиск городов

```go
cityName, size := "Москва", 10

cities, err := service.ListCities(ctx, &cdek.CitiesRequest{
    City: &cityName,
    Size: &size,
})
```

#### 13. ListRegions - Список регионов

```go
countryCode, size := "RU", 50

regions, err := service.ListRegions(ctx, &cdek.RegionsRequest{
    CountryCode: &countryCode,
    Size:        &size,
})
```

### 🚚 Заявки на забор груза

#### 14. CreateIntake - Создание заявки на забор

```go
intakeComment := "Звонить за час"
intakeCityCode, intakeAddress := int32(44), "Москва, ул. Ленина, 1"

intake, err := service.CreateIntake(ctx, &cdek.IntakeRequest{
    IntakeDate:     "2026-02-20",
    IntakeTimeFrom: "10:00",
    IntakeTimeTo:   "18:00",
    Comment:        &intakeComment,
    Sender: cdek.Contact{
        Name:   "Иван Иванов",
        Phones: []cdek.Phone{{Number: "+79001234567"}},
    },
    FromLocation: cdek.Location{
        Code:    &intakeCityCode,
        Address: &intakeAddress,
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

## Опциональные поля-указатели

Опциональные поля в запросах — указатели. Библиотека их не экспортирует:
берите адрес локальной переменной, как в примерах выше.

```go
cityName := "Москва"
size := 10

cities, _ := service.ListCities(ctx, &cdek.CitiesRequest{
    City: &cityName,
    Size: &size,
})
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
