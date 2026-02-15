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
