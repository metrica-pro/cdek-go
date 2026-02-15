# CDEK Go Client

[![Go Reference](https://pkg.go.dev/badge/github.com/metrica-pro/cdek-go.svg)](https://pkg.go.dev/github.com/metrica-pro/cdek-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/metrica-pro/cdek-go)](https://goreportcard.com/report/github.com/metrica-pro/cdek-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Официальный Go клиент для работы с [CDEK API v2](https://apidoc.cdek.ru/).

## Возможности

- ✅ **OAuth2** авторизация с автоматическим обновлением токенов
- ✅ **Мультиаккаунт** поддержка (несколько аккаунтов CDEK одновременно)
- ✅ **High-level Service API** для упрощенной работы с CDEK
- ✅ **Circuit Breaker** защита от каскадных сбоев
- ✅ **Thread-safe** реализация
- ✅ Автогенерированный клиент из **OpenAPI 3.0** спецификации (40+ endpoints)
- ✅ Покрытие тестами **69%+**

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
}
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
