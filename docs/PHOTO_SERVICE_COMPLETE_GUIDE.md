# 📸 CDEK Photo Service - Полное Руководство

**Дата:** 2026-02-15
**Договор:** ИМ1222128
**API:** Production (https://api.cdek.ru)

---

## ✅ Подтверждено от Техподдержки СДЭК

**Email:** integrator@cdek.ru
**Ответ:**

> "Услуга 'Фото документов' доступна при режиме доставки 'до двери'.
> Фото документов - **пилот дополнительной услуги**, при которой курьер
> фотографирует документы и получателя по заданию клиента и передает
> фото через систему СДЭК.
> **Для данных заказов подключение такой услуги, к сожалению, невозможно.**"

---

## 📋 Что Мы Узнали

### 1. Режимы Доставки для Фотоуслуги

Согласно техподдержке, фотоуслуга доступна при:

**✅ Поддерживаемые режимы:**
- **delivery_mode = 1** - дверь-дверь
- **delivery_mode = 3** - склад-дверь

**❌ НЕ поддерживаемые режимы:**
- delivery_mode = 4 - склад-склад (наш первый заказ #10226542899)
- delivery_mode = 7 - склад-постамат
- и другие режимы без доставки "до двери"

### 2. Типы Заказов

**Тип 1 - "Интернет-магазин":**
- Только для договоров типа "Договор с ИМ"
- Не поддерживает фотоуслугу (по крайней мере для нашего договора)

**Тип 2 - "Доставка":**
- Для любого договора
- Требует обязательное поле `sender.company`
- Но при указании company → API возвращает ошибку "должно быть физлицо"

### 3. Тарифы Дверь-Дверь

**Найдено 17+ тарифов с режимом "дверь-дверь":**

| Код | Название |
|-----|----------|
| 342 | Parcel Express |
| 2281 | Parcel Standard |
| 2291 | Parcel Economy |
| 293 | E-com Express |
| 2311 | Экспресс тяжеловесы |
| 2327 | Business Express TEST LK |
| 2493 | Parcel Express. |
| 2573 | Экспресс без договора |
| 2593 | Интеграция FBS |
| 547 | CDEK Global Express Document |
| 554 | CDEK Global Standard |
| 568 | My Global Standard |
| 657 | CDEK Express Electronics |
| 7 | Международный экспресс документы |
| 726 | Business Cargo Express |
| 8 | Международный экспресс грузы |
| 813 | Business Cargo Standard |

---

## 🧪 Результаты Тестирования API

### Тест 1: Склад-Склад (Заказ #10226542899)

```json
{
  "type": 1,
  "tariff_code": 136,
  "shipment_point": "MSK2326",
  "delivery_point": "SPB335",
  "services": [{"code": "INSURANCE", "parameter": "2500"}]
}
```

**Результат:** ✅ Заказ создан успешно
**Фото:** ❌ Режим доставки 4 (склад-склад) - фотоуслуга недоступна

---

### Тест 2: Дверь-Дверь, Тип 1 (ИМ) + PHOTO_OF_DOCUMENTS

```json
{
  "type": 1,
  "tariff_code": 342,
  "services": [
    {"code": "INSURANCE", "parameter": "2500"},
    {"code": "PHOTO_OF_DOCUMENTS"}
  ],
  "sender": {
    "company": "OOO Test",
    "name": "Ivan Ivanov"
  }
}
```

**Результат:** ❌ INVALID
**Ошибки:**
- `err_invalid_tariff_with_ordertype` - "Несоответствие выбранной услуги типу заказа"
- `ve_calc_contragent_type_sender_incorrect` - "Выбранная услуга доступна для заказов, где отправителем является Физическое лицо"

---

### Тест 3: Дверь-Дверь, Тип 1 (ИМ) + INDIVIDUAL

```json
{
  "type": 1,
  "tariff_code": 342,
  "services": [
    {"code": "INSURANCE", "parameter": "2500"},
    {"code": "PHOTO_OF_DOCUMENTS"}
  ],
  "sender": {
    "name": "Иванов Иван Иванович",
    "contragent_type": "INDIVIDUAL"
  }
}
```

**Результат:** ❌ INVALID
**Ошибка:** Та же - требует физлицо (даже с указанным INDIVIDUAL)

---

### Тест 4: Дверь-Дверь, Тип 2 (Доставка) + Company

```json
{
  "type": 2,
  "tariff_code": 342,
  "services": [
    {"code": "INSURANCE", "parameter": "2500"},
    {"code": "PHOTO_OF_DOCUMENTS"}
  ],
  "sender": {
    "company": "Test Company LLC",
    "name": "Иванов Иван Иванович"
  },
  "packages": [{
    "comment": "required field"
  }]
}
```

**Результат:** ❌ INVALID
**Ошибка:** `ve_calc_contragent_type_sender_incorrect` - требует физлицо

---

### Тест 5: Дверь-Дверь, Тип 2 (Доставка) БЕЗ Company

```json
{
  "type": 2,
  "tariff_code": 342,
  "services": [
    {"code": "INSURANCE", "parameter": "2500"},
    {"code": "PHOTO_OF_DOCUMENTS"}
  ],
  "sender": {
    "name": "Иванов Иван Иванович"
  }
}
```

**Результат:** ❌ INVALID
**Ошибка:** `v2_field_is_empty` - `[sender.company] is empty`

---

## ⚠️ Парадокс API

**С полем `company`:**
```
❌ "Выбранная услуга доступна для заказов, где отправителем является Физическое лицо"
```

**БЕЗ поля `company`:**
```
❌ "[sender.company] is empty"
```

**Вывод:** Для договора ИМ1222128 фотоуслуга **не подключена** и не может быть использована через API.

---

## 📊 POST /v2/photoDocument - Проверка Фото

### Запрос По Периоду

```bash
curl -X POST "https://api.cdek.ru/v2/photoDocument" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "period_begin": "2026-01-15T00:00:00+0000",
    "period_end": "2026-02-15T00:00:00+0000"
  }'
```

**Ответ:** `{}` (пустой объект)

### Запрос По Заказу

```bash
curl -X POST "https://api.cdek.ru/v2/photoDocument" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "orders": [
      {"order_uuid": "300f5b78-f432-4bda-bd20-740f9348b075"}
    ]
  }'
```

**Ответ:** `{}` (пустой объект)

### Пустой Запрос

```bash
curl -X POST "https://api.cdek.ru/v2/photoDocument" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{}'
```

**Ответ:**
```json
{
  "errors": [{
    "code": "v2_field_is_empty",
    "message": "[orders] is empty"
  }]
}
```

### Несуществующий Заказ

```bash
curl -X POST "https://api.cdek.ru/v2/photoDocument" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "orders": [
      {"cdek_number": 99999999999}
    ]
  }'
```

**Ответ:**
```json
{
  "errors": [{
    "code": "v2_orders_number_is_empty",
    "message": "Entity is empty. All orders are incorrect"
  }]
}
```

**Вывод:** API работает корректно, но фотоуслуга не подключена.

---

## 📋 Из Официальной Документации

**OpenAPI Specification (api/cdek-api.yaml):**

### Endpoint: POST /v2/photoDocument (строка 1419)

> "Для корректной работы метода, для договора должна быть **подключена фотоуслуга**,
> а также **настроен фотопроект**."

### Услуга PHOTO_OF_DOCUMENTS

**Параметр услуги:**
> "5. **Код фотопроекта** для услуги PHOTO_OF_DOCUMENTS (добавление услуги доступно **только при создании заказа**)"

**Пример добавления услуги:**
```json
{
  "services": [
    {
      "code": "PHOTO_OF_DOCUMENTS",
      "parameter": "КОД_ФОТОПРОЕКТА"
    }
  ]
}
```

---

## ✅ Как Подключить Фотоуслугу

### Шаг 1: Обращение в Техподдержку

**Email:** integrator@cdek.ru

**Шаблон письма:**
```
Тема: Подключение услуги "Фото документов" для договора ИМ1222128

Здравствуйте!

Прошу подключить услугу "Фото документов" для договора:
- Номер договора: ИМ1222128
- Компания: МЕТРИКА
- Email: sa@tmont.ru

Требуется для интеграции с API СДЭК.

С уважением,
[Ваше имя]
```

### Шаг 2: Настройка Фотопроекта

После подключения услуги:
1. Зайти в личный кабинет СДЭК
2. Раздел "Фотоуслуги"
3. Создать фотопроект
4. Получить **код фотопроекта** (например: `PHOTO_PROJECT_001`)

### Шаг 3: Создание Заказа

```go
order := &cdek.OrderCreateRequestDto{
    Type:        2,              // Тип "доставка"
    TariffCode:  342,            // Дверь-дверь

    FromLocation: &cdek.LocationDto{
        Code:    44,
        Address: "ул. Тестовая, д. 10, кв. 5",
    },

    ToLocation: &cdek.LocationDto{
        Code:    137,
        Address: "Невский проспект, д. 20, кв. 15",
    },

    Sender: &cdek.SenderDto{
        Name:  "Иванов Иван Иванович",
        // Для физлица НЕ указывать company
        Phones: []cdek.Phone{{Number: "+79991234567"}},
    },

    Recipient: &cdek.RecipientContactDto{
        Name: "Петров Петр Петрович",
        Phones: []cdek.Phone{{Number: "+79997654321"}},
    },

    Services: []cdek.ServiceDto{
        {
            Code:      "INSURANCE",
            Parameter: "2500",
        },
        {
            Code:      "PHOTO_OF_DOCUMENTS",
            Parameter: "PHOTO_PROJECT_001", // Код из ЛК
        },
    },

    Packages: []cdek.PackageRequestDto{{
        Comment: "Тестовая посылка",
        Weight:  1000,
        // ...
    }},
}
```

### Шаг 4: Получение Фото

После **вручения заказа** получателю:

```go
photoRequest := &cdek.PhotoRequestDto{
    Orders: []cdek.OrderPhoto{
        {
            OrderUuid: "uuid-заказа",
        },
    },
}

resp, _ := client.GetPhotoDocuments(ctx, photoRequest)

for _, order := range resp.Orders {
    fmt.Printf("Photo URL: %s\n", order.PhotoUrl)
    fmt.Printf("Expires: %s\n", order.PhotoExpiry)
}
```

---

## 🎯 Все Доступные Режимы Доставки

| Код | Название |
|-----|----------|
| 1 | дверь-дверь |
| 2 | дверь-склад |
| 3 | склад-дверь |
| 4 | склад-склад |
| 5 | терминал-терминал |
| 6 | дверь-постамат |
| 7 | склад-постамат |
| 8 | постамат-дверь |
| 9 | постамат-склад |
| 10 | постамат-постамат |

---

## 📞 Контакты

- **Техподдержка:** integrator@cdek.ru
- **Договор:** ИМ1222128
- **Компания:** МЕТРИКА
- **Email:** sa@tmont.ru

---

## 📚 Источники

1. [CDEK API Documentation](https://apidoc.cdek.ru/)
2. [CDEK Integration Portal](https://www.cdek.ru/ru/integration/api/)
3. Ответ техподдержки СДЭК (2026-02-15)
4. Тестирование на production API

---

**Статус:** Фотоуслуга не подключена для договора ИМ1222128
**Требуется:** Обращение в integrator@cdek.ru для подключения
