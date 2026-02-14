package cdek

import (
	"context"
)

// Service - высокоуровневый сервис для работы с CDEK API (PascalCase по регламенту)
type Service struct {
	client *AuthenticatedClient

	// Компоненты (camelCase по регламенту 4.6)
	costCalculator *costCalculator
	orderValidator *orderValidator
	parser         *responseParser
}

// NewService создает новый высокоуровневый сервис
func NewService(client *AuthenticatedClient) *Service {
	return &Service{
		client:         client,
		costCalculator: newCostCalculator(client),
		orderValidator: newOrderValidator(),
		parser:         newResponseParser(),
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

// Методы для работы с заказами, расчетами и т.д. будут добавлены
// после исправления дубликатов в OpenAPI спецификации
