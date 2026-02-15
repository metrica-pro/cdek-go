# CDEK Go Client

[![Go Reference](https://pkg.go.dev/badge/github.com/metrica-pro/cdek-go.svg)](https://pkg.go.dev/github.com/metrica-pro/cdek-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/metrica-pro/cdek-go)](https://goreportcard.com/report/github.com/metrica-pro/cdek-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Официальный Go клиент для работы с [CDEK API v2](https://apidoc.cdek.ru/).

## Возможности

- ✅ **18 High-level Service методов** для упрощенной работы с CDEK:
  - Расчет стоимости, создание и управление заказами (CRUD)
  - Печать документов (штрих-коды, накладные)
  - Отслеживание статусов доставки
  - Справочники (города, регионы, ПВЗ)
  - Заявки на забор груза (intakes)
  - Webhooks для автоматических уведомлений
- ✅ **OAuth2** авторизация с автоматическим обновлением и кешированием токенов
- ✅ **Мультиаккаунт** поддержка (несколько аккаунтов CDEK одновременно)
- ✅ **Circuit Breaker** (sony/gobreaker) защита от каскадных сбоев
- ✅ **Structured Logging** (zerolog) для мониторинга и отладки
- ✅ **Thread-safe** реализация для многопоточных приложений
- ✅ **Поддержка юрлиц** - получатели с ИНН (companies) и физлица (individuals)
- ✅ Автогенерированный клиент из **OpenAPI 3.0** спецификации (40+ endpoints)
- ✅ **70.2% test coverage** domain кода + 16 интеграционных тестов

## Установка

\`\`\`bash
go get github.com/metrica-pro/cdek-go
\`\`\`

## Быстрый старт

### Простое использование (один аккаунт)

\`\`\`go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/metrica-pro/cdek-go/pkg/cdek"
)

func main() {
    // Создание конфигурации
    config := cdek.DefaultConfig(
        "your-client-id",
        "your-client-secret",
    )

    // Создание менеджера
    manager, err := cdek.NewManager(config)
    if err != nil {
        log.Fatal(err)
    }

    // Получение клиента
    client, _ := manager.GetDefaultClient()

    // Создание высокоуровневого сервиса (с Circuit Breaker защитой)
    service := cdek.NewService(client, nil)
    ctx := context.Background()

    // Проверка доступности API
    if err := service.HealthCheck(ctx); err != nil {
        log.Fatal("CDEK API недоступен:", err)
    }

    // Расчет стоимости доставки Москва → Санкт-Петербург
    cost, err := service.CalculateCost(ctx, &cdek.CostRequest{
        FromCityCode: 44,  // Москва
        ToCityCode:   137, // Санкт-Петербург
        Packages: []cdek.Package{
            {Weight: 1000, Length: 20, Width: 15, Height: 10}, // 1 кг
        },
    })
    if err != nil {
        log.Fatal("Ошибка расчета:", err)
    }

    // Вывод тарифов
    for _, tariff := range cost.Tariffs {
        fmt.Printf("%s: %.2f руб, %d-%d дней\n",
            tariff.TariffName, tariff.DeliverySum,
            tariff.PeriodMin, tariff.PeriodMax)
    }

    // Создание заказа (пример)
    order, err := service.CreateOrder(ctx, &cdek.OrderRequest{
        Type:       "1", // интернет-магазин
        TariffCode: cost.Tariffs[0].TariffCode,
        Recipient: cdek.Recipient{
            Contact: cdek.Contact{
                Name:   "Иванов Иван Иванович",
                Phones: []cdek.Phone{{Number: "+79001234567"}},
            },
        },
        ToLocation: cdek.Location{
            Code: &[]int32{137}[0], // СПб
        },
        Packages: []cdek.OrderPackage{
            {
                Number: "1",
                Weight: 1000,
                Items: []cdek.Item{
                    {Name: "Товар", WareKey: "SKU123", Cost: 1000, Weight: 1000, Amount: 1},
                },
            },
        },
    })
    if err != nil {
        log.Fatal("Ошибка создания заказа:", err)
    }

    fmt.Printf("✅ Заказ создан: %s\n", order.UUID)

    // Отслеживание заказа
    tracking, _ := service.TrackOrder(ctx, order.UUID)
    fmt.Printf("Статус: %s\n", tracking.CurrentStatus.Name)

    // Список ПВЗ
    points, _ := service.ListDeliveryPoints(ctx, &cdek.DeliveryPointsRequest{
        CityCode: "137",
        Type:     "PVZ",
    })
    fmt.Printf("Найдено %d пунктов выдачи\n", len(points))
}
\`\`\`

### Все методы Service API (18 методов)

#### Расчет стоимости и создание заказов

```go
// Расчет стоимости доставки
cost, err := service.CalculateCost(ctx, &cdek.CostRequest{
    FromCityCode: 44,   // Москва
    ToCityCode:   137,  // Санкт-Петербург
    Packages:     []cdek.Package{{Weight: 1000, Length: 20, Width: 15, Height: 10}},
})

// Создание заказа (физическое лицо)
order, err := service.CreateOrder(ctx, &cdek.OrderRequest{
    TariffCode: 136,
    Recipient: cdek.Recipient{
        Contact: cdek.Contact{
            Name:   "Иван Иванов",
            Phones: []cdek.Phone{{Number: "+79001234567"}},
        },
        PassportSeries: ptrString("1234"),
        PassportNumber: ptrString("567890"),
    },
    // ... остальные поля
})

// Создание заказа (юридическое лицо с ИНН)
order, err := service.CreateOrder(ctx, &cdek.OrderRequest{
    TariffCode: 136,
    Recipient: cdek.Recipient{
        Contact: cdek.Contact{
            Company: ptrString("ООО Компания"),
            Name:    "Петр Петров", // Контактное лицо
            Phones:  []cdek.Phone{{Number: "+79007654321"}},
        },
        TIN: ptrString("7707083893"), // ИНН (10 цифр для юрлиц, 12 для ИП)
    },
    // ... остальные поля
})
```

#### Управление заказами (CRUD)

```go
// Получение информации о заказе
orderInfo, err := service.GetOrder(ctx, orderUUID)

// Обновление заказа
updated, err := service.UpdateOrder(ctx, orderUUID, &cdek.UpdateOrderRequest{
    Recipient: &cdek.Recipient{...}, // Новые данные получателя
})

// Отмена заказа
err = service.CancelOrder(ctx, orderUUID)

// Отслеживание заказа
tracking, err := service.TrackOrder(ctx, orderUUID)
fmt.Printf("Статус: %s\n", tracking.CurrentStatus.Name)
fmt.Printf("История: %d событий\n", len(tracking.StatusHistory))
```

#### Печать документов

```go
// Создание задания на печать штрих-кодов
printJob, err := service.PrintBarcode(ctx, &cdek.PrintRequest{
    Orders: []cdek.PrintOrder{{OrderUUID: orderUUID}},
    Format: cdek.FormatA4,
})

// Скачивание PDF штрих-кодов (когда готово)
pdfData, err := service.DownloadBarcode(ctx, printJob.UUID)
ioutil.WriteFile("barcodes.pdf", pdfData, 0644)

// Создание задания на печать накладных
waybillJob, err := service.PrintWaybill(ctx, &cdek.PrintRequest{
    Orders: []cdek.PrintOrder{{OrderUUID: orderUUID}},
})

// Скачивание PDF накладных
pdfData, err := service.DownloadWaybill(ctx, waybillJob.UUID)
ioutil.WriteFile("waybill.pdf", pdfData, 0644)
```

#### Справочники и ПВЗ

```go
// Поиск городов
cities, err := service.ListCities(ctx, &cdek.CitiesRequest{
    City: ptrString("Москва"),
    Size: ptrInt32(10),
})

// Список регионов
regions, err := service.ListRegions(ctx, &cdek.RegionsRequest{
    CountryCodes: []string{"RU"},
    Size:         ptrInt32(50),
})

// Список пунктов выдачи
points, err := service.ListDeliveryPoints(ctx, &cdek.DeliveryPointsRequest{
    CityCode: "137",
    Type:     "PVZ",
})
for _, p := range points {
    fmt.Printf("%s: %s\n", p.Code, p.Location.Address)
}
```

#### Заявки на забор груза (Intakes)

```go
// Создание заявки на забор
intake, err := service.CreateIntake(ctx, &cdek.IntakeRequest{
    IntakeDate:   "2026-02-20",
    IntakeTimeFrom: "10:00",
    IntakeTimeTo:   "18:00",
    LunchTimeFrom:  ptrString("13:00"),
    LunchTimeTo:    ptrString("14:00"),
    Name:          "Иван Иванов",
    Phone:         "+79001234567",
})

// Получение информации о заявке
intakeInfo, err := service.GetIntake(ctx, intake.UUID)

// Отмена заявки на забор
err = service.DeleteIntake(ctx, intake.UUID)
```

#### Webhooks для уведомлений

```go
// Регистрация webhook
webhook, err := service.CreateWebhook(ctx, &cdek.WebhookRequest{
    URL:  "https://example.com/webhook/cdek",
    Type: "ORDER_STATUS",
})

// Список зарегистрированных webhooks
webhooks, err := service.ListWebhooks(ctx)
for _, wh := range webhooks {
    fmt.Printf("Webhook: %s (active: %t)\n", wh.URL, wh.Active)
}
```

**Вспомогательная функция:**
```go
func ptrString(s string) *string { return &s }
func ptrInt32(i int32) *int32 { return &i }
\`\`\`

## Архитектура

\`\`\`
User Code
  ↓
Service (high-level API + Circuit Breaker + Logging)
  ↓
  ├─ costCalculator (валидация)
  ├─ dtoMapper (преобразование типов)
  └─ AuthenticatedClient (OAuth2 + token caching)
      ↓
Client (автогенерированный из OpenAPI 3.0)
  ↓
CDEK API v2
\`\`\`

### Production-Ready Features

- **Circuit Breaker** (sony/gobreaker): защита от каскадных сбоев
  - Автоматическое открытие circuit при >= 60% ошибок
  - Half-open state с ограничением запросов
  - Timeout 60 секунд для восстановления
- **Structured Logging** (zerolog): опциональное логирование всех операций
- **Thread-Safe**: безопасная работа в многопоточной среде
- **Typed Errors**: удобная обработка ошибок API

## Лицензия

MIT License - см. [LICENSE](LICENSE)

---

Made with ❤️ by [Metrica Pro](https://metrica.pro)
