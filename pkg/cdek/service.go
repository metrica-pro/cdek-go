package cdek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/sony/gobreaker/v2"
)

// ServiceConfig - конфигурация для высокоуровневого сервиса
type ServiceConfig struct {
	BreakerName        string        // "cdek-api"
	BreakerMaxRequests uint32        // 5 (half-open state)
	BreakerInterval    time.Duration // 30s (reset interval)
	BreakerTimeout     time.Duration // 60s (open → half-open timeout)
	Logger             *zerolog.Logger
}

// DefaultServiceConfig возвращает конфигурацию по умолчанию
func DefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		BreakerName:        "cdek-api",
		BreakerMaxRequests: 5,
		BreakerInterval:    30 * time.Second,
		BreakerTimeout:     60 * time.Second,
		Logger:             nil, // no-op logger по умолчанию
	}
}

// Service - высокоуровневый сервис для работы с CDEK API (PascalCase по регламенту)
type Service struct {
	client  *AuthenticatedClient
	breaker *gobreaker.CircuitBreaker[any]
	logger  zerolog.Logger

	// Компоненты (camelCase по регламенту 4.6)
	costCalculator *costCalculator
	orderValidator *orderValidator
	parser         *responseParser
	mapper         *dtoMapper
}

// NewService создает новый высокоуровневый сервис
func NewService(client *AuthenticatedClient, config *ServiceConfig) *Service {
	if config == nil {
		config = DefaultServiceConfig()
	}

	// Создание Circuit Breaker с настройками из конфигурации
	breaker := gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        config.BreakerName,
		MaxRequests: config.BreakerMaxRequests,
		Interval:    config.BreakerInterval,
		Timeout:     config.BreakerTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Открываем circuit если >= 60% запросов падают и минимум 3 запроса
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
	})

	// Инициализация logger (no-op если не передан)
	logger := zerolog.Nop()
	if config.Logger != nil {
		logger = *config.Logger
	}

	return &Service{
		client:         client,
		breaker:        breaker,
		logger:         logger,
		costCalculator: newCostCalculator(client),
		orderValidator: newOrderValidator(),
		parser:         newResponseParser(),
		mapper:         newDtoMapper(),
	}
}

// GetClient возвращает базовый аутентифицированный клиент для прямого доступа к API
func (s *Service) GetClient() *AuthenticatedClient {
	return s.client
}

// HealthCheck проверяет доступность API
func (s *Service) HealthCheck(ctx context.Context) error {
	_, err := s.client.GetToken(ctx)
	return err
}

// ========================
// High-Level Service Methods
// ========================

// CalculateCost рассчитывает стоимость доставки по указанному маршруту
func (s *Service) CalculateCost(ctx context.Context, req *CostRequest) (*CostResponse, error) {
	s.logger.Info().
		Int32("from_city", req.FromCityCode).
		Int32("to_city", req.ToCityCode).
		Int("packages", len(req.Packages)).
		Msg("calculating delivery cost")

	// Выполнение через Circuit Breaker для защиты от каскадных сбоев
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Валидация запроса
		if err := s.costCalculator.validate(req); err != nil {
			return nil, fmt.Errorf("validation: %w", err)
		}

		// Преобразование Service Request → CDEK DTO
		cdekReq := s.mapper.toCDEKCalculatorRequest(req)

		// Получение токена авторизации
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Маршалинг запроса в JSON
		requestBody, err := json.Marshal(cdekReq)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}

		// Вызов автогенерированного API метода
		resp, err := s.client.ClientWithResponses().TariffListWithBody(
			ctx,
			nil, // query params
			"application/json",
			bytes.NewReader(requestBody),
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Проверка HTTP статуса
		if resp.StatusCode >= 400 {
			return nil, wrapHTTPError(resp)
		}

		// Чтение тела ответа
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		// Преобразование CDEK Response → Service Response
		return s.mapper.fromCDEKCalculatorResponse(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Msg("calculate cost failed")
		return nil, err
	}

	costResp := result.(*CostResponse)
	s.logger.Info().
		Int("tariffs_count", len(costResp.Tariffs)).
		Msg("calculate cost success")

	return costResp, nil
}

// CreateOrder создает заказ на доставку в CDEK
func (s *Service) CreateOrder(ctx context.Context, req *OrderRequest) (*OrderResponse, error) {
	s.logger.Info().
		Str("type", req.Type).
		Int("tariff_code", req.TariffCode).
		Int("packages", len(req.Packages)).
		Msg("creating order")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Валидация заказа
		if err := s.orderValidator.validate(req); err != nil {
			return nil, fmt.Errorf("validation: %w", err)
		}

		// Преобразование Service Request → CDEK map
		orderMap, err := s.mapper.toCDEKOrderRequest(req)
		if err != nil {
			return nil, fmt.Errorf("map request: %w", err)
		}

		// Получение токена авторизации
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Маршалинг в JSON
		requestBody, err := json.Marshal(orderMap)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().CreateOrderWithBody(
			ctx,
			nil, // query params
			"application/json",
			bytes.NewReader(requestBody),
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение тела ответа
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		// Проверка HTTP статуса
		if resp.StatusCode >= 400 {
			// Пересоздаем response для wrapHTTPError с body
			httpResp := &http.Response{
				StatusCode: resp.StatusCode,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Преобразование CDEK Response → Service Response
		return s.mapper.fromCDEKOrderResponse(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Msg("create order failed")
		return nil, err
	}

	orderResp := result.(*OrderResponse)
	s.logger.Info().
		Str("uuid", orderResp.UUID).
		Msg("create order success")

	return orderResp, nil
}

// TrackOrder получает информацию об отслеживании заказа
func (s *Service) TrackOrder(ctx context.Context, orderUUID string) (*TrackingInfo, error) {
	s.logger.Info().
		Str("uuid", orderUUID).
		Msg("tracking order")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().GetOrder(
			ctx,
			orderUUID,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Проверка HTTP статуса
		if resp.StatusCode >= 400 {
			return nil, wrapHTTPError(resp)
		}

		// Чтение ответа
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		// Преобразование в TrackingInfo
		return s.mapper.fromCDEKOrderToTracking(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Str("uuid", orderUUID).Msg("track order failed")
		return nil, err
	}

	trackingInfo := result.(*TrackingInfo)
	s.logger.Info().
		Str("uuid", trackingInfo.UUID).
		Str("current_status", trackingInfo.CurrentStatus.Name).
		Msg("track order success")

	return trackingInfo, nil
}

// ListDeliveryPoints возвращает список пунктов выдачи заказов
func (s *Service) ListDeliveryPoints(ctx context.Context, req *DeliveryPointsRequest) ([]DeliveryPoint, error) {
	s.logger.Info().
		Str("city_code", req.CityCode).
		Str("type", req.Type).
		Msg("listing delivery points")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Строим path с query parameters
		path := "/v2/deliverypoints"
		queryParts := []string{}

		if req.CityCode != "" {
			queryParts = append(queryParts, fmt.Sprintf("city_code=%s", req.CityCode))
		}
		if req.Type != "" {
			queryParts = append(queryParts, fmt.Sprintf("type=%s", req.Type))
		}

		if len(queryParts) > 0 {
			path += "?" + queryParts[0]
			for i := 1; i < len(queryParts); i++ {
				path += "&" + queryParts[i]
			}
		}

		// Используем AuthenticatedClient.Do для автоматической авторизации
		httpResp, err := s.client.Do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}
		defer func() { _ = httpResp.Body.Close() }()

		// Проверка HTTP статуса
		if httpResp.StatusCode >= 400 {
			return nil, wrapHTTPError(httpResp)
		}

		// Чтение ответа
		bodyBytes, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		// Преобразование в []DeliveryPoint
		return s.mapper.fromCDEKDeliveryPoints(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Msg("list delivery points failed")
		return nil, err
	}

	points := result.([]DeliveryPoint)
	s.logger.Info().
		Int("points_count", len(points)).
		Msg("list delivery points success")

	return points, nil
}

// PrintBarcode создает задание на печать этикеток (штрихкодов)
func (s *Service) PrintBarcode(ctx context.Context, req *PrintBarcodeRequest) (*PrintResponse, error) {
	s.logger.Info().
		Int("orders_count", len(req.Orders)).
		Msg("creating barcode print job")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Формируем request body
		requestMap := make(map[string]interface{})

		orders := make([]map[string]interface{}, len(req.Orders))
		for i, order := range req.Orders {
			orders[i] = map[string]interface{}{
				"order_uuid": order.OrderUUID,
			}
		}
		requestMap["orders"] = orders

		if req.Copy != nil {
			requestMap["copy_count"] = *req.Copy
		}
		if req.Format != nil {
			requestMap["format"] = *req.Format
		}

		requestBody, err := json.Marshal(requestMap)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().BarcodePrintWithBodyWithResponse(
			ctx,
			"application/json",
			bytes.NewReader(requestBody),
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Парсим ответ
		var printResp map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &printResp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}

		response := &PrintResponse{}
		if entity, ok := printResp["entity"].(map[string]interface{}); ok {
			if uuid, ok := entity["uuid"].(string); ok {
				response.UUID = uuid
			}
			if url, ok := entity["url"].(string); ok {
				response.URL = url
			}
		}

		return response, nil
	})

	if err != nil {
		s.logger.Error().Err(err).Msg("create barcode print job failed")
		return nil, err
	}

	printResp := result.(*PrintResponse)
	s.logger.Info().
		Str("uuid", printResp.UUID).
		Msg("barcode print job created")

	return printResp, nil
}

// PrintWaybill создает задание на печать накладных
func (s *Service) PrintWaybill(ctx context.Context, req *PrintWaybillRequest) (*PrintResponse, error) {
	s.logger.Info().
		Int("orders_count", len(req.Orders)).
		Msg("creating waybill print job")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Формируем request body
		requestMap := make(map[string]interface{})

		orders := make([]map[string]interface{}, len(req.Orders))
		for i, order := range req.Orders {
			orders[i] = map[string]interface{}{
				"order_uuid": order.OrderUUID,
			}
		}
		requestMap["orders"] = orders

		if req.Copy != nil {
			requestMap["copy_count"] = *req.Copy
		}
		if req.Format != nil {
			requestMap["format"] = *req.Format
		}

		requestBody, err := json.Marshal(requestMap)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().WaybillPrintWithBodyWithResponse(
			ctx,
			"application/json",
			bytes.NewReader(requestBody),
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Парсим ответ
		var printResp map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &printResp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}

		response := &PrintResponse{}
		if entity, ok := printResp["entity"].(map[string]interface{}); ok {
			if uuid, ok := entity["uuid"].(string); ok {
				response.UUID = uuid
			}
			if url, ok := entity["url"].(string); ok {
				response.URL = url
			}
		}

		return response, nil
	})

	if err != nil {
		s.logger.Error().Err(err).Msg("create waybill print job failed")
		return nil, err
	}

	printResp := result.(*PrintResponse)
	s.logger.Info().
		Str("uuid", printResp.UUID).
		Msg("waybill print job created")

	return printResp, nil
}

// GetOrder получает полную информацию о заказе (включая все детали)
func (s *Service) GetOrder(ctx context.Context, orderUUID string) (*OrderInfo, error) {
	s.logger.Info().
		Str("uuid", orderUUID).
		Msg("getting order info")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().GetOrderWithResponse(
			ctx,
			orderUUID,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Преобразование CDEK Response → OrderInfo
		return s.mapper.fromCDEKOrderToInfo(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Str("uuid", orderUUID).Msg("get order failed")
		return nil, err
	}

	orderInfo := result.(*OrderInfo)
	s.logger.Info().
		Str("uuid", orderInfo.UUID).
		Msg("get order success")

	return orderInfo, nil
}

// UpdateOrder обновляет существующий заказ
func (s *Service) UpdateOrder(ctx context.Context, req *UpdateOrderRequest) (*OrderResponse, error) {
	s.logger.Info().
		Str("uuid", req.OrderUUID).
		Msg("updating order")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Преобразование Service Request → CDEK map (используем тот же маппер что и для Create)
		updateMap, err := s.mapper.toCDEKUpdateOrderRequest(req)
		if err != nil {
			return nil, fmt.Errorf("map request: %w", err)
		}

		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Маршалинг в JSON
		requestBody, err := json.Marshal(updateMap)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}

		// Вызов API (PATCH /orders)
		resp, err := s.client.ClientWithResponses().UpdateWithBodyWithResponse(
			ctx,
			nil, // params (только developer-key, опционально)
			"application/json",
			bytes.NewReader(requestBody),
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Преобразование CDEK Response → OrderResponse
		return s.mapper.fromCDEKOrderResponse(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Str("uuid", req.OrderUUID).Msg("update order failed")
		return nil, err
	}

	orderResp := result.(*OrderResponse)
	s.logger.Info().
		Str("uuid", orderResp.UUID).
		Msg("update order success")

	return orderResp, nil
}

// CancelOrder отменяет заказ
func (s *Service) CancelOrder(ctx context.Context, orderUUID string) error {
	s.logger.Info().
		Str("uuid", orderUUID).
		Msg("canceling order")

	// Выполнение через Circuit Breaker
	_, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().DeleteWithResponse(
			ctx,
			orderUUID,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		return nil, nil
	})

	if err != nil {
		s.logger.Error().Err(err).Str("uuid", orderUUID).Msg("cancel order failed")
		return err
	}

	s.logger.Info().
		Str("uuid", orderUUID).
		Msg("cancel order success")

	return nil
}

// DownloadBarcode скачивает готовый PDF с этикетками
func (s *Service) DownloadBarcode(ctx context.Context, printUUID string) ([]byte, error) {
	s.logger.Info().
		Str("print_uuid", printUUID).
		Msg("downloading barcode PDF")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().BarcodeDownloadWithResponse(
			ctx,
			printUUID,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение PDF
		pdfBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(pdfBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		return pdfBytes, nil
	})

	if err != nil {
		s.logger.Error().Err(err).Str("print_uuid", printUUID).Msg("download barcode failed")
		return nil, err
	}

	pdfBytes := result.([]byte)
	s.logger.Info().
		Str("print_uuid", printUUID).
		Int("size_bytes", len(pdfBytes)).
		Msg("download barcode success")

	return pdfBytes, nil
}

// DownloadWaybill скачивает готовый PDF с накладной
func (s *Service) DownloadWaybill(ctx context.Context, printUUID string) ([]byte, error) {
	s.logger.Info().
		Str("print_uuid", printUUID).
		Msg("downloading waybill PDF")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().WaybillDownloadWithResponse(
			ctx,
			printUUID,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение PDF
		pdfBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(pdfBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		return pdfBytes, nil
	})

	if err != nil {
		s.logger.Error().Err(err).Str("print_uuid", printUUID).Msg("download waybill failed")
		return nil, err
	}

	pdfBytes := result.([]byte)
	s.logger.Info().
		Str("print_uuid", printUUID).
		Int("size_bytes", len(pdfBytes)).
		Msg("download waybill success")

	return pdfBytes, nil
}

// ========================
// Location Reference (Cities/Regions)
// ========================

// ListCities возвращает список городов из справочника СДЭК
func (s *Service) ListCities(ctx context.Context, req *CitiesRequest) ([]City, error) {
	s.logger.Info().Msg("listing cities")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Формируем query параметры
		params := &CitiesParams{}
		if req.CountryCode != nil {
			params.CountryCodes = req.CountryCode
		}
		if req.RegionCode != nil {
			regionCode := int32(*req.RegionCode)
			params.RegionCode = &regionCode
		}
		if req.PostalCode != nil {
			params.PostalCode = req.PostalCode
		}
		if req.Code != nil {
			code := int32(*req.Code)
			params.Code = &code
		}
		if req.City != nil {
			params.City = req.City
		}
		if req.Size != nil {
			size := int32(*req.Size)
			params.Size = &size
		}
		if req.Page != nil {
			page := int32(*req.Page)
			params.Page = &page
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().CitiesWithResponse(
			ctx,
			params,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Преобразование CDEK Response → []City
		return s.mapper.fromCDEKCities(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Msg("list cities failed")
		return nil, err
	}

	cities := result.([]City)
	s.logger.Info().
		Int("count", len(cities)).
		Msg("list cities success")

	return cities, nil
}

// ListRegions возвращает список регионов из справочника СДЭК
func (s *Service) ListRegions(ctx context.Context, req *RegionsRequest) ([]Region, error) {
	s.logger.Info().Msg("listing regions")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Формируем query параметры
		params := &RegionsParams{}
		if req.CountryCode != nil {
			params.CountryCodes = req.CountryCode
		}
		if req.Size != nil {
			size := int32(*req.Size)
			params.Size = &size
		}
		if req.Page != nil {
			page := int32(*req.Page)
			params.Page = &page
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().RegionsWithResponse(
			ctx,
			params,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Преобразование CDEK Response → []Region
		return s.mapper.fromCDEKRegions(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Msg("list regions failed")
		return nil, err
	}

	regions := result.([]Region)
	s.logger.Info().
		Int("count", len(regions)).
		Msg("list regions success")

	return regions, nil
}

// ========================
// Intakes (Заявки на забор)
// ========================

// CreateIntake создает заявку на забор груза
func (s *Service) CreateIntake(ctx context.Context, req *IntakeRequest) (*IntakeResponse, error) {
	s.logger.Info().
		Str("intake_date", req.IntakeDate).
		Msg("creating intake")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Преобразование Service Request → CDEK map
		intakeMap, err := s.mapper.toCDEKIntakeRequest(req)
		if err != nil {
			return nil, fmt.Errorf("map request: %w", err)
		}

		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Маршалинг в JSON
		requestBody, err := json.Marshal(intakeMap)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().CreateIntakeWithBodyWithResponse(
			ctx,
			"application/json",
			bytes.NewReader(requestBody),
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Преобразование CDEK Response → IntakeResponse
		return s.mapper.fromCDEKIntakeResponse(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Msg("create intake failed")
		return nil, err
	}

	intakeResp := result.(*IntakeResponse)
	s.logger.Info().
		Str("uuid", intakeResp.UUID).
		Msg("create intake success")

	return intakeResp, nil
}

// GetIntake получает информацию о заявке на забор
func (s *Service) GetIntake(ctx context.Context, intakeUUID string) (*IntakeInfo, error) {
	s.logger.Info().
		Str("uuid", intakeUUID).
		Msg("getting intake info")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().GetByUuidWithResponse(
			ctx,
			intakeUUID,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Преобразование CDEK Response → IntakeInfo
		return s.mapper.fromCDEKIntakeInfo(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Str("uuid", intakeUUID).Msg("get intake failed")
		return nil, err
	}

	intakeInfo := result.(*IntakeInfo)
	s.logger.Info().
		Str("uuid", intakeInfo.UUID).
		Msg("get intake success")

	return intakeInfo, nil
}

// DeleteIntake отменяет заявку на забор
func (s *Service) DeleteIntake(ctx context.Context, intakeUUID string) error {
	s.logger.Info().
		Str("uuid", intakeUUID).
		Msg("deleting intake")

	// Выполнение через Circuit Breaker
	_, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().DeleteByUuidWithResponse(
			ctx,
			intakeUUID,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		return nil, nil
	})

	if err != nil {
		s.logger.Error().Err(err).Str("uuid", intakeUUID).Msg("delete intake failed")
		return err
	}

	s.logger.Info().
		Str("uuid", intakeUUID).
		Msg("delete intake success")

	return nil
}

// ========================
// Webhooks
// ========================

// CreateWebhook регистрирует webhook для получения уведомлений
func (s *Service) CreateWebhook(ctx context.Context, req *WebhookRequest) (*WebhookResponse, error) {
	s.logger.Info().
		Str("url", req.URL).
		Str("type", req.Type).
		Msg("creating webhook")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Преобразование Service Request → CDEK map
		webhookMap := map[string]interface{}{
			"url":  req.URL,
			"type": req.Type,
		}

		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Маршалинг в JSON
		requestBody, err := json.Marshal(webhookMap)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().CreateV2WebhooksWithBodyWithResponse(
			ctx,
			"application/json",
			bytes.NewReader(requestBody),
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Преобразование CDEK Response → WebhookResponse
		return s.mapper.fromCDEKWebhookResponse(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Msg("create webhook failed")
		return nil, err
	}

	webhookResp := result.(*WebhookResponse)
	s.logger.Info().
		Str("uuid", webhookResp.UUID).
		Msg("create webhook success")

	return webhookResp, nil
}

// ListWebhooks возвращает список зарегистрированных webhooks
func (s *Service) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	s.logger.Info().Msg("listing webhooks")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().GetAllWithResponse(
			ctx,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Преобразование CDEK Response → []Webhook
		return s.mapper.fromCDEKWebhooks(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Msg("list webhooks failed")
		return nil, err
	}

	webhooks := result.([]Webhook)
	s.logger.Info().
		Int("count", len(webhooks)).
		Msg("list webhooks success")

	return webhooks, nil
}

// GetWebhook получает информацию о конкретном webhook
func (s *Service) GetWebhook(ctx context.Context, webhookUUID string) (*Webhook, error) {
	s.logger.Info().
		Str("uuid", webhookUUID).
		Msg("getting webhook")

	// Выполнение через Circuit Breaker
	result, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().GetByIdWithResponse(
			ctx,
			webhookUUID,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		// Преобразование CDEK Response → Webhook
		return s.mapper.fromCDEKWebhook(bodyBytes)
	})

	if err != nil {
		s.logger.Error().Err(err).Str("uuid", webhookUUID).Msg("get webhook failed")
		return nil, err
	}

	webhook := result.(*Webhook)
	s.logger.Info().
		Str("uuid", webhook.UUID).
		Str("type", webhook.Type).
		Msg("get webhook success")

	return webhook, nil
}

// DeleteWebhook удаляет webhook
func (s *Service) DeleteWebhook(ctx context.Context, webhookUUID string) error {
	s.logger.Info().
		Str("uuid", webhookUUID).
		Msg("deleting webhook")

	// Выполнение через Circuit Breaker
	_, err := s.breaker.Execute(func() (interface{}, error) {
		// Получение токена
		token, err := s.client.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().DeleteByIdWithResponse(
			ctx,
			webhookUUID,
			func(ctx context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		if err != nil {
			return nil, fmt.Errorf("api call: %w", err)
		}

		// Чтение ответа
		bodyBytes := resp.Body

		// Проверка HTTP статуса
		if resp.StatusCode() >= 400 {
			httpResp := &http.Response{
				StatusCode: resp.StatusCode(),
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}
			return nil, wrapHTTPError(httpResp)
		}

		return nil, nil
	})

	if err != nil {
		s.logger.Error().Err(err).Str("uuid", webhookUUID).Msg("delete webhook failed")
		return err
	}

	s.logger.Info().
		Str("uuid", webhookUUID).
		Msg("delete webhook success")

	return nil
}
