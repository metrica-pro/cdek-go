package cdek

import (
	"context"
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	config := &AccountConfig{
		Name:         "test",
		ClientID:     "id",
		ClientSecret: "secret",
		BaseURL:      URLProduction,
	}

	client, err := NewAuthenticatedClient(config)
	if err != nil {
		t.Fatalf("NewAuthenticatedClient() error = %v", err)
	}

	service := NewService(client)

	if service == nil {
		t.Fatal("NewService() returned nil")
	}
	if service.client != client {
		t.Error("Service client doesn't match")
	}
	if service.costCalculator == nil {
		t.Error("Cost calculator not initialized")
	}
	if service.orderValidator == nil {
		t.Error("Order validator not initialized")
	}
	if service.parser == nil {
		t.Error("Parser not initialized")
	}
}

func TestService_GetClient(t *testing.T) {
	config := &AccountConfig{
		Name:         "test",
		ClientID:     "id",
		ClientSecret: "secret",
		BaseURL:      URLProduction,
	}

	authClient, _ := NewAuthenticatedClient(config)
	service := NewService(authClient)

	client := service.GetClient()
	if client == nil {
		t.Error("GetClient() returned nil")
	}
	if client != authClient {
		t.Error("GetClient() returned different client")
	}
}

func TestService_HealthCheck(t *testing.T) {
	t.Run("fails for invalid credentials", func(t *testing.T) {
		config := &AccountConfig{
			Name:         "test",
			ClientID:     "invalid",
			ClientSecret: "invalid",
			BaseURL:      URLProduction,
			Timeout:      2 * time.Second,
		}

		client, _ := NewAuthenticatedClient(config)
		service := NewService(client)

		ctx := context.Background()
		err := service.HealthCheck(ctx)

		// Ожидаем ошибку т.к. credentials невалидны
		if err == nil {
			t.Error("HealthCheck() should return error for invalid credentials")
		}
	})

	t.Run("respects context", func(t *testing.T) {
		config := &AccountConfig{
			Name:         "test",
			ClientID:     "id",
			ClientSecret: "secret",
			BaseURL:      "http://localhost:9999", // Unreachable
			Timeout:      100 * time.Millisecond,
		}

		client, _ := NewAuthenticatedClient(config)
		service := NewService(client)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := service.HealthCheck(ctx)
		if err == nil {
			t.Error("HealthCheck() should return error for timeout")
		}
	})
}
