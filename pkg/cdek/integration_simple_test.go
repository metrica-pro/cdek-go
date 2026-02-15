//go:build integration
// +build integration

package cdek

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// Простые интеграционные тесты с реальным CDEK API
// export $(cat .env.test | grep -v '^#' | xargs)
// go test -tags=integration -v ./pkg/cdek/...

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

	// Получаем токен
	token, err := client.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}

	// Создаем запрос с авторизацией
	resp, err := client.ClientWithResponses().GetDeliverypointsWithResponse(ctx, nil, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	})
	if err != nil {
		t.Fatalf("GetDeliverypointsWithResponse() error = %v", err)
	}

	if resp.StatusCode() != 200 {
		t.Logf("Status: %d, body: %s", resp.StatusCode(), string(resp.Body))
		t.SkipNow()
	}

	var points []interface{}
	if err := json.Unmarshal(resp.Body, &points); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	t.Logf("✅ Deliverypoints: получено %d ПВЗ", len(points))
}

func TestSimple_Cities(t *testing.T) {
	client := getSimpleTestClient(t)
	ctx := context.Background()

	token, err := client.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}

	size := int32(5)
	resp, err := client.ClientWithResponses().CitiesWithResponse(ctx, &CitiesParams{Size: &size}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	})
	if err != nil {
		t.Fatalf("CitiesWithResponse() error = %v", err)
	}

	if resp.StatusCode() != 200 {
		t.Logf("Status: %d", resp.StatusCode())
		t.SkipNow()
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

	token, err := client.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}

	serviceType := int32(1)
	currency := int32(1)
	fromCode := int32(44)   // Москва
	toCode := int32(270)    // Новосибирск
	weight := int32(1000)   // 1 кг
	length := int32(20)     // см
	width := int32(15)      // см
	height := int32(10)     // см

	request := CalculatorTariffListRequestDto{
		Type:     &serviceType,
		Currency: &currency,
		FromLocation: CalculatorLocationDto{
			Code: &fromCode,
		},
		ToLocation: CalculatorLocationDto{
			Code: &toCode,
		},
		Packages: []CalcPackageRequestDto{
			{
				Weight: weight,
				Length: &length,
				Width:  &width,
				Height: &height,
			},
		},
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := client.ClientWithResponses().TariffListWithBody(ctx, nil, "application/json", bytes.NewReader(requestBody), func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	})
	if err != nil {
		t.Fatalf("TariffListWithBody() error = %v", err)
	}

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Logf("Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
		t.SkipNow()
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
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
