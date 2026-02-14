# Тестирование CDEK Go Client

## Unit тесты

Запуск всех unit тестов:

```bash
go test -v ./pkg/cdek/...
```

С race detector:

```bash
go test -race ./pkg/cdek/...
```

С покрытием:

```bash
go test -coverprofile=coverage.out ./pkg/cdek/...
go tool cover -html=coverage.out
```

**Текущее покрытие:** 93.6% для domain кода

---

## Интеграционные тесты

Интеграционные тесты проверяют реальное взаимодействие с CDEK Sandbox API.

### Подготовка

1. Получите тестовые credentials на [CDEK Sandbox](https://api.edu.cdek.ru/)

2. Создайте файл `.env.test`:

```bash
cp .env.test.example .env.test
```

3. Заполните credentials:

```env
CDEK_TEST_CLIENT_ID=ваш-тестовый-client-id
CDEK_TEST_CLIENT_SECRET=ваш-тестовый-client-secret
```

4. Загрузите переменные окружения:

```bash
source .env.test
# или
export $(cat .env.test | xargs)
```

### Запуск интеграционных тестов

```bash
# Запуск всех интеграционных тестов
go test -tags=integration -v ./pkg/cdek/...

# Запуск конкретного теста
go test -tags=integration -v ./pkg/cdek/... -run TestIntegration_OAuth2

# С подробным выводом
go test -tags=integration -v ./pkg/cdek/... -count=1
```

### Покрываемые эндпоинты

Интеграционные тесты проверяют следующие ключевые эндпоинты:

#### 1. OAuth2 (`TestIntegration_OAuth2`)
- ✅ Получение access token
- ✅ Кеширование токенов
- ✅ Автоматическое обновление

#### 2. Health Check (`TestIntegration_HealthCheck`)
- ✅ Проверка доступности CDEK API
- ✅ Валидация credentials

#### 3. Delivery Points (`TestIntegration_DeliveryPoints`)
- ✅ Получение списка ПВЗ по городу
- ✅ Фильтрация ПВЗ (с примерочной, оплата картой и т.д.)
- ✅ Проверка структуры данных

#### 4. Location Cities (`TestIntegration_LocationCities`)
- ✅ Получение списка городов
- ✅ Поиск города по названию
- ✅ Проверка структуры данных

#### 5. Calculator (`TestIntegration_Calculator`)
- ✅ Расчет стоимости по одному тарифу
- ✅ Расчет стоимости по всем доступным тарифам
- ✅ Проверка срока доставки

#### 6. Multi-Account Manager (`TestIntegration_Manager_MultiAccount`)
- ✅ Параллельная проверка нескольких аккаунтов
- ✅ Thread-safe операции

---

## Примеры вывода

### Unit тесты

```
=== RUN   TestConfig_Validate
--- PASS: TestConfig_Validate (0.00s)
=== RUN   TestAuthenticatedClient_GetToken
--- PASS: TestAuthenticatedClient_GetToken (0.00s)
=== RUN   TestManager_HealthCheck
--- PASS: TestManager_HealthCheck (0.00s)
...
PASS
ok      github.com/metrica-pro/cdek-go/pkg/cdek 2.270s
coverage: 93.6% of statements
```

### Интеграционные тесты

```
=== RUN   TestIntegration_OAuth2
=== RUN   TestIntegration_OAuth2/get_valid_token
    integration_test.go:55: ✅ Получен токен длиной: 1024 символов
=== RUN   TestIntegration_OAuth2/token_caching_works
    integration_test.go:71: ✅ Кеширование токенов работает корректно
--- PASS: TestIntegration_OAuth2 (0.25s)

=== RUN   TestIntegration_DeliveryPoints
=== RUN   TestIntegration_DeliveryPoints/get_delivery_points_list
    integration_test.go:103: ✅ Получено 523 ПВЗ в Москве
    integration_test.go:115: Пример ПВЗ: код=MSK123, адрес=ул. Ленина, д. 1
--- PASS: TestIntegration_DeliveryPoints (0.48s)

=== RUN   TestIntegration_Calculator
=== RUN   TestIntegration_Calculator/calculate_tariff
    integration_test.go:256: ✅ Расчет тарифа выполнен:
    integration_test.go:257: Стоимость доставки: 345.50 руб
    integration_test.go:258: Срок доставки: 3-5 дней
--- PASS: TestIntegration_Calculator (0.32s)

PASS
ok      github.com/metrica-pro/cdek-go/pkg/cdek 1.150s
```

---

## CI/CD

### GitHub Actions

Интеграционные тесты НЕ запускаются автоматически в CI, т.к. требуют credentials.

Unit тесты запускаются на каждый push:

```yaml
# .github/workflows/ci.yml
- name: Run tests
  run: go test -v -race -coverprofile=coverage.out ./...
```

Для запуска интеграционных тестов в CI нужно добавить secrets:

```yaml
- name: Run integration tests
  env:
    CDEK_TEST_CLIENT_ID: ${{ secrets.CDEK_TEST_CLIENT_ID }}
    CDEK_TEST_CLIENT_SECRET: ${{ secrets.CDEK_TEST_CLIENT_SECRET }}
  run: go test -tags=integration -v ./pkg/cdek/...
```

---

## Troubleshooting

### Ошибка "credentials not set"

```
--- SKIP: TestIntegration_OAuth2 (0.00s)
    integration_test.go:28: Skipping integration test: credentials not set
```

**Решение:** Установите переменные окружения `CDEK_TEST_CLIENT_ID` и `CDEK_TEST_CLIENT_SECRET`

### Ошибка "unauthorized"

```
GetToken() error = cdek: unauthorized: invalid_client
```

**Решение:** Проверьте правильность client_id и client_secret. Убедитесь, что используете Sandbox credentials.

### Ошибка "timeout"

```
context deadline exceeded
```

**Решение:** Увеличьте timeout в конфигурации или проверьте доступность CDEK API:

```bash
curl -I https://api.edu.cdek.ru/v2/oauth/token
```

---

## Рекомендации

1. **Всегда запускайте unit тесты** перед коммитом:
   ```bash
   go test -race ./...
   ```

2. **Запускайте интеграционные тесты** перед релизом:
   ```bash
   go test -tags=integration -v ./...
   ```

3. **Проверяйте покрытие** регулярно:
   ```bash
   go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
   ```

4. **Используйте Sandbox API** для тестов, никогда production.

5. **Не коммитьте credentials** - добавьте `.env.test` в `.gitignore`.

---

## Дополнительная информация

- [CDEK API Documentation](https://apidoc.cdek.ru/)
- [CDEK Sandbox](https://api.edu.cdek.ru/)
- [README.md](README.md) - основная документация
- [ARCHITECTURE.md](ARCHITECTURE.md) - архитектура проекта
