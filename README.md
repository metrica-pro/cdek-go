# CDEK Go Client

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Coverage](https://img.shields.io/badge/coverage-93.6%25-brightgreen)](./coverage.html)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

Официальный Go клиент для работы с CDEK API v2 (Российская служба доставки).

## ✨ Возможности

- ✅ **OAuth2 авторизация** с автоматическим обновлением токенов
- ✅ **Мультиаккаунт поддержка** - управление несколькими складами/аккаунтами
- ✅ **High-level API** - удобные обертки над сложными операциями
- ✅ **Полное покрытие 40+ CDEK endpoints** (автогенерировано из OpenAPI)
- ✅ **Thread-safe** - безопасное использование в concurrent окружении
- ✅ **Production tested** - проверено в боевых условиях
- ✅ **Typed errors** - типизированные ошибки для удобной обработки
- ✅ **Component architecture** - модульная архитектура по регламенту Metrica Pro

## 📦 Установка

```bash
go get github.com/metrica-pro/cdek-go/pkg/cdek
```

## 🚀 Быстрый старт

### Простой клиент

```go
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

    // High-level сервис
    service := cdek.NewService(client)
    ctx := context.Background()

    // Health check
    if err := service.HealthCheck(ctx); err != nil {
        log.Fatal("API недоступен:", err)
    }

    fmt.Println("✅ CDEK API доступен")
}
```

### Мультиаккаунт режим

```go
import "time"

config := &cdek.Config{
    Accounts: []cdek.AccountConfig{
        {
            Name:         "warehouse-moscow",
            ClientID:     "msk-client-id",
            ClientSecret: "msk-secret",
            BaseURL:      cdek.URLProduction,
            Timeout:      30 * time.Second,
        },
        {
            Name:         "warehouse-spb",
            ClientID:     "spb-client-id",
            ClientSecret: "spb-secret",
            BaseURL:      cdek.URLProduction,
            Timeout:      30 * time.Second,
        },
    },
    DefaultAccount: "warehouse-moscow",
}

manager, _ := cdek.NewManager(config)

// Использование разных аккаунтов
mskClient, _ := manager.GetClient("warehouse-moscow")
spbClient, _ := manager.GetClient("warehouse-spb")

// Health check всех аккаунтов параллельно
results := manager.HealthCheck(ctx)
for name, err := range results {
    if err != nil {
        fmt.Printf("❌ %s: %v\n", name, err)
    } else {
        fmt.Printf("✅ %s: OK\n", name)
    }
}
```

### Прямой доступ к API

```go
// Низкоуровневый доступ через автогенерированный клиент
apiClient := client.Client()

// Получение списка ПВЗ
params := &cdek.GetDeliverypointsParams{}
resp, err := apiClient.GetDeliverypointsWithResponse(ctx, params)
if err != nil {
    log.Fatal(err)
}

for _, point := range *resp.JSON200 {
    fmt.Printf("ПВЗ: %s, Адрес: %s\n",
        *point.Code,
        *point.Location.Address)
}
```

## 📚 Основные функции

- ✅ Создание заказов
- ✅ Расчет стоимости доставки
- ✅ Отслеживание статусов
- ✅ Печать документов (накладные, этикетки)
- ✅ Управление ПВЗ (пункты выдачи заказов)
- ✅ Webhooks
- ✅ Фотоуслуга (при подключении)

## 🏗 Архитектура

Проект следует **регламенту Metrica Pro** (go_project_rules_v_1_3.5.md):

```
pkg/cdek/
├── client.go          # Автогенерированный API клиент (НЕ РЕДАКТИРОВАТЬ)
├── config.go          # Конфигурация мультиаккаунта
├── auth.go            # OAuth2 с кешированием токенов
├── manager.go         # Менеджер аккаунтов
├── components.go      # Компоненты (camelCase по регламенту 4.6)
│   ├── tokenCache      - Кеширование OAuth токенов
│   ├── requestBuilder  - Построение HTTP запросов
│   ├── responseParser  - Парсинг API ответов
│   ├── orderValidator  - Валидация заказов
│   └── costCalculator  - Расчет стоимости
├── service.go         # High-level API обертка
├── errors.go          # Типизированные ошибки
└── *_test.go          # Unit тесты (93.6% coverage)
```

Подробнее: [ARCHITECTURE.md](ARCHITECTURE.md)

## 🧪 Тестирование

```bash
# Запуск тестов
go test -v ./pkg/cdek/...

# С race detector
go test -race ./pkg/cdek/...

# С покрытием
go test -coverprofile=coverage.out ./pkg/cdek/...
go tool cover -html=coverage.out
```

**Покрытие:** 93.6% для вручную написанного кода

## 🔧 Регенерация клиента

При обновлении OpenAPI спецификации:

```bash
# Установка oapi-codegen (если еще не установлен)
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

# Регенерация
oapi-codegen -config oapi-codegen-config.yaml api/cdek-api.yaml
```

## 📖 API Reference

- **CDEK API Docs:** https://apidoc.cdek.ru/
- **GoDoc:** https://pkg.go.dev/github.com/metrica-pro/cdek-go/pkg/cdek

## 🔗 Интеграция с ERP

Для интеграции в ERP систему см. [INTEGRATION_WITH_ERP.md](INTEGRATION_WITH_ERP.md)

## 📝 Требования

- Go >= 1.26
- Тестирование: >= 40% general, >= 70% domain code
- Линтер: golangci-lint
- OpenAPI 3.0 спецификация

## 🐛 Известные ограничения

### Фотоуслуга

Услуга "Фото документов" доступна только при:
- Режим доставки: **дверь-дверь** (delivery_mode = 1) или **склад-дверь** (delivery_mode = 3)
- Фотоуслуга **подключена в договоре** СДЭК
- Настроен **фотопроект** в личном кабинете

См. подробности: [docs/PHOTO_SERVICE_COMPLETE_GUIDE.md](docs/PHOTO_SERVICE_COMPLETE_GUIDE.md)

**Для подключения:** integrator@cdek.ru

## 📞 Поддержка

- **Техподдержка СДЭК:** integrator@cdek.ru
- **API Документация:** https://apidoc.cdek.ru/
- **Issues:** https://github.com/metrica-pro/cdek-go/issues

## 📄 Лицензия

MIT

---

**Разработано с соблюдением регламента Metrica Pro**
