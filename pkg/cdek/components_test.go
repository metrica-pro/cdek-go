package cdek

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestTokenCache(t *testing.T) {
	cache := newTokenCache()

	// Initially empty
	token, ok := cache.get()
	if ok {
		t.Error("Expected empty cache")
	}
	if token != "" {
		t.Error("Expected empty token")
	}

	// Set token
	cache.set("test-token", 3600)
	token, ok = cache.get()
	if !ok {
		t.Error("Expected cached token")
	}
	if token != "test-token" {
		t.Errorf("Got token %v, want test-token", token)
	}

	// Expired token
	cache.set("expired", -100) // Already expired
	time.Sleep(10 * time.Millisecond)
	_, ok = cache.get()
	if ok {
		t.Error("Expected expired token to return false")
	}

	// Clear cache
	cache.set("to-clear", 3600)
	cache.clear()
	_, ok = cache.get()
	if ok {
		t.Error("Expected cleared cache")
	}
}

func TestRequestBuilder(t *testing.T) {
	ctx := context.Background()

	t.Run("simple GET request", func(t *testing.T) {
		builder := newRequestBuilder("https://api.example.com")
		req, err := builder.build(ctx, "GET", "/test", nil)
		if err != nil {
			t.Fatalf("build() error = %v", err)
		}
		if req.Method != "GET" {
			t.Errorf("Method = %v, want GET", req.Method)
		}
		if req.URL.String() != "https://api.example.com/test" {
			t.Errorf("URL = %v", req.URL.String())
		}
	})

	t.Run("with authorization", func(t *testing.T) {
		builder := newRequestBuilder("https://api.example.com")
		req, err := builder.withAuthorization("test-token").build(ctx, "GET", "/auth", nil)
		if err != nil {
			t.Fatalf("build() error = %v", err)
		}
		if req.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization header = %v", req.Header.Get("Authorization"))
		}
	})

	t.Run("with content type", func(t *testing.T) {
		builder := newRequestBuilder("https://api.example.com")
		req, err := builder.withContentType("application/xml").build(ctx, "POST", "/xml", nil)
		if err != nil {
			t.Fatalf("build() error = %v", err)
		}
		if req.Header.Get("Content-Type") != "application/xml" {
			t.Errorf("Content-Type header = %v", req.Header.Get("Content-Type"))
		}
	})

	t.Run("with JSON body", func(t *testing.T) {
		builder := newRequestBuilder("https://api.example.com")
		body := map[string]string{"key": "value"}
		req, err := builder.build(ctx, "POST", "/json", body)
		if err != nil {
			t.Fatalf("build() error = %v", err)
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type header = %v (should auto-set)", req.Header.Get("Content-Type"))
		}
		if req.Body == nil {
			t.Error("Expected non-nil body")
		}
	})
}

func TestResponseParser(t *testing.T) {
	parser := newResponseParser()

	t.Run("parse success JSON", func(t *testing.T) {
		body := `{"status":"ok","value":42}`
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}

		var result map[string]interface{}
		err := parser.parse(resp, &result)
		if err != nil {
			t.Fatalf("parse() error = %v", err)
		}

		if result["status"] != "ok" {
			t.Errorf("status = %v, want ok", result["status"])
		}
	})

	t.Run("parse nil response value", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 204,
			Body:       io.NopCloser(bytes.NewBufferString("")),
		}

		err := parser.parse(resp, nil)
		if err != nil {
			t.Errorf("parse() with nil value should not error: %v", err)
		}
	})

	t.Run("parse error response", func(t *testing.T) {
		body := `{"errors":[{"code":"test_error","message":"test message"}]}`
		resp := &http.Response{
			StatusCode: 400,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Header:     http.Header{},
		}

		err := parser.parse(resp, nil)
		if err == nil {
			t.Error("Expected error for 400 status")
		}
	})

	t.Run("parseBytes success", func(t *testing.T) {
		expectedData := []byte{0x50, 0x44, 0x46} // PDF header
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(expectedData)),
		}

		data, err := parser.parseBytes(resp)
		if err != nil {
			t.Fatalf("parseBytes() error = %v", err)
		}

		if !bytes.Equal(data, expectedData) {
			t.Errorf("parseBytes() = %v, want %v", data, expectedData)
		}
	})

	t.Run("parseBytes error response", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(bytes.NewBufferString(`{"errors":[{"code":"not_found"}]}`)),
			Header:     http.Header{},
		}

		_, err := parser.parseBytes(resp)
		if err == nil {
			t.Error("Expected error for 404 status")
		}
	})
}

func TestOrderValidator(t *testing.T) {
	validator := newOrderValidator()

	t.Run("validate nil", func(t *testing.T) {
		err := validator.validate(nil)
		if err == nil {
			t.Error("Expected error for nil data")
		}
	})

	t.Run("validate non-nil", func(t *testing.T) {
		err := validator.validate(map[string]string{"test": "data"})
		if err != nil {
			t.Errorf("validate() error = %v", err)
		}
	})
}

func TestCostCalculator(t *testing.T) {
	// Создаем mock client (без реального API)
	config := &AccountConfig{
		Name:         "test",
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		BaseURL:      "https://api.example.com",
		Timeout:      5 * time.Second,
		MaxRetries:   1,
	}

	client, err := NewAuthenticatedClient(config)
	if err != nil {
		t.Fatalf("NewAuthenticatedClient() error = %v", err)
	}

	calculator := newCostCalculator(client)

	t.Run("validate with nil request", func(t *testing.T) {
		err := calculator.validate(nil)
		if err == nil {
			t.Error("Expected error for nil request")
		}
	})

	t.Run("validate with valid request", func(t *testing.T) {
		req := &CostRequest{
			FromCityCode: 44,  // Москва
			ToCityCode:   270, // Новосибирск
			Packages: []Package{
				{Weight: 1000, Length: 20, Width: 15, Height: 10},
			},
		}
		err := calculator.validate(req)
		if err != nil {
			t.Errorf("Expected no error for valid request, got: %v", err)
		}
	})

	t.Run("validate with empty packages", func(t *testing.T) {
		req := &CostRequest{
			FromCityCode: 44,
			ToCityCode:   270,
			Packages:     []Package{},
		}
		err := calculator.validate(req)
		if err == nil {
			t.Error("Expected error for empty packages")
		}
	})

	t.Run("validate with zero weight", func(t *testing.T) {
		req := &CostRequest{
			FromCityCode: 44,
			ToCityCode:   270,
			Packages: []Package{
				{Weight: 0, Length: 20, Width: 15, Height: 10},
			},
		}
		err := calculator.validate(req)
		if err == nil {
			t.Error("Expected error for zero weight")
		}
	})
}
