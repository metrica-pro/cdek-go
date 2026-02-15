package cdek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// tokenCache - компонент кеширования OAuth2 токенов (camelCase по регламенту 4.6)
type tokenCache struct {
	mu          sync.RWMutex
	accessToken string
	expiresAt   time.Time
}

// newTokenCache создает новый кеш токенов
func newTokenCache() *tokenCache {
	return &tokenCache{}
}

// get возвращает закешированный токен если он валидный
func (tc *tokenCache) get() (string, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	// Проверяем с запасом в 1 минуту
	if tc.accessToken != "" && time.Now().Add(time.Minute).Before(tc.expiresAt) {
		return tc.accessToken, true
	}
	return "", false
}

// set сохраняет токен в кеш
func (tc *tokenCache) set(token string, expiresIn int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.accessToken = token
	tc.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
}

// clear очищает кеш
//
//nolint:unused // Будет использовано в service layer
func (tc *tokenCache) clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.accessToken = ""
	tc.expiresAt = time.Time{}
}

// requestBuilder - компонент построения HTTP запросов (camelCase по регламенту 4.6)
type requestBuilder struct {
	baseURL string
	headers map[string]string
}

// newRequestBuilder создает новый builder запросов
func newRequestBuilder(baseURL string) *requestBuilder {
	return &requestBuilder{
		baseURL: baseURL,
		headers: make(map[string]string),
	}
}

// withAuthorization добавляет токен авторизации
func (rb *requestBuilder) withAuthorization(token string) *requestBuilder {
	rb.headers["Authorization"] = "Bearer " + token
	return rb
}

// withContentType устанавливает Content-Type
//
//nolint:unused // Будет использовано в service layer
func (rb *requestBuilder) withContentType(contentType string) *requestBuilder {
	rb.headers["Content-Type"] = contentType
	return rb
}

// build создает HTTP запрос
func (rb *requestBuilder) build(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	var reqBody io.Reader

	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, rb.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range rb.headers {
		req.Header.Set(k, v)
	}

	// Устанавливаем Content-Type по умолчанию если не установлен
	if req.Header.Get("Content-Type") == "" && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// responseParser - компонент парсинга API ответов (camelCase по регламенту 4.6)
type responseParser struct{}

// newResponseParser создает новый parser ответов
func newResponseParser() *responseParser {
	return &responseParser{}
}

// parse парсит HTTP ответ в структуру
func (rp *responseParser) parse(resp *http.Response, v interface{}) error {
	defer func() {
		_ = resp.Body.Close() // Игнорируем ошибку закрытия
	}()

	// Проверяем статус код
	if resp.StatusCode >= 400 {
		return wrapHTTPError(resp)
	}

	// Если не нужно парсить, просто возвращаем nil
	if v == nil {
		return nil
	}

	// Парсим JSON
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		// Если ошибка парсинга, читаем body для отладки
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to decode response: %w (body: %s)", err, string(body))
	}

	return nil
}

// parseBytes читает ответ как байты (для PDF, изображений)
//
//nolint:unused // Будет использовано в service layer для получения PDF/изображений
func (rp *responseParser) parseBytes(resp *http.Response) ([]byte, error) {
	defer func() {
		_ = resp.Body.Close() // Игнорируем ошибку закрытия
	}()

	if resp.StatusCode >= 400 {
		return nil, wrapHTTPError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// orderValidator - компонент валидации заказов (camelCase по регламенту 4.6)
type orderValidator struct{}

// newOrderValidator создает новый validator заказов
func newOrderValidator() *orderValidator {
	return &orderValidator{}
}

// validate проверяет корректность данных (generic validation)
//
//nolint:unused // Будет использовано в service layer
func (ov *orderValidator) validate(data interface{}) error {
	if data == nil {
		return fmt.Errorf("data: %w", ErrInvalidRequest)
	}
	// Детальная валидация будет в service layer
	return nil
}

// costCalculator - компонент расчета стоимости (camelCase по регламенту 4.6)
type costCalculator struct {
	client *AuthenticatedClient
	parser *responseParser
}

// newCostCalculator создает новый calculator стоимости
func newCostCalculator(client *AuthenticatedClient) *costCalculator {
	return &costCalculator{
		client: client,
		parser: newResponseParser(),
	}
}

// validate проверяет корректность CostRequest
func (cc *costCalculator) validate(req *CostRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil: %w", ErrInvalidRequest)
	}

	if req.FromCityCode <= 0 {
		return fmt.Errorf("from_city_code must be positive: %w", ErrInvalidRequest)
	}

	if req.ToCityCode <= 0 {
		return fmt.Errorf("to_city_code must be positive: %w", ErrInvalidRequest)
	}

	if len(req.Packages) == 0 {
		return fmt.Errorf("packages cannot be empty: %w", ErrInvalidRequest)
	}

	for i, pkg := range req.Packages {
		if pkg.Weight <= 0 {
			return fmt.Errorf("packages[%d].weight must be positive: %w", i, ErrInvalidRequest)
		}
	}

	return nil
}
