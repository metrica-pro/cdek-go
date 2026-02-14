package cdek

import (
	"context"
	"testing"
	"time"
)

func TestNewAuthenticatedClient(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &AccountConfig{
			Name:         "test",
			ClientID:     "test-id",
			ClientSecret: "test-secret",
			BaseURL:      URLProduction,
		}

		client, err := NewAuthenticatedClient(config)
		if err != nil {
			t.Fatalf("NewAuthenticatedClient() error = %v", err)
		}
		if client == nil {
			t.Fatal("NewAuthenticatedClient() returned nil")
		}
		if client.config != config {
			t.Error("Client config doesn't match")
		}
	})

	t.Run("sets default timeout", func(t *testing.T) {
		config := &AccountConfig{
			Name:         "test",
			ClientID:     "id",
			ClientSecret: "secret",
			BaseURL:      URLProduction,
		}

		client, _ := NewAuthenticatedClient(config)
		if config.Timeout != 30*time.Second {
			t.Errorf("Default timeout = %v, want 30s", config.Timeout)
		}
		if client.httpClient.Timeout != 30*time.Second {
			t.Errorf("HTTP client timeout = %v, want 30s", client.httpClient.Timeout)
		}
	})

	t.Run("sets default max retries", func(t *testing.T) {
		config := &AccountConfig{
			Name:         "test",
			ClientID:     "id",
			ClientSecret: "secret",
			BaseURL:      URLProduction,
		}

		_, _ = NewAuthenticatedClient(config)
		if config.MaxRetries != 3 {
			t.Errorf("Default max_retries = %v, want 3", config.MaxRetries)
		}
	})

	t.Run("respects custom timeout", func(t *testing.T) {
		config := &AccountConfig{
			Name:         "test",
			ClientID:     "id",
			ClientSecret: "secret",
			BaseURL:      URLProduction,
			Timeout:      5 * time.Second,
		}

		client, _ := NewAuthenticatedClient(config)
		if client.httpClient.Timeout != 5*time.Second {
			t.Errorf("Custom timeout not respected: %v", client.httpClient.Timeout)
		}
	})

	t.Run("respects custom max retries", func(t *testing.T) {
		config := &AccountConfig{
			Name:         "test",
			ClientID:     "id",
			ClientSecret: "secret",
			BaseURL:      URLProduction,
			MaxRetries:   5,
		}

		_, _ = NewAuthenticatedClient(config)
		if config.MaxRetries != 5 {
			t.Errorf("Custom max_retries not respected: %v", config.MaxRetries)
		}
	})

	t.Run("initializes components", func(t *testing.T) {
		config := &AccountConfig{
			Name:         "test",
			ClientID:     "id",
			ClientSecret: "secret",
			BaseURL:      URLProduction,
		}

		client, _ := NewAuthenticatedClient(config)
		if client.cache == nil {
			t.Error("Token cache not initialized")
		}
		if client.parser == nil {
			t.Error("Response parser not initialized")
		}
		if client.builder == nil {
			t.Error("Request builder not initialized")
		}
	})
}

func TestAuthenticatedClient_Client(t *testing.T) {
	config := &AccountConfig{
		Name:         "test",
		ClientID:     "id",
		ClientSecret: "secret",
		BaseURL:      URLProduction,
	}

	authClient, _ := NewAuthenticatedClient(config)
	client := authClient.Client()

	if client == nil {
		t.Error("Client() returned nil")
	}
	if client != authClient.client {
		t.Error("Client() returned different client")
	}
}

func TestAuthenticatedClient_GetToken(t *testing.T) {
	// Note: This test will fail without a real API or mock server
	// For now we're just testing the error path

	t.Run("returns error for invalid credentials", func(t *testing.T) {
		config := &AccountConfig{
			Name:         "test",
			ClientID:     "invalid",
			ClientSecret: "invalid",
			BaseURL:      URLProduction,
			Timeout:      2 * time.Second,
		}

		client, _ := NewAuthenticatedClient(config)
		ctx := context.Background()

		_, err := client.GetToken(ctx)
		// Ожидаем ошибку т.к. credentials невалидны
		if err == nil {
			t.Error("GetToken() should return error for invalid credentials")
		}
	})

	t.Run("respects context timeout", func(t *testing.T) {
		config := &AccountConfig{
			Name:         "test",
			ClientID:     "id",
			ClientSecret: "secret",
			BaseURL:      "http://localhost:9999", // Unreachable
			Timeout:      100 * time.Millisecond,
		}

		client, _ := NewAuthenticatedClient(config)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.GetToken(ctx)
		if err == nil {
			t.Error("GetToken() should return error for context timeout")
		}
	})
}

func TestAuthenticatedClient_Do(t *testing.T) {
	config := &AccountConfig{
		Name:         "test",
		ClientID:     "id",
		ClientSecret: "secret",
		BaseURL:      URLProduction,
		Timeout:      2 * time.Second,
	}

	client, _ := NewAuthenticatedClient(config)

	t.Run("returns error when token fails", func(t *testing.T) {
		ctx := context.Background()

		// Do() попытается получить токен, что не удастся с fake credentials
		_, err := client.Do(ctx, "GET", "/test", nil)
		if err == nil {
			t.Error("Do() should return error when GetToken fails")
		}
	})
}

func TestAuthenticatedClient_DoWithResponse(t *testing.T) {
	config := &AccountConfig{
		Name:         "test",
		ClientID:     "id",
		ClientSecret: "secret",
		BaseURL:      URLProduction,
		Timeout:      2 * time.Second,
	}

	client, _ := NewAuthenticatedClient(config)

	t.Run("returns error when Do fails", func(t *testing.T) {
		ctx := context.Background()
		var result map[string]interface{}

		err := client.DoWithResponse(ctx, "GET", "/test", nil, &result)
		if err == nil {
			t.Error("DoWithResponse() should return error when Do fails")
		}
	})
}
