# CDEK Go Client

[![Go Reference](https://pkg.go.dev/badge/github.com/metrica-pro/cdek-go.svg)](https://pkg.go.dev/github.com/metrica-pro/cdek-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/metrica-pro/cdek-go)](https://goreportcard.com/report/github.com/metrica-pro/cdek-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Официальный Go клиент для работы с [CDEK API v2](https://apidoc.cdek.ru/).

## Возможности

- ✅ OAuth2 авторизация с автоматическим обновлением токенов
- ✅ Мультиаккаунт поддержка (несколько аккаунтов CDEK одновременно)
- ✅ High-level API обертка для упрощения работы
- ✅ Полное покрытие 40+ CDEK API endpoints
- ✅ Thread-safe реализация
- ✅ Автогенерированный клиент из OpenAPI спецификации
- ✅ Покрытие тестами 89%

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

    // Создание высокоуровневого сервиса
    service := cdek.NewService(client)
    ctx := context.Background()

    // Проверка доступности API
    if err := service.HealthCheck(ctx); err != nil {
        log.Fatal("CDEK API недоступен:", err)
    }

    fmt.Println("✅ Подключение к CDEK API успешно!")
}
\`\`\`

## Архитектура

\`\`\`
User Code
  ↓
Service (high-level API)
  ↓
AuthenticatedClient (OAuth2 + кеширование)
  ↓
Client (автогенерированный из OpenAPI)
  ↓
CDEK API v2
\`\`\`

## Лицензия

MIT License - см. [LICENSE](LICENSE)

---

Made with ❤️ by [Metrica Pro](https://metrica.pro)
