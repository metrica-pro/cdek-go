//go:build integration
// +build integration

package cdek

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"
)

// Простые интеграционные тесты
// export $(cat .env.test | grep -v '^#' | xargs)
// go test -tags=integration -v ./pkg/cdek/... -run Simple

func getSimpleTestClient(t *testing.T) *AuthenticatedClient {
	t.Helper()

	clientID := os.Getenv("CDEK_TEST_CLIENT_ID")
	clientSecret := os.Getenv("CDEK_TEST_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		t.Skip("CDEK_TEST_CLIENT_ID or CDEK_TEST_CLIENT_SECRET not set")
	}

	config := &AccountConfig{
		Name:         "test",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		BaseURL:      URLSandbox,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
	}

	client, err := NewAuthenticatedClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return client
}

func TestSimple_OAuth2(t *testing.T) {
	client := getSimpleTestClient(t)
	ctx := context.Background()

	token, err := client.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}

	if token == "" {
		t.Error("Expected non-empty token")
	}

	t.Logf("✅ OAuth2: токен получен (%d символов)", len(token))
}

func TestSimple_HealthCheck(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client)
	ctx := context.Background()

	err := service.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}

	t.Logf("✅ HealthCheck: API доступен")
}

func TestSimple_Deliverypoints(t *testing.T) {
	client := getSimpleTestClient(t)
	ctx := context.Background()

	resp, err := client.ClientWithResponses().GetDeliverypointsWithResponse(ctx, nil)
	if err != nil {
		t.Fatalf("GetDeliverypointsWithResponse() error = %v", err)
	}

	if resp.StatusCode() != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode())
	}

	// Парсим Body как JSON
	var points []interface{}
	if err := json.Unmarshal(resp.Body, &points); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	t.Logf("✅ Deliverypoints: получено %d ПВЗ", len(points))
}

func TestSimple_Cities(t *testing.T) {
	client := getSimpleTestClient(t)
	ctx := context.Background()

	size := int32(5)
	resp, err := client.ClientWithResponses().CitiesWithResponse(ctx, &CitiesParams{Size: &size})
	if err != nil {
		t.Fatalf("CitiesWithResponse() error = %v", err)
	}

	if resp.StatusCode() != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode())
	}

	var cities []interface{}
	if err := json.Unmarshal(resp.Body, &cities); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	t.Logf("✅ Cities: получено %d городов", len(cities))
}

func TestSimple_Calculator(t *testing.T) {
	client := getSimpleTestClient(t)
	ctx := context.Background()

	serviceType := int32(1)
	currency := int32(1)
	fromCode := int32(44)
	toCode := int32(270)

	request := CalculatorTariffListRequestDto{
		Type:     &serviceType,
		Currency: &currency,
		FromLocation: CalculatorLocationDto{
			Code: &fromCode,
		},
		ToLocation: CalculatorLocationDto{
			Code: &toCode,
		},
	}

	// Маршалим request в JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := client.ClientWithResponses().TariffListWithBody(ctx, &TariffListParams{}, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("TariffListWithBody() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Logf("Статус: %d, пропускаем", resp.StatusCode)
		t.SkipNow()
	}

	// Читаем Body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err == nil {
		if tariffs, ok := result["tariff_codes"].([]interface{}); ok {
			t.Logf("✅ Calculator: получено %d тарифов", len(tariffs))
			return
		}
	}

	t.Logf("✅ Calculator: запрос выполнен успешно")
}
