package cdek

// edge_cases_test.go — добивающие случаи вспомогательных компонентов:
// ветки валидатора, ошибки requestBuilder, декод-ошибки responseParser,
// нестандартные статусы wrapHTTPError, ошибки конструкторов, int32-границы.

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ========================
// orderValidator: недостающие ветки
// ========================

func TestOrderValidatorTable(t *testing.T) {
	validator := newOrderValidator()

	mutate := map[string]func(req *OrderRequest){
		"пустой type":           func(r *OrderRequest) { r.Type = "" },
		"нулевый tariff_code":   func(r *OrderRequest) { r.TariffCode = 0 },
		"нет телефонов":         func(r *OrderRequest) { r.Recipient.Phones = nil },
		"пустой номер упаковки": func(r *OrderRequest) { r.Packages[0].Number = "" },
		"нулевой вес упаковки":  func(r *OrderRequest) { r.Packages[0].Weight = 0 },
		"пустое имя товара":     func(r *OrderRequest) { r.Packages[0].Items[0].Name = "" },
		"пустой ware_key":       func(r *OrderRequest) { r.Packages[0].Items[0].WareKey = "" },
		"нулевое количество":    func(r *OrderRequest) { r.Packages[0].Items[0].Amount = 0 },
		"нет items у ИМ":        func(r *OrderRequest) { r.Packages[0].Items = nil },
	}

	for name, mutate := range mutate {
		t.Run(name, func(t *testing.T) {
			req := baseOrderRequest()
			mutate(req)
			if err := validator.validate(req); err == nil {
				t.Errorf("ожидалась ошибка валидации для %q", name)
			}
		})
	}
}

// ========================
// requestBuilder
// ========================

func TestRequestBuilderErrors(t *testing.T) {
	rb := newRequestBuilder("http://localhost")

	t.Run("немаршализуемое тело (NaN)", func(t *testing.T) {
		_, err := rb.build(context.Background(), http.MethodPost, "/x",
			map[string]interface{}{"value": math.NaN()})
		if err == nil || !strings.Contains(err.Error(), "marshal") {
			t.Errorf("error = %v, ожидалась ошибка marshal", err)
		}
	})

	t.Run("некорректный метод", func(t *testing.T) {
		_, err := rb.build(context.Background(), "GET / HTTP/1.1", "/x", nil)
		if err == nil || !strings.Contains(err.Error(), "create request") {
			t.Errorf("error = %v, ожидалась ошибка create request", err)
		}
	})
}

// ========================
// responseParser
// ========================

func TestResponseParserDecodeError(t *testing.T) {
	parser := newResponseParser()

	t.Run("битый JSON в 200", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser("not-json"),
		}
		var out map[string]interface{}
		if err := parser.parse(resp, &out); err == nil {
			t.Error("ожидалась ошибка декодирования")
		}
	})

	t.Run("v == nil: только статус-проверка", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{}`),
		}
		if err := parser.parse(resp, nil); err != nil {
			t.Errorf("parse(nil) error = %v", err)
		}
	})
}

func TestResponseParserBytesErrors(t *testing.T) {
	parser := newResponseParser()

	t.Run("4xx", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusNotFound, Body: ioNopCloser(`{"errors":[]}`)}
		if _, err := parser.parseBytes(resp); err == nil {
			t.Error("ожидалась ошибка 404")
		}
	})

	t.Run("успех", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusOK, Body: ioNopCloser(`pdf-bytes`)}
		got, err := parser.parseBytes(resp)
		if err != nil || string(got) != "pdf-bytes" {
			t.Errorf("parseBytes() = %q, %v", string(got), err)
		}
	})
}

// ioNopCloser — строковое тело ответа
func ioNopCloser(s string) ioNopReadCloser { return ioNopReadCloser{strings.NewReader(s)} }

type ioNopReadCloser struct{ r *strings.Reader }

func (c ioNopReadCloser) Read(p []byte) (int, error) { return c.r.Read(p) }
func (c ioNopReadCloser) Close() error               { return nil }

// ========================
// errors: нестандартные статусы и склейка запросов
// ========================

func TestWrapHTTPErrorUnknownStatus(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTeapot, // 418 — вне switch
		Body:       ioNopCloser(`{"errors":[{"code":"teapot","message":"I'm a teapot"}]}`),
	}
	err := wrapHTTPError(resp)
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	for _, sentinel := range []error{ErrInvalidRequest, ErrUnauthorized, ErrNotFound, ErrServerError, ErrRateLimitExceeded} {
		if errorsIs(err, sentinel) {
			t.Errorf("ошибка %v не должна нести сентинел %v", err, sentinel)
		}
	}
	if !strings.Contains(err.Error(), "teapot") {
		t.Errorf("error = %v, ожидался текст ошибки API", err)
	}
}

// errorsIs — локальный алиас для читаемости
func errorsIs(err, target error) bool {
	return err != nil && target != nil && strings.Contains(err.Error(), target.Error())
}

func TestErrorResponseRequestsMerge(t *testing.T) {
	errResp := &ErrorResponse{
		Errors:   []ErrorDetail{{Code: "top", Message: "верхнеуровневая"}},
		Requests: []RequestState{{State: "INVALID", Errors: []ErrorDetail{{Code: "nested", Message: "из requests"}}}},
	}
	msg := errResp.Error()
	if !strings.Contains(msg, "2 errors") {
		t.Errorf("Error() = %q, ожидались сведенные ошибки из errors+requests", msg)
	}
}

func TestParseErrorResponseBrokenBody(t *testing.T) {
	// Тело — невалидный JSON: ошибка отдается как текст
	resp := &http.Response{StatusCode: http.StatusBadRequest, Body: ioNopCloser("plain text")}
	err := parseErrorResponse(resp)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %v, ожидался http 400 c текстом", err)
	}

	// Валидный JSON без errors/requests: unknown error
	resp = &http.Response{StatusCode: http.StatusBadRequest, Body: ioNopCloser(`{}`)}
	err = parseErrorResponse(resp)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error = %v, ожидался unknown error", err)
	}
}

// ========================
// Конструкторы
// ========================

func TestNewServiceWithLogger(t *testing.T) {
	logger := slog.Default()
	service := NewService(nil, &ServiceConfig{Logger: logger})
	if service.logger != logger {
		t.Error("переданный logger не использован")
	}
}

func TestAuthenticatedClientDoWithResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, `{"access_token":"t","token_type":"Bearer","expires_in":3600}`)
	}))
	t.Cleanup(server.Close)

	config := &AccountConfig{
		Name: "test", ClientID: "id", ClientSecret: "secret",
		BaseURL: server.URL, Timeout: 5 * time.Second,
	}
	client, err := NewAuthenticatedClient(config)
	if err != nil {
		t.Fatalf("NewAuthenticatedClient() error = %v", err)
	}

	t.Run("успех с разбором", func(t *testing.T) {
		var out TokenResponse
		if err := client.DoWithResponse(context.Background(), http.MethodGet, "/v2/oauth/token", nil, &out); err != nil {
			t.Fatalf("DoWithResponse() error = %v", err)
		}
		if out.AccessToken != "t" {
			t.Errorf("AccessToken = %q", out.AccessToken)
		}
	})

	t.Run("некорректный метод → ошибка построения запроса", func(t *testing.T) {
		var out TokenResponse
		err := client.DoWithResponse(context.Background(), "GET / HTTP/1.1", "/x", nil, &out)
		if err == nil || !strings.Contains(err.Error(), "build request") {
			t.Errorf("error = %v, ожидалась ошибка build request", err)
		}
	})

	t.Run("HTTP 500 → ошибка", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			respondError(w, http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)
		cfg := &AccountConfig{
			Name: "test", ClientID: "id", ClientSecret: "secret",
			BaseURL: server.URL, Timeout: 5 * time.Second,
		}
		client, err := NewAuthenticatedClient(cfg)
		if err != nil {
			t.Fatalf("NewAuthenticatedClient() error = %v", err)
		}
		var out TokenResponse
		if err := client.DoWithResponse(context.Background(), http.MethodGet, "/x", nil, &out); err == nil {
			t.Error("ожидалась ошибка 500")
		}
	})
}

// ========================
// int32-границы разбора заказов
// ========================

func TestFromCDEKOrderToInfoInt32Overflow(t *testing.T) {
	m := newDtoMapper()

	// weight вне диапазона int32 → 0 (конверсия с проверкой границ)
	info, err := m.fromCDEKOrderToInfo([]byte(`{"entity":{
		"uuid": "u1",
		"packages": [{
			"number": "1",
			"weight": 9999999999,
			"length": 9999999999,
			"items": [{"name": "n", "ware_key": "w", "weight": 9999999999, "amount": 9999999999}]
		}],
		"statuses": []
	}}`))
	if err != nil {
		t.Fatalf("fromCDEKOrderToInfo() error = %v", err)
	}
	pkg := info.Packages[0]
	if pkg.Weight != 0 || pkg.Length == nil || *pkg.Length != 0 {
		t.Errorf("package weight/length = %d/%v, ожидались 0/0", pkg.Weight, pkg.Length)
	}
	if pkg.Items[0].Weight != 0 || pkg.Items[0].Amount != 0 {
		t.Errorf("item weight/amount = %d/%d, ожидались 0/0", pkg.Items[0].Weight, pkg.Items[0].Amount)
	}
}
