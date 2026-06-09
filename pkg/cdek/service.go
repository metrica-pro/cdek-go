package cdek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/gobreaker/v2"
)

// ServiceConfig - конфигурация для высокоуровневого сервиса
type ServiceConfig struct {
	BreakerName        string        // "cdek-api"
	BreakerMaxRequests uint32        // 5 (half-open state)
	BreakerInterval    time.Duration // 30s (reset interval)
	BreakerTimeout     time.Duration // 60s (open → half-open timeout)
	Logger             *slog.Logger
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
	logger  *slog.Logger

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

	// Инициализация logger (discard если не передан)
	logger := slog.New(slog.DiscardHandler)
	if config.Logger != nil {
		logger = config.Logger
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
	s.logger.Info("calculating delivery cost", "from_city", req.FromCityCode, "to_city", req.ToCityCode, "packages", len(req.Packages))

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("calculate cost failed", "err", err)
		return nil, err
	}

	costResp := result.(*CostResponse)
	s.logger.Info("calculate cost success", "tariffs_count", len(costResp.Tariffs))

	return costResp, nil
}

// CreateOrder создает заказ на доставку в CDEK
func (s *Service) CreateOrder(ctx context.Context, req *OrderRequest) (*OrderResponse, error) {
	s.logger.Info("creating order", "type", req.Type, "tariff_code", req.TariffCode, "packages", len(req.Packages))

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("create order failed", "err", err)
		return nil, err
	}

	orderResp := result.(*OrderResponse)
	s.logger.Info("create order success", "uuid", orderResp.UUID)

	return orderResp, nil
}

// TrackOrder получает информацию об отслеживании заказа
func (s *Service) TrackOrder(ctx context.Context, orderUUID string) (*TrackingInfo, error) {
	s.logger.Info("tracking order", "uuid", orderUUID)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("track order failed", "err", err, "uuid", orderUUID)
		return nil, err
	}

	trackingInfo := result.(*TrackingInfo)
	s.logger.Info("track order success", "uuid", trackingInfo.UUID, "current_status", trackingInfo.CurrentStatus.Name)

	return trackingInfo, nil
}

// ListDeliveryPoints возвращает список пунктов выдачи заказов
func (s *Service) ListDeliveryPoints(ctx context.Context, req *DeliveryPointsRequest) ([]DeliveryPoint, error) {
	s.logger.Info("listing delivery points", "city_code", req.CityCode, "type", req.Type)

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
		s.logger.Error("list delivery points failed", "err", err)
		return nil, err
	}

	points := result.([]DeliveryPoint)
	s.logger.Info("list delivery points success", "points_count", len(points))

	return points, nil
}

// PrintBarcode создает задание на печать этикеток (штрихкодов)
func (s *Service) PrintBarcode(ctx context.Context, req *PrintBarcodeRequest) (*PrintResponse, error) {
	s.logger.Info("creating barcode print job", "orders_count", len(req.Orders))

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("create barcode print job failed", "err", err)
		return nil, err
	}

	printResp := result.(*PrintResponse)
	s.logger.Info("barcode print job created", "uuid", printResp.UUID)

	return printResp, nil
}

// PrintWaybill создает задание на печать накладных
func (s *Service) PrintWaybill(ctx context.Context, req *PrintWaybillRequest) (*PrintResponse, error) {
	s.logger.Info("creating waybill print job", "orders_count", len(req.Orders))

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("create waybill print job failed", "err", err)
		return nil, err
	}

	printResp := result.(*PrintResponse)
	s.logger.Info("waybill print job created", "uuid", printResp.UUID)

	return printResp, nil
}

// GetOrder получает полную информацию о заказе (включая все детали)
func (s *Service) GetOrder(ctx context.Context, orderUUID string) (*OrderInfo, error) {
	s.logger.Info("getting order info", "uuid", orderUUID)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("get order failed", "err", err, "uuid", orderUUID)
		return nil, err
	}

	orderInfo := result.(*OrderInfo)
	s.logger.Info("get order success", "uuid", orderInfo.UUID)

	return orderInfo, nil
}

// UpdateOrder обновляет существующий заказ
func (s *Service) UpdateOrder(ctx context.Context, req *UpdateOrderRequest) (*OrderResponse, error) {
	s.logger.Info("updating order", "uuid", req.OrderUUID)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("update order failed", "err", err, "uuid", req.OrderUUID)
		return nil, err
	}

	orderResp := result.(*OrderResponse)
	s.logger.Info("update order success", "uuid", orderResp.UUID)

	return orderResp, nil
}

// CancelOrder отменяет заказ
func (s *Service) CancelOrder(ctx context.Context, orderUUID string) error {
	s.logger.Info("canceling order", "uuid", orderUUID)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("cancel order failed", "err", err, "uuid", orderUUID)
		return err
	}

	s.logger.Info("cancel order success", "uuid", orderUUID)

	return nil
}

// DownloadBarcode скачивает готовый PDF с этикетками
func (s *Service) DownloadBarcode(ctx context.Context, printUUID string) ([]byte, error) {
	s.logger.Info("downloading barcode PDF", "print_uuid", printUUID)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("download barcode failed", "err", err, "print_uuid", printUUID)
		return nil, err
	}

	pdfBytes := result.([]byte)
	s.logger.Info("download barcode success", "print_uuid", printUUID, "size_bytes", len(pdfBytes))

	return pdfBytes, nil
}

// DownloadWaybill скачивает готовый PDF с накладной
func (s *Service) DownloadWaybill(ctx context.Context, printUUID string) ([]byte, error) {
	s.logger.Info("downloading waybill PDF", "print_uuid", printUUID)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("download waybill failed", "err", err, "print_uuid", printUUID)
		return nil, err
	}

	pdfBytes := result.([]byte)
	s.logger.Info("download waybill success", "print_uuid", printUUID, "size_bytes", len(pdfBytes))

	return pdfBytes, nil
}

// ========================
// Location Reference (Cities/Regions)
// ========================

// ListCities возвращает список городов из справочника СДЭК
func (s *Service) ListCities(ctx context.Context, req *CitiesRequest) ([]City, error) {
	s.logger.Info("listing cities")

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
			regionCode := int32(*req.RegionCode) //nolint:gosec
			params.RegionCode = &regionCode
		}
		if req.PostalCode != nil {
			params.PostalCode = req.PostalCode
		}
		if req.Code != nil {
			code := int32(*req.Code) //nolint:gosec
			params.Code = &code
		}
		if req.City != nil {
			params.City = req.City
		}
		if req.Size != nil {
			size := int32(*req.Size) //nolint:gosec
			params.Size = &size
		}
		if req.Page != nil {
			page := int32(*req.Page) //nolint:gosec
			params.Page = &page
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().CitiesWithResponse(
			ctx,
			params,
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("list cities failed", "err", err)
		return nil, err
	}

	cities := result.([]City)
	s.logger.Info("list cities success", "count", len(cities))

	return cities, nil
}

// ListRegions возвращает список регионов из справочника СДЭК
func (s *Service) ListRegions(ctx context.Context, req *RegionsRequest) ([]Region, error) {
	s.logger.Info("listing regions")

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
			size := int32(*req.Size) //nolint:gosec
			params.Size = &size
		}
		if req.Page != nil {
			page := int32(*req.Page) //nolint:gosec
			params.Page = &page
		}

		// Вызов API
		resp, err := s.client.ClientWithResponses().RegionsWithResponse(
			ctx,
			params,
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("list regions failed", "err", err)
		return nil, err
	}

	regions := result.([]Region)
	s.logger.Info("list regions success", "count", len(regions))

	return regions, nil
}

// ========================
// Intakes (Заявки на забор)
// ========================

// CreateIntake создает заявку на забор груза
func (s *Service) CreateIntake(ctx context.Context, req *IntakeRequest) (*IntakeResponse, error) {
	s.logger.Info("creating intake", "intake_date", req.IntakeDate)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("create intake failed", "err", err)
		return nil, err
	}

	intakeResp := result.(*IntakeResponse)
	s.logger.Info("create intake success", "uuid", intakeResp.UUID)

	return intakeResp, nil
}

// GetIntake получает информацию о заявке на забор
func (s *Service) GetIntake(ctx context.Context, intakeUUID string) (*IntakeInfo, error) {
	s.logger.Info("getting intake info", "uuid", intakeUUID)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("get intake failed", "err", err, "uuid", intakeUUID)
		return nil, err
	}

	intakeInfo := result.(*IntakeInfo)
	s.logger.Info("get intake success", "uuid", intakeInfo.UUID)

	return intakeInfo, nil
}

// DeleteIntake отменяет заявку на забор
func (s *Service) DeleteIntake(ctx context.Context, intakeUUID string) error {
	s.logger.Info("deleting intake", "uuid", intakeUUID)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("delete intake failed", "err", err, "uuid", intakeUUID)
		return err
	}

	s.logger.Info("delete intake success", "uuid", intakeUUID)

	return nil
}

// ========================
// Webhooks
// ========================

// CreateWebhook регистрирует webhook для получения уведомлений
func (s *Service) CreateWebhook(ctx context.Context, req *WebhookRequest) (*WebhookResponse, error) {
	s.logger.Info("creating webhook", "url", req.URL, "type", req.Type)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("create webhook failed", "err", err)
		return nil, err
	}

	webhookResp := result.(*WebhookResponse)
	s.logger.Info("create webhook success", "uuid", webhookResp.UUID)

	return webhookResp, nil
}

// ListWebhooks возвращает список зарегистрированных webhooks
func (s *Service) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	s.logger.Info("listing webhooks")

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("list webhooks failed", "err", err)
		return nil, err
	}

	webhooks := result.([]Webhook)
	s.logger.Info("list webhooks success", "count", len(webhooks))

	return webhooks, nil
}

// GetWebhook получает информацию о конкретном webhook
func (s *Service) GetWebhook(ctx context.Context, webhookUUID string) (*Webhook, error) {
	s.logger.Info("getting webhook", "uuid", webhookUUID)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("get webhook failed", "err", err, "uuid", webhookUUID)
		return nil, err
	}

	webhook := result.(*Webhook)
	s.logger.Info("get webhook success", "uuid", webhook.UUID, "type", webhook.Type)

	return webhook, nil
}

// DeleteWebhook удаляет webhook
func (s *Service) DeleteWebhook(ctx context.Context, webhookUUID string) error {
	s.logger.Info("deleting webhook", "uuid", webhookUUID)

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
			func(_ context.Context, r *http.Request) error {
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
		s.logger.Error("delete webhook failed", "err", err, "uuid", webhookUUID)
		return err
	}

	s.logger.Info("delete webhook success", "uuid", webhookUUID)

	return nil
}
