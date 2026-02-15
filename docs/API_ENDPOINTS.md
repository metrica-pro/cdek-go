# CDEK API v2 - Полный список Endpoints

Документация всех доступных endpoints в cdek-go библиотеке.

## Содержание

- [Service API (High-Level) - 16 методов](#service-api-high-level)
- [Low-Level Client API (Generated) - 40+ endpoints](#low-level-client-api)
- [Типы данных](#типы-данных)
- [Коды ошибок](#коды-ошибок)

---

## Service API (High-Level)

Рекомендуется использовать Service API для большинства задач. Включает автоматическую авторизацию, валидацию, Circuit Breaker и логирование.

### 📦 Заказы (Orders)

#### 1. CalculateCost - Расчет стоимости доставки

**Метод:** `service.CalculateCost(ctx, req)`

**Эндпоинт:** `POST /v2/calculator/tarifflist`

**Параметры:**
```go
type CostRequest struct {
    FromCityCode int32     // Код города отправления (обязательно)
    ToCityCode   int32     // Код города получения (обязательно)
    Packages     []Package // Список мест (обязательно)
}

type Package struct {
    Weight int32  // Вес в граммах (обязательно)
    Length int32  // Длина в см
    Width  int32  // Ширина в см
    Height int32  // Высота в см
}
```

**Ответ:**
```go
type CostResponse struct {
    Tariffs []TariffOption
}

type TariffOption struct {
    TariffCode   int     // Код тарифа
    TariffName   string  // Название тарифа
    DeliveryMode int     // Режим доставки (1-дверь-дверь, 2-дверь-склад, 3-склад-дверь, 4-склад-склад)
    DeliverySum  float64 // Стоимость доставки (руб)
    PeriodMin    int     // Минимальный срок (дней)
    PeriodMax    int     // Максимальный срок (дней)
}
```

**Пример:**
```go
cost, err := service.CalculateCost(ctx, &cdek.CostRequest{
    FromCityCode: 44,  // Москва
    ToCityCode:   137, // СПб
    Packages: []cdek.Package{{Weight: 1000, Length: 20, Width: 15, Height: 10}},
})
```

---

#### 2. CreateOrder - Создание заказа

**Метод:** `service.CreateOrder(ctx, req)`

**Эндпоинт:** `POST /v2/orders`

**Параметры:**
```go
type OrderRequest struct {
    Type         string          // "1" - интернет-магазин, "2" - доставка
    TariffCode   int             // Код тарифа (обязательно)
    Comment      *string         // Комментарий
    Sender       Recipient       // Отправитель (обязательно)
    Recipient    Recipient       // Получатель (обязательно)
    Seller       *Seller         // Истинный продавец (для интернет-магазинов)
    FromLocation Location        // Адрес отправления
    ToLocation   Location        // Адрес получения
    Packages     []OrderPackage  // Список мест (обязательно)
}

type Recipient struct {
    Contact
    TIN                  *string // ИНН (10 или 12 цифр)
    PassportSeries       *string // Серия паспорта
    PassportNumber       *string // Номер паспорта
    PassportDateOfIssue  *string // Дата выдачи (ISO 8601)
    PassportOrganization *string // Орган выдачи
    PassportDateOfBirth  *string // Дата рождения (ISO 8601)
}

type Contact struct {
    Company *string // Название компании
    Name    string  // ФИО или название (обязательно)
    Email   *string // Email
    Phones  []Phone // Телефоны (обязательно хотя бы один)
}

type Seller struct {
    Name          *string // Наименование истинного продавца
    INN           *string // ИНН продавца
    Phone         *string // Телефон продавца
    OwnershipForm *int    // Форма собственности (1-63, см. Приложение 2)
    Address       *string // Адрес продавца
}

type Location struct {
    Code        *int32  // Код CDEK населенного пункта
    FiasGuid    *string // ФИАС UUID
    PostalCode  *string // Почтовый индекс
    CountryCode *string // Код страны (RU, BY, KZ, etc.)
    Region      *string // Регион
    City        *string // Город
    Address     *string // Адрес строкой
}

type OrderPackage struct {
    Number  string       // Номер упаковки (обязательно, уникальный)
    Weight  int32        // Вес в граммах (обязательно)
    Length  *int32       // Длина в см
    Width   *int32       // Ширина в см
    Height  *int32       // Высота в см
    Comment *string      // Комментарий
    Items   []Item       // Позиции товаров (обязательно для type="1")
}

type Item struct {
    Name    string  // Наименование товара (обязательно)
    WareKey string  // Артикул (обязательно)
    Payment float64 // Оплата за единицу при получении (0 если предоплата)
    Cost    float64 // Объявленная стоимость (обязательно)
    Weight  int32   // Вес единицы в граммах (обязательно)
    Amount  int32   // Количество (обязательно, 1-999)
}
```

**Ответ:**
```go
type OrderResponse struct {
    UUID       string        // UUID заказа в CDEK
    Number     *string       // Номер заказа CDEK
    TariffCode int           // Код тарифа
    Statuses   []StatusEvent // Статусы
    CreatedAt  string        // Дата создания (ISO 8601)
}
```

**Пример:**
```go
order, err := service.CreateOrder(ctx, &cdek.OrderRequest{
    Type:       "1",
    TariffCode: 136,
    Sender: cdek.Recipient{
        Contact: cdek.Contact{
            Company: ptrString("ООО Отправитель"),
            Name:    "Иван Иванов",
            Phones:  []cdek.Phone{{Number: "+79001234567"}},
        },
        TIN: ptrString("7707083893"),
    },
    Recipient: cdek.Recipient{
        Contact: cdek.Contact{
            Name:   "Петр Петров",
            Phones: []cdek.Phone{{Number: "+79007654321"}},
        },
    },
    FromLocation: cdek.Location{Code: ptrInt32(44)},
    ToLocation:   cdek.Location{Code: ptrInt32(137)},
    Packages: []cdek.OrderPackage{{
        Number: "1",
        Weight: 1000,
        Items:  []cdek.Item{{Name: "Товар", WareKey: "SKU-001", Cost: 1000, Weight: 1000, Amount: 1}},
    }},
})
```

---

#### 3. GetOrder - Получение информации о заказе

**Метод:** `service.GetOrder(ctx, uuid)`

**Эндпоинт:** `GET /v2/orders/{uuid}`

**Параметры:**
- `uuid` (string) - UUID заказа в CDEK

**Ответ:**
```go
type OrderInfo struct {
    UUID              string
    Number            *string
    Type              string
    TariffCode        int
    Sender            Recipient
    Recipient         Recipient
    Seller            *Seller
    FromLocation      Location
    ToLocation        Location
    Packages          []OrderPackage
    Statuses          []StatusEvent
    CreatedAt         string
    DeliveryCost      *float64
    EstimatedDelivery *string
    ActualDelivery    *string
}
```

---

#### 4. UpdateOrder - Обновление заказа

**Метод:** `service.UpdateOrder(ctx, req)`

**Эндпоинт:** `PATCH /v2/orders/{uuid}`

**Параметры:**
```go
type UpdateOrderRequest struct {
    OrderUUID    string      // UUID заказа (обязательно)
    Recipient    *Recipient  // Новые данные получателя
    Sender       *Recipient  // Новые данные отправителя
    Seller       *Seller     // Новые данные продавца
    ToLocation   *Location   // Новый адрес доставки
    FromLocation *Location   // Новый адрес отправления
    Comment      *string     // Новый комментарий
    Packages     []OrderPackage // Новые места (полная замена)
}
```

**Ответ:** `OrderResponse` (аналогично CreateOrder)

**Ограничения:**
- Можно обновлять только заказы в статусе CREATED или ACCEPTED
- Нельзя изменить TariffCode после создания
- При изменении Packages происходит полная замена списка

---

#### 5. CancelOrder - Отмена заказа

**Метод:** `service.CancelOrder(ctx, uuid)`

**Эндпоинт:** `DELETE /v2/orders/{uuid}`

**Параметры:**
- `uuid` (string) - UUID заказа

**Ответ:** `error` (nil если успешно)

**Ограничения:**
- Можно отменить только заказы в начальных статусах
- После передачи на доставку отмена невозможна

---

#### 6. TrackOrder - Отслеживание заказа

**Метод:** `service.TrackOrder(ctx, uuid)`

**Эндпоинт:** `GET /v2/orders/{uuid}` (парсинг статусов)

**Параметры:**
- `uuid` (string) - UUID заказа

**Ответ:**
```go
type TrackingInfo struct {
    UUID              string
    Number            *string
    CurrentStatus     StatusEvent   // Последний статус
    StatusHistory     []StatusEvent // Вся история
    EstimatedDelivery *string       // Планируемая дата (ISO 8601)
    ActualDelivery    *string       // Фактическая дата (ISO 8601)
}

type StatusEvent struct {
    Code     string  // Код статуса (CREATED, ACCEPTED, DELIVERED, etc.)
    Name     string  // Название статуса
    DateTime string  // Дата и время (ISO 8601)
    City     *string // Город события
}
```

**Коды статусов:**
- `CREATED` - Создан
- `ACCEPTED` - Принят на склад
- `DELIVERED_SENDER_CITY_CDEK` - Доставлен на склад CDEK отправителя
- `READY_FOR_SHIPMENT_IN_SENDER_CITY` - Готов к отправке
- `TAKEN_BY_TRANSPORTER_FROM_SENDER_CITY` - Передан на доставку
- `SENT_TO_TRANSIT_CITY` - Отправлен в транзитный город
- `ARRIVED_AT_TRANSIT_CITY` - Прибыл в транзитный город
- `SENT_TO_RECIPIENT_CITY` - Отправлен в город получателя
- `ARRIVED_AT_RECIPIENT_CITY` - Прибыл в город получателя
- `READY_FOR_RECIPIENT_CITY_DELIVERY` - Готов к выдаче
- `DELIVERED` - Доставлен получателю
- `NOT_DELIVERED` - Не доставлен
- `RETURNED` - Возвращен отправителю

---

### 📄 Печать документов

#### 7. PrintBarcode - Печать штрих-кодов

**Метод:** `service.PrintBarcode(ctx, req)`

**Эндпоинт:** `POST /v2/print/barcodes`

**Параметры:**
```go
type PrintRequest struct {
    Orders []PrintOrder // Список заказов (обязательно, макс 1000)
    Format string       // Формат: "A4", "A5", "A6" (по умолчанию A4)
}

type PrintOrder struct {
    OrderUUID string // UUID заказа (обязательно)
}
```

**Ответ:**
```go
type PrintResponse struct {
    UUID string // UUID задания на печать
}
```

---

#### 8. DownloadBarcode - Скачивание PDF штрих-кодов

**Метод:** `service.DownloadBarcode(ctx, printUUID)`

**Эндпоинт:** `GET /v2/print/barcodes/{uuid}`

**Параметры:**
- `printUUID` (string) - UUID задания на печать

**Ответ:** `[]byte` (PDF file)

**Ошибки:**
- `ErrNotFound (404)` - Задание еще не готово, повторить через несколько секунд

**Пример с retry:**
```go
var pdfData []byte
var err error
for i := 0; i < 10; i++ {
    pdfData, err = service.DownloadBarcode(ctx, printJob.UUID)
    if err == nil {
        break
    }
    if errors.Is(err, cdek.ErrNotFound) {
        time.Sleep(2 * time.Second)
        continue
    }
    return err
}
os.WriteFile("barcodes.pdf", pdfData, 0644)
```

---

#### 9. PrintWaybill - Печать накладных

**Метод:** `service.PrintWaybill(ctx, req)`

**Эндпоинт:** `POST /v2/print/orders`

**Параметры:** Аналогично PrintBarcode

---

#### 10. DownloadWaybill - Скачивание PDF накладных

**Метод:** `service.DownloadWaybill(ctx, printUUID)`

**Эндпоинт:** `GET /v2/print/orders/{uuid}`

**Параметры:** Аналогично DownloadBarcode

---

### 📍 Справочники

#### 11. ListDeliveryPoints - Список ПВЗ

**Метод:** `service.ListDeliveryPoints(ctx, req)`

**Эндпоинт:** `GET /v2/deliverypoints`

**Параметры:**
```go
type DeliveryPointsRequest struct {
    CityCode string // Код города CDEK (обязательно)
    Type     string // Тип: "PVZ" (пункт выдачи), "POSTAMAT" (постамат), "ALL" (все)
}
```

**Ответ:**
```go
type DeliveryPoint struct {
    Code        string        // Код ПВЗ
    Name        string        // Название
    Type        string        // Тип (PVZ, POSTAMAT)
    Location    PointLocation // Адрес и координаты
    WorkTime    string        // Режим работы
    Phones      []Phone       // Телефоны
    Email       *string       // Email
    Note        *string       // Примечание
    OfficeImage *string       // URL фото
}

type PointLocation struct {
    Country     string  // Страна
    Region      string  // Регион
    City        string  // Город
    Address     string  // Адрес
    PostalCode  string  // Индекс
    Latitude    float64 // Широта
    Longitude   float64 // Долгота
}
```

**Пример:**
```go
points, err := service.ListDeliveryPoints(ctx, &cdek.DeliveryPointsRequest{
    CityCode: "137",
    Type:     "PVZ",
})
for _, p := range points {
    fmt.Printf("%s: %s (%.6f, %.6f)\n", p.Code, p.Location.Address, p.Location.Latitude, p.Location.Longitude)
}
```

---

#### 12. ListCities - Поиск городов

**Метод:** `service.ListCities(ctx, req)`

**Эндпоинт:** `GET /v2/location/cities`

**Параметры:**
```go
type CitiesRequest struct {
    CountryCodes []string // Коды стран (RU, BY, KZ, etc.)
    RegionCode   *int32   // Код региона CDEK
    City         *string  // Название города (поиск)
    PostalCode   *string  // Почтовый индекс
    FiasGuid     *string  // ФИАС UUID
    Size         *int32   // Количество результатов (по умолчанию 1000)
}
```

**Ответ:**
```go
type City struct {
    Code         int32   // Код города CDEK
    City         string  // Название города
    FiasGuid     *string // ФИАС UUID
    Region       string  // Регион
    RegionCode   int32   // Код региона CDEK
    Country      string  // Страна
    CountryCode  string  // Код страны (ISO)
    Latitude     float64 // Широта
    Longitude    float64 // Долгота
    TimeZone     string  // Временная зона
    PaymentLimit float64 // Лимит наложенного платежа
}
```

---

#### 13. ListRegions - Список регионов

**Метод:** `service.ListRegions(ctx, req)`

**Эндпоинт:** `GET /v2/location/regions`

**Параметры:**
```go
type RegionsRequest struct {
    CountryCodes []string // Коды стран (обязательно)
    Region       *string  // Название региона (поиск)
    Size         *int32   // Количество результатов
}
```

**Ответ:**
```go
type Region struct {
    Code        int32   // Код региона CDEK
    Region      string  // Название региона
    Country     string  // Страна
    CountryCode string  // Код страны
    FiasGuid    *string // ФИАС UUID
}
```

---

### 🚚 Заявки на забор груза (Intakes)

#### 14. CreateIntake - Создание заявки на забор

**Метод:** `service.CreateIntake(ctx, req)`

**Эндпоинт:** `POST /v2/intakes`

**Параметры:**
```go
type IntakeRequest struct {
    IntakeDate     string       // Дата забора (YYYY-MM-DD) (обязательно)
    IntakeTimeFrom string       // Время начала (HH:MM) (обязательно)
    IntakeTimeTo   string       // Время конца (HH:MM) (обязательно)
    LunchTimeFrom  *string      // Время начала обеда (HH:MM)
    LunchTimeTo    *string      // Время конца обеда (HH:MM)
    Comment        *string      // Комментарий
    Sender         Contact      // Отправитель (обязательно)
    FromLocation   Location     // Адрес забора (обязательно)
    NeedCall       *bool        // Нужен ли звонок (по умолчанию true)
    Orders         []IntakeOrder // Список заказов для забора (обязательно)
}

type IntakeOrder struct {
    OrderUUID string // UUID заказа
}
```

**Ответ:**
```go
type IntakeResponse struct {
    UUID       string // UUID заявки
    IntakeDate string // Дата забора
    Number     string // Номер заявки CDEK
}
```

**Ограничения:**
- Дата забора должна быть в будущем
- Можно создать заявку только на заказы в статусе CREATED или ACCEPTED
- Время работы склада должно входить в интервал IntakeTimeFrom-IntakeTimeTo

---

#### 15. GetIntake - Информация о заявке

**Метод:** `service.GetIntake(ctx, uuid)`

**Эндпоинт:** `GET /v2/intakes/{uuid}`

**Параметры:**
- `uuid` (string) - UUID заявки

**Ответ:**
```go
type IntakeInfo struct {
    UUID           string
    Number         string
    IntakeDate     string
    IntakeTimeFrom string
    IntakeTimeTo   string
    Statuses       []StatusEvent
    Orders         []IntakeOrder
}
```

---

#### 16. DeleteIntake - Отмена заявки на забор

**Метод:** `service.DeleteIntake(ctx, uuid)`

**Эндпоинт:** `DELETE /v2/intakes/{uuid}`

**Параметры:**
- `uuid` (string) - UUID заявки

**Ответ:** `error` (nil если успешно)

**Ограничения:**
- Можно отменить только заявки в статусе CREATED

---

### 🔔 Webhooks (в разработке)

#### 17. ListWebhooks - Список webhooks

**Метод:** `service.ListWebhooks(ctx)`

**Эндпоинт:** `GET /v2/webhooks`

**Ответ:**
```go
type Webhook struct {
    UUID   string // UUID webhook
    URL    string // URL для уведомлений
    Type   string // Тип события
    Active bool   // Активен ли webhook
}
```

#### 18. CreateWebhook - Создание webhook

**Метод:** `service.CreateWebhook(ctx, req)` (в плане)

**Эндпоинт:** `POST /v2/webhooks`

---

## Low-Level Client API (Generated)

Для продвинутых сценариев доступен низкоуровневый автогенерированный клиент с 40+ endpoints.

### Использование

```go
client, _ := manager.GetDefaultClient()
token, _ := client.GetToken(ctx)

// Прямой вызов OpenAPI метода
resp, err := client.ClientWithResponses().GetOrderByUUIDWithResponse(
    ctx,
    orderUUID,
    nil,
    func(ctx context.Context, req *http.Request) error {
        req.Header.Set("Authorization", "Bearer "+token)
        return nil
    },
)
```

### Список Low-Level Endpoints

**Orders:**
- `POST /v2/orders` - Создание заказа
- `GET /v2/orders` - Список заказов (с фильтрами)
- `GET /v2/orders/{uuid}` - Информация о заказе
- `PATCH /v2/orders/{uuid}` - Обновление заказа
- `DELETE /v2/orders/{uuid}` - Отмена заказа

**Calculator:**
- `POST /v2/calculator/tariff` - Расчет одного тарифа
- `POST /v2/calculator/tarifflist` - Расчет всех доступных тарифов

**Delivery Points:**
- `GET /v2/deliverypoints` - Список ПВЗ

**Regions & Cities:**
- `GET /v2/location/regions` - Справочник регионов
- `GET /v2/location/cities` - Справочник городов

**Print:**
- `POST /v2/print/barcodes` - Печать штрих-кодов
- `GET /v2/print/barcodes/{uuid}` - Скачать штрих-коды
- `POST /v2/print/orders` - Печать накладных
- `GET /v2/print/orders/{uuid}` - Скачать накладные

**Intakes:**
- `POST /v2/intakes` - Создание заявки на забор
- `GET /v2/intakes` - Список заявок
- `GET /v2/intakes/{uuid}` - Информация о заявке
- `DELETE /v2/intakes/{uuid}` - Отмена заявки

**Webhooks:**
- `GET /v2/webhooks` - Список webhooks
- `POST /v2/webhooks` - Регистрация webhook
- `DELETE /v2/webhooks/{uuid}` - Удаление webhook

**И другие...**

Полный список см. в [OpenAPI спецификации](../api/cdek-api.yaml).

---

## Типы данных

### Форматы дат

Все даты в ISO 8601 format:
- **Дата:** `2026-02-15`
- **Дата и время:** `2026-02-15T14:30:00+0300`
- **Время:** `14:30`

### Коды тарифов

Основные тарифы:
- `136` - Посылка склад-склад
- `137` - Посылка склад-дверь
- `138` - Посылка дверь-склад
- `139` - Посылка дверь-дверь
- `366` - Посылка дверь-постамат
- `368` - Посылка склад-постамат
- `480` - Экспресс дверь-дверь
- `481` - Экспресс дверь-склад
- `482` - Экспресс склад-дверь
- `483` - Экспресс склад-склад

Полный список получайте через `CalculateCost`.

### Режимы доставки (DeliveryMode)

- `1` - дверь-дверь
- `2` - дверь-склад (ПВЗ/постамат)
- `3` - склад-дверь
- `4` - склад-склад

---

## Коды ошибок

### HTTP статусы

- `200 OK` - Успешный запрос
- `400 Bad Request` - Невалидные данные
- `401 Unauthorized` - Неверные credentials или истекший токен
- `404 Not Found` - Ресурс не найден
- `409 Conflict` - Конфликт данных (например, заказ уже существует)
- `422 Unprocessable Entity` - Бизнес-логика не позволяет выполнить операцию
- `429 Too Many Requests` - Rate limit превышен
- `500 Internal Server Error` - Ошибка на стороне CDEK
- `503 Service Unavailable` - CDEK API временно недоступен

### Обработка ошибок

```go
order, err := service.CreateOrder(ctx, req)
if err != nil {
    // Circuit breaker открыт
    if errors.Is(err, gobreaker.ErrOpenState) {
        return handleCircuitOpen()
    }

    // CDEK API ошибка
    if cdekErr, ok := err.(*cdek.ErrorResponse); ok {
        switch cdekErr.Code {
        case "validation_error":
            return handleValidationError(cdekErr)
        case "invalid_credentials":
            return handleAuthError(cdekErr)
        default:
            return handleGenericCDEKError(cdekErr)
        }
    }

    // Другая ошибка (сеть, таймаут, etc.)
    return handleGenericError(err)
}
```

### Typed Errors

Библиотека предоставляет typed errors:
- `cdek.ErrNotFound` - Ресурс не найден (404)
- `cdek.ErrUnauthorized` - Ошибка авторизации (401)
- `cdek.ErrInvalidRequest` - Невалидный запрос (400)
- `cdek.ErrRateLimited` - Rate limit (429)
- `cdek.ErrServerError` - Ошибка сервера (500+)
- `gobreaker.ErrOpenState` - Circuit breaker открыт

---

## Rate Limits

CDEK API имеет лимиты:
- **Авторизация:** 10 запросов/минуту на IP
- **API операции:** Зависит от метода (обычно 100-1000 запросов/минуту)

При превышении лимита:
- HTTP 429 Too Many Requests
- Header `Retry-After` указывает время ожидания

**Рекомендации:**
- Используйте Circuit Breaker (включен по умолчанию)
- Кешируйте справочники (города, регионы, ПВЗ)
- Батчите операции где возможно (PrintBarcode до 1000 заказов)

---

## Поддержка

- 📘 [CDEK Official API Docs](https://api.cdek.ru/v2/)
- 🐛 [GitHub Issues](https://github.com/metrica-pro/cdek-go/issues)
- 📧 Email: support@metrica.pro
