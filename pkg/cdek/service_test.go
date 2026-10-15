package cdek

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
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

	service := NewService(client, nil)

	if service == nil {
		t.Fatal("NewService() returned nil")
	}
	if service.client != client {
		t.Error("Service client doesn't match")
	}
	if service.breaker == nil {
		t.Error("Circuit breaker not initialized")
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
	if service.mapper == nil {
		t.Error("Mapper not initialized")
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
	service := NewService(authClient, nil)

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
		service := NewService(client, nil)

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
		service := NewService(client, nil)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := service.HealthCheck(ctx)
		if err == nil {
			t.Error("HealthCheck() should return error for timeout")
		}
	})
}

// ========================
// Хелперы для httptest-тестов (без сети)
// ========================

func ptrString(s string) *string    { return &s }
func ptrInt32(i int32) *int32       { return &i }
func ptrBool(b bool) *bool          { return &b }
func ptrFloat64(f float64) *float64 { return &f }

// testOrderResponse - минимальный валидный ответ создания/обновления заказа
const testOrderResponse = `{"entity":{"uuid":"72753031-95f6-4e0a-8dd5-1b125f46a3e5","tariff_code":136,` +
	`"statuses":[],"requests":[{"date_time":"2026-10-15T10:00:00+03:00"}]}}`

// newTestService поднимает httptest-сервер: токен отдаем сами, остальные пути - в handler
func newTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.Handle("/", handler)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	config := &AccountConfig{
		Name:         "test",
		ClientID:     "id",
		ClientSecret: "secret",
		BaseURL:      server.URL,
		Timeout:      5 * time.Second,
	}
	client, err := NewAuthenticatedClient(config)
	if err != nil {
		t.Fatalf("NewAuthenticatedClient() error = %v", err)
	}
	return NewService(client, nil)
}

// baseOrderRequest - валидный заказ, проходящий orderValidator
func baseOrderRequest() *OrderRequest {
	return &OrderRequest{
		Type:       "1",
		TariffCode: 136,
		Recipient: Recipient{
			Contact: Contact{
				Name:   "Петр Петров",
				Phones: []Phone{{Number: "+79007654321"}},
			},
		},
		Packages: []OrderPackage{
			{
				Number: "1",
				Weight: 1000,
				Items: []Item{
					{Name: "Товар", WareKey: "SKU-001", Cost: 1000, Weight: 1000, Amount: 1},
				},
			},
		},
	}
}

// assertOptionalJSON сверяет объектное поле тела запроса с ожидаемым JSON.
// wantJSON == "" означает, что поле отправляться не должно
func assertOptionalJSON(t *testing.T, key string, got interface{}, wantJSON string) {
	t.Helper()
	if wantJSON == "" {
		if _, ok := got.(map[string]interface{}); ok {
			t.Errorf("поле %s отправлено, хотя не должно: %v", key, got)
		}
		return
	}
	gotMap, ok := got.(map[string]interface{})
	if !ok {
		t.Errorf("поле %s отсутствует или не объект: %v", key, got)
		return
	}
	var wantMap map[string]interface{}
	if err := json.Unmarshal([]byte(wantJSON), &wantMap); err != nil {
		t.Fatalf("некорректный wantJSON для %s: %v", key, err)
	}
	if !reflect.DeepEqual(gotMap, wantMap) {
		t.Errorf("поле %s = %v, ожидалось %v", key, gotMap, wantMap)
	}
}

// ========================
// CreateOrder: ПВЗ и адресная доставка
// ========================

func TestService_CreateOrderDeliveryPoint(t *testing.T) {
	tests := []struct {
		name              string
		req               *OrderRequest
		wantDeliveryPoint string // "" - ключа быть не должно
		wantShipmentPoint string // "" - ключа быть не должно
		wantToJSON        string // "" - ключа быть не должно
		wantFromJSON      string // "" - ключа быть не должно
	}{
		{
			name: "доставка до ПВЗ: delivery_point без to_location",
			req: func() *OrderRequest {
				req := baseOrderRequest()
				req.DeliveryPoint = ptrString("SPB12")
				req.FromLocation = Location{Code: ptrInt32(44)}
				return req
			}(),
			wantDeliveryPoint: "SPB12",
			wantFromJSON:      `{"code":44}`,
		},
		{
			name: "ПВЗ в ПВЗ: обе точки, адресные блоки не отправляются",
			req: func() *OrderRequest {
				req := baseOrderRequest()
				req.ShipmentPoint = ptrString("MSK109")
				req.DeliveryPoint = ptrString("SPB12")
				return req
			}(),
			wantShipmentPoint: "MSK109",
			wantDeliveryPoint: "SPB12",
		},
		{
			name: "склад-дверь: shipment_point + to_location",
			req: func() *OrderRequest {
				req := baseOrderRequest()
				req.ShipmentPoint = ptrString("MSK109")
				req.ToLocation = Location{Code: ptrInt32(137), Address: ptrString("Невский пр-т, 1")}
				return req
			}(),
			wantShipmentPoint: "MSK109",
			wantToJSON:        `{"code":137,"address":"Невский пр-т, 1"}`,
		},
		{
			name: "адресная доставка не изменилась (регрессия)",
			req: func() *OrderRequest {
				req := baseOrderRequest()
				req.FromLocation = Location{Code: ptrInt32(44), Address: ptrString("Ленинградский пр-т, 1")}
				req.ToLocation = Location{Code: ptrInt32(137)}
				return req
			}(),
			wantFromJSON: `{"code":44,"address":"Ленинградский пр-т, 1"}`,
			wantToJSON:   `{"code":137}`,
		},
		{
			name: "пустые Location не отправляются",
			req:  baseOrderRequest(),
		},
		{
			name: "Location только с городом отправляется",
			req: func() *OrderRequest {
				req := baseOrderRequest()
				req.ToLocation = Location{City: ptrString("Санкт-Петербург")}
				return req
			}(),
			wantToJSON: `{"city":"Санкт-Петербург"}`,
		},
		{
			name: "пустой код ПВЗ получателя не блокирует to_location",
			req: func() *OrderRequest {
				req := baseOrderRequest()
				req.DeliveryPoint = ptrString("")
				req.ToLocation = Location{Code: ptrInt32(137)}
				return req
			}(),
			wantToJSON: `{"code":137}`,
		},
		{
			name: "пустой код ПВЗ отправителя не блокирует from_location",
			req: func() *OrderRequest {
				req := baseOrderRequest()
				req.ShipmentPoint = ptrString("")
				req.FromLocation = Location{Code: ptrInt32(44)}
				return req
			}(),
			wantFromJSON: `{"code":44}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody []byte
			service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == "/v2/orders" {
					var err error
					gotBody, err = io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("read request body: %v", err)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(testOrderResponse))
					return
				}
				t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			})

			resp, err := service.CreateOrder(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("CreateOrder() error = %v", err)
			}
			if resp.UUID == "" {
				t.Error("CreateOrder() вернул пустой UUID")
			}

			var body map[string]interface{}
			if err := json.Unmarshal(gotBody, &body); err != nil {
				t.Fatalf("unmarshal request body: %v", err)
			}

			if tt.wantDeliveryPoint == "" {
				if _, ok := body["delivery_point"]; ok {
					t.Errorf("delivery_point отправлен, хотя не должен: %v", body["delivery_point"])
				}
			} else if v, _ := body["delivery_point"].(string); v != tt.wantDeliveryPoint {
				t.Errorf("delivery_point = %q, ожидалось %q", v, tt.wantDeliveryPoint)
			}

			if tt.wantShipmentPoint == "" {
				if _, ok := body["shipment_point"]; ok {
					t.Errorf("shipment_point отправлен, хотя не должен: %v", body["shipment_point"])
				}
			} else if v, _ := body["shipment_point"].(string); v != tt.wantShipmentPoint {
				t.Errorf("shipment_point = %q, ожидалось %q", v, tt.wantShipmentPoint)
			}

			assertOptionalJSON(t, "to_location", body["to_location"], tt.wantToJSON)
			assertOptionalJSON(t, "from_location", body["from_location"], tt.wantFromJSON)
		})
	}
}

// TestService_CreateOrderPointAddressConflict: конфликт «код ПВЗ + адрес одной
// стороны» (shipment_point+from_location / delivery_point+to_location) — ошибка
// ErrInvalidRequest, запрос в API не уходит. Смешанные режимы (адрес с одной
// стороны, ПВЗ с другой) валидны и покрыты таблицей выше
func TestService_CreateOrderPointAddressConflict(t *testing.T) {
	tests := []struct {
		name  string
		build func() *OrderRequest
	}{
		{
			name: "shipment_point + from_location",
			build: func() *OrderRequest {
				req := baseOrderRequest()
				req.ShipmentPoint = ptrString("MSK109")
				req.FromLocation = Location{Code: ptrInt32(44)}
				return req
			},
		},
		{
			name: "delivery_point + to_location",
			build: func() *OrderRequest {
				req := baseOrderRequest()
				req.DeliveryPoint = ptrString("SPB12")
				req.ToLocation = Location{Code: ptrInt32(137)}
				return req
			},
		},
		{
			name: "адрес только город — тоже конфликт",
			build: func() *OrderRequest {
				req := baseOrderRequest()
				req.DeliveryPoint = ptrString("SPB12")
				req.ToLocation = Location{City: ptrString("Санкт-Петербург")}
				return req
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestSent bool
			service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
				requestSent = true
				w.WriteHeader(http.StatusInternalServerError)
			})

			_, err := service.CreateOrder(context.Background(), tt.build())
			if err == nil {
				t.Fatal("CreateOrder() ожидалась ошибка конфликта, получен nil")
			}
			if !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("CreateOrder() error = %v, ожидалась обертка ErrInvalidRequest", err)
			}
			if requestSent {
				t.Error("запрос ушел в API несмотря на конфликт ПВЗ+адрес")
			}
		})
	}
}

// ========================
// UpdateOrder: ПВЗ и адресная доставка
// ========================

func TestService_UpdateOrderDeliveryPoint(t *testing.T) {
	tests := []struct {
		name              string
		req               *UpdateOrderRequest
		wantDeliveryPoint string // "" - ключа быть не должно
		wantShipmentPoint string // "" - ключа быть не должно
		wantToJSON        string // "" - ключа быть не должно
		wantFromJSON      string // "" - ключа быть не должно
	}{
		{
			name: "обновление ПВЗ в ПВЗ: обе точки, адресные блоки не отправляются",
			req: &UpdateOrderRequest{
				OrderUUID:     "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
				ShipmentPoint: ptrString("MSK109"),
				DeliveryPoint: ptrString("SPB12"),
			},
			wantShipmentPoint: "MSK109",
			wantDeliveryPoint: "SPB12",
		},
		{
			name: "адресное обновление не изменилось (регрессия)",
			req: &UpdateOrderRequest{
				OrderUUID:  "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
				ToLocation: &Location{Code: ptrInt32(137)},
			},
			wantToJSON: `{"code":137}`,
		},
		{
			name: "пустой указатель Location не дает пустого to_location",
			req: &UpdateOrderRequest{
				OrderUUID:  "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
				ToLocation: &Location{},
			},
		},
		{
			name: "пустые коды ПВЗ не блокируют адресные блоки",
			req: &UpdateOrderRequest{
				OrderUUID:     "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
				ShipmentPoint: ptrString(""),
				FromLocation:  &Location{Code: ptrInt32(44)},
				DeliveryPoint: ptrString(""),
				ToLocation:    &Location{Code: ptrInt32(137)},
			},
			wantToJSON:   `{"code":137}`,
			wantFromJSON: `{"code":44}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody []byte
			service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPatch && r.URL.Path == "/v2/orders" {
					var err error
					gotBody, err = io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("read request body: %v", err)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(testOrderResponse))
					return
				}
				t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			})

			resp, err := service.UpdateOrder(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("UpdateOrder() error = %v", err)
			}
			if resp.UUID == "" {
				t.Error("UpdateOrder() вернул пустой UUID")
			}

			var body map[string]interface{}
			if err := json.Unmarshal(gotBody, &body); err != nil {
				t.Fatalf("unmarshal request body: %v", err)
			}

			if tt.wantDeliveryPoint == "" {
				if _, ok := body["delivery_point"]; ok {
					t.Errorf("delivery_point отправлен, хотя не должен: %v", body["delivery_point"])
				}
			} else if v, _ := body["delivery_point"].(string); v != tt.wantDeliveryPoint {
				t.Errorf("delivery_point = %q, ожидалось %q", v, tt.wantDeliveryPoint)
			}

			if tt.wantShipmentPoint == "" {
				if _, ok := body["shipment_point"]; ok {
					t.Errorf("shipment_point отправлен, хотя не должен: %v", body["shipment_point"])
				}
			} else if v, _ := body["shipment_point"].(string); v != tt.wantShipmentPoint {
				t.Errorf("shipment_point = %q, ожидалось %q", v, tt.wantShipmentPoint)
			}

			assertOptionalJSON(t, "to_location", body["to_location"], tt.wantToJSON)
			assertOptionalJSON(t, "from_location", body["from_location"], tt.wantFromJSON)
		})
	}
}

// TestService_UpdateOrderPointAddressConflict: в UpdateOrder тот же XOR-конфликт
// «код ПВЗ + адрес одной стороны» — ошибка ErrInvalidRequest из маппера, PATCH не уходит
func TestService_UpdateOrderPointAddressConflict(t *testing.T) {
	tests := []struct {
		name string
		req  *UpdateOrderRequest
	}{
		{
			name: "delivery_point + to_location",
			req: &UpdateOrderRequest{
				OrderUUID:     "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
				DeliveryPoint: ptrString("SPB12"),
				ToLocation:    &Location{Code: ptrInt32(137)},
			},
		},
		{
			name: "shipment_point + from_location",
			req: &UpdateOrderRequest{
				OrderUUID:     "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
				ShipmentPoint: ptrString("MSK109"),
				FromLocation:  &Location{Code: ptrInt32(44)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestSent bool
			service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
				requestSent = true
				w.WriteHeader(http.StatusInternalServerError)
			})

			_, err := service.UpdateOrder(context.Background(), tt.req)
			if err == nil {
				t.Fatal("UpdateOrder() ожидалась ошибка конфликта, получен nil")
			}
			if !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("UpdateOrder() error = %v, ожидалась обертка ErrInvalidRequest", err)
			}
			if requestSent {
				t.Error("PATCH ушел в API несмотря на конфликт ПВЗ+адрес")
			}
		})
	}
}

// TestService_UpdateOrderType: по спеке OrderUpdateRequestDto требует type и recipient.
// Type отправляется только когда задан; единицы API — 1 (интернет-магазин) / 2 (доставка)
func TestService_UpdateOrderType(t *testing.T) {
	tests := []struct {
		name     string
		reqType  string
		wantType interface{} // nil - ключа type быть не должно
	}{
		{name: "delivery маппится в 2", reqType: "delivery", wantType: float64(2)},
		{name: "online_store маппится в 1", reqType: "online_store", wantType: float64(1)},
		{name: "2 передается как есть", reqType: "2", wantType: float64(2)},
		{name: "пустой тип не отправляется (поведение как раньше)", reqType: "", wantType: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody []byte
			service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPatch && r.URL.Path == "/v2/orders" {
					var err error
					gotBody, err = io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("read request body: %v", err)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(testOrderResponse))
					return
				}
				t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			})

			_, err := service.UpdateOrder(context.Background(), &UpdateOrderRequest{
				OrderUUID: "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
				Type:      tt.reqType,
				Recipient: &Recipient{
					Contact: Contact{Name: "Петр Петров", Phones: []Phone{{Number: "+79007654321"}}},
				},
			})
			if err != nil {
				t.Fatalf("UpdateOrder() error = %v", err)
			}

			var body map[string]interface{}
			if err := json.Unmarshal(gotBody, &body); err != nil {
				t.Fatalf("unmarshal request body: %v", err)
			}

			if tt.wantType == nil {
				if _, ok := body["type"]; ok {
					t.Errorf("type отправлен, хотя не должен: %v", body["type"])
				}
			} else if got, _ := body["type"].(float64); got != tt.wantType {
				t.Errorf("type = %v, ожидалось %v", body["type"], tt.wantType)
			}
		})
	}
}

// ========================
// ListDeliveryPoints: query-параметры
// ========================

func TestService_ListDeliveryPointsQuery(t *testing.T) {
	tests := []struct {
		name      string
		req       *DeliveryPointsRequest
		wantQuery url.Values
	}{
		{
			name: "все фильтры доходят в query",
			req: &DeliveryPointsRequest{
				CityCode:       "137",
				Type:           "PVZ",
				Code:           "NSK1",
				CountryCode:    "RU",
				RegionCode:     ptrInt32(77),
				PostalCode:     "190000",
				FiasGUID:       "fias&guid=1",
				HaveCashless:   ptrBool(true),
				HaveCash:       ptrBool(false),
				AllowedCOD:     ptrBool(true),
				IsDressingRoom: ptrBool(false),
				IsHandout:      ptrBool(true),
				IsReception:    ptrBool(true),
				TakeOnly:       ptrBool(false),
				WeightMax:      ptrInt32(10), // кг
				WeightMin:      ptrInt32(5),  // кг
				Size:           ptrInt32(50),
				Page:           ptrInt32(2),
			},
			wantQuery: url.Values{
				"city_code":        {"137"},
				"type":             {"PVZ"},
				"code":             {"NSK1"},
				"country_code":     {"RU"},
				"region_code":      {"77"},
				"postal_code":      {"190000"},
				"fias_guid":        {"fias&guid=1"},
				"have_cashless":    {"1"},
				"have_cash":        {"0"},
				"allowed_cod":      {"1"},
				"is_dressing_room": {"0"},
				"is_handout":       {"1"},
				"is_reception":     {"1"},
				"take_only":        {"0"},
				"weight_max":       {"10"},
				"weight_min":       {"5"},
				"size":             {"50"},
				"page":             {"2"},
			},
		},
		{
			name: "фильтр по коду ПВЗ",
			req: &DeliveryPointsRequest{
				Code: "NSK1",
			},
			wantQuery: url.Values{
				"code": {"NSK1"},
			},
		},
		{
			name: "size/page со значением <= 0 не отправляются",
			req: &DeliveryPointsRequest{
				CityCode: "137",
				Size:     ptrInt32(0),
				Page:     ptrInt32(-1),
			},
			wantQuery: url.Values{
				"city_code": {"137"},
			},
		},
		{
			name:      "пустой запрос - query пустая",
			req:       &DeliveryPointsRequest{},
			wantQuery: url.Values{},
		},
		{
			name: "спецсимволы кодируются",
			req: &DeliveryPointsRequest{
				CityCode: "137&type=POSTAMAT",
				FiasGUID: "{0a1b2c3d}",
			},
			wantQuery: url.Values{
				"city_code": {"137&type=POSTAMAT"},
				"fias_guid": {"{0a1b2c3d}"},
			},
		},
		{
			name: "ложные флаги отправляются, nil - нет",
			req: &DeliveryPointsRequest{
				CityCode: "44",
				HaveCash: ptrBool(false),
			},
			wantQuery: url.Values{
				"city_code": {"44"},
				"have_cash": {"0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery url.Values
			var gotRawQuery string
			service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/v2/deliverypoints" {
					gotQuery = r.URL.Query()
					gotRawQuery = r.URL.RawQuery
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`[{"code":"SPB12","type":"PVZ"}]`))
					return
				}
				t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			})

			points, err := service.ListDeliveryPoints(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("ListDeliveryPoints() error = %v", err)
			}
			if len(points) != 1 || points[0].Code != "SPB12" {
				t.Errorf("ListDeliveryPoints() = %+v, ожидался один пункт SPB12", points)
			}

			if !reflect.DeepEqual(gotQuery, tt.wantQuery) {
				t.Errorf("query = %v, ожидалось %v", gotQuery, tt.wantQuery)
			}

			// Спецсимволы обязаны быть закодированы в raw query
			if strings.Contains(tt.req.FiasGUID, "&") &&
				!strings.Contains(gotRawQuery, "fias_guid=fias%26guid%3D1") {
				t.Errorf("fias_guid не закодирован в raw query: %s", gotRawQuery)
			}
			if len(tt.wantQuery) == 0 && gotRawQuery != "" {
				t.Errorf("пустой запрос дал непустую query: %s", gotRawQuery)
			}
		})
	}
}

// ========================
// Разбор ответа /v2/deliverypoints
// ========================

func TestFromCDEKDeliveryPoints(t *testing.T) {
	m := newDtoMapper()

	tests := []struct {
		name      string
		json      string
		want      []DeliveryPoint
		wantError bool
	}{
		{
			name: "все новые поля",
			json: `[{
				"code": "NSK4",
				"name": "ПВЗ Новосибирск",
				"type": "PVZ",
				"work_time": "Пн-Пт 10-20",
				"have_cashless": true,
				"have_cash": true,
				"allowed_cod": false,
				"is_dressing_room": true,
				"weight_min": 1.5,
				"weight_max": 15,
				"owner_code": "CDEK",
				"location": {
					"country": "Россия",
					"region": "Новосибирская область",
					"region_code": 54,
					"city": "Новосибирск",
					"city_code": 1374,
					"address": "ул. Ленина, 12",
					"address_full": "Россия, Новосибирск, ул. Ленина, 12",
					"postal_code": "630099",
					"latitude": 55.030204,
					"longitude": 82.920430
				}
			}]`,
			want: []DeliveryPoint{{
				Code:           "NSK4",
				Name:           "ПВЗ Новосибирск",
				Type:           "PVZ",
				WorkTime:       "Пн-Пт 10-20",
				HaveCashless:   true,
				HaveCash:       true,
				AllowedCOD:     false,
				IsDressingRoom: true,
				WeightMin:      ptrFloat64(1.5), // кг
				WeightMax:      ptrFloat64(15),  // кг
				OwnerCode:      "CDEK",
				Location: PointLocation{
					Country:     "Россия",
					Region:      "Новосибирская область",
					RegionCode:  54,
					City:        "Новосибирск",
					CityCode:    1374,
					Address:     "ул. Ленина, 12",
					AddressFull: "Россия, Новосибирск, ул. Ленина, 12",
					PostalCode:  "630099",
					Latitude:    55.030204,
					Longitude:   82.920430,
				},
			}},
		},
		{
			name: "без новых полей - нулевые значения и nil-указатели без ошибки",
			json: `[{"code": "MSK1", "name": "ПВЗ Москва", "type": "POSTAMAT",
				"location": {"city": "Москва"}}]`,
			want: []DeliveryPoint{{
				Code: "MSK1",
				Name: "ПВЗ Москва",
				Type: "POSTAMAT",
				Location: PointLocation{
					City: "Москва",
				},
			}},
		},
		{
			name: "city_code/region_code вне диапазона int32 остаются нулем",
			json: `[{"code": "BIG", "type": "PVZ",
				"location": {"city_code": 9999999999, "region_code": 54}}]`,
			want: []DeliveryPoint{{
				Code: "BIG",
				Type: "PVZ",
				Location: PointLocation{
					RegionCode: 54,
				},
			}},
		},
		{
			name: "пустой список",
			json: `[]`,
			want: []DeliveryPoint{},
		},
		{
			name:      "битый JSON",
			json:      `{`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			points, err := m.fromCDEKDeliveryPoints([]byte(tt.json))
			if tt.wantError {
				if err == nil {
					t.Fatal("fromCDEKDeliveryPoints() ожидалась ошибка, получен nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("fromCDEKDeliveryPoints() error = %v", err)
			}
			if len(points) != len(tt.want) {
				t.Fatalf("len(points) = %d, ожидалось %d", len(points), len(tt.want))
			}
			for i := range tt.want {
				if !reflect.DeepEqual(points[i], tt.want[i]) {
					t.Errorf("points[%d] = %+v, ожидалось %+v", i, points[i], tt.want[i])
				}
			}
		})
	}
}

// TestFromCDEKOrderToInfo: разбор ответа GetOrder — коды ПВЗ это верхнеуровневые
// строки entity (OrderResponseDto: delivery_point, shipment_point)
func TestFromCDEKOrderToInfoPoints(t *testing.T) {
	m := newDtoMapper()

	tests := []struct {
		name              string
		json              string
		wantDeliveryPoint string
		wantShipmentPoint string
	}{
		{
			name: "коды ПВЗ разбираются из ответа",
			json: `{"entity":{
				"uuid": "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
				"type": 1,
				"tariff_code": 136,
				"shipment_point": "MSK109",
				"delivery_point": "SPB12",
				"statuses": []
			}}`,
			wantDeliveryPoint: "SPB12",
			wantShipmentPoint: "MSK109",
		},
		{
			name: "адресная доставка - коды ПВЗ пустые",
			json: `{"entity":{
				"uuid": "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
				"type": 2,
				"tariff_code": 1,
				"statuses": []
			}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := m.fromCDEKOrderToInfo([]byte(tt.json))
			if err != nil {
				t.Fatalf("fromCDEKOrderToInfo() error = %v", err)
			}
			if info.DeliveryPoint != tt.wantDeliveryPoint {
				t.Errorf("DeliveryPoint = %q, ожидалось %q", info.DeliveryPoint, tt.wantDeliveryPoint)
			}
			if info.ShipmentPoint != tt.wantShipmentPoint {
				t.Errorf("ShipmentPoint = %q, ожидалось %q", info.ShipmentPoint, tt.wantShipmentPoint)
			}
		})
	}
}

// TestService_GetOrderLocations: entity.from_location / entity.to_location из ответа
// GET /v2/orders/{uuid} разбираются в OrderInfo.FromLocation/ToLocation
func TestService_GetOrderLocations(t *testing.T) {
	const orderJSON = `{"entity":{
		"uuid": "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
		"type": 1,
		"tariff_code": 136,
		"from_location": {
			"code": 44,
			"fias_guid": "0c5b2444-70a0-4932-980c-b4dc0d3f02df",
			"postal_code": "125167",
			"country_code": "RU",
			"region": "Москва",
			"region_code": 77,
			"city": "Москва",
			"city_uuid": "0c5b2444-70a0-4932-980c-b4dc0d3f02df",
			"address": "Ленинградский пр-т, 39",
			"sub_region": "Москва",
			"latitude": 55.789833,
			"longitude": 37.548308
		},
		"to_location": {
			"code": 250,
			"fias_guid": "c2e16ded-6f61-4024-9c05-4b7c0f035957",
			"postal_code": "190000",
			"country_code": "RU",
			"region": "Санкт-Петербург",
			"region_code": 78,
			"city": "Санкт-Петербург",
			"city_uuid": "c2e16ded-6f61-4024-9c05-4b7c0f035957",
			"address": "Невский пр-т, 1",
			"latitude": 59.9386,
			"longitude": 30.3141
		},
		"shipment_point": "MSK109",
		"delivery_point": "SPB12",
		"statuses": []
	}}`

	wantFrom := Location{
		Code:        ptrInt32(44),
		FiasGUID:    ptrString("0c5b2444-70a0-4932-980c-b4dc0d3f02df"),
		PostalCode:  ptrString("125167"),
		CountryCode: ptrString("RU"),
		Region:      ptrString("Москва"),
		City:        ptrString("Москва"),
		Address:     ptrString("Ленинградский пр-т, 39"),
	}
	wantTo := Location{
		Code:        ptrInt32(250),
		FiasGUID:    ptrString("c2e16ded-6f61-4024-9c05-4b7c0f035957"),
		PostalCode:  ptrString("190000"),
		CountryCode: ptrString("RU"),
		Region:      ptrString("Санкт-Петербург"),
		City:        ptrString("Санкт-Петербург"),
		Address:     ptrString("Невский пр-т, 1"),
	}

	var gotPath string
	service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/orders/") {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(orderJSON))
			return
		}
		t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})

	info, err := service.GetOrder(context.Background(), "72753031-95f6-4e0a-8dd5-1b125f46a3e5")
	if err != nil {
		t.Fatalf("GetOrder() error = %v", err)
	}
	if gotPath != "/v2/orders/72753031-95f6-4e0a-8dd5-1b125f46a3e5" {
		t.Errorf("путь запроса = %q, ожидался /v2/orders/{uuid}", gotPath)
	}

	// Ключи без аналогов в Location (region_code, city_uuid, sub_region,
	// latitude, longitude) игнорируются
	if !reflect.DeepEqual(info.FromLocation, wantFrom) {
		t.Errorf("FromLocation = %+v, ожидалось %+v", info.FromLocation, wantFrom)
	}
	if !reflect.DeepEqual(info.ToLocation, wantTo) {
		t.Errorf("ToLocation = %+v, ожидалось %+v", info.ToLocation, wantTo)
	}
}

// TestService_GetOrderWithoutLocations: ответ без from_location/to_location —
// Location остаются нулевыми, ошибки нет
func TestService_GetOrderWithoutLocations(t *testing.T) {
	const orderJSON = `{"entity":{
		"uuid": "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
		"type": 1,
		"tariff_code": 136,
		"statuses": []
	}}`

	service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/orders/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(orderJSON))
			return
		}
		t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})

	info, err := service.GetOrder(context.Background(), "72753031-95f6-4e0a-8dd5-1b125f46a3e5")
	if err != nil {
		t.Fatalf("GetOrder() error = %v", err)
	}
	if !reflect.DeepEqual(info.FromLocation, Location{}) {
		t.Errorf("FromLocation = %+v, ожидался пустой Location", info.FromLocation)
	}
	if !reflect.DeepEqual(info.ToLocation, Location{}) {
		t.Errorf("ToLocation = %+v, ожидался пустой Location", info.ToLocation)
	}
}

// ========================
// fromCDEKOrderToInfo: type, packages, requests
// ========================

// TestFromCDEKOrderToInfoType: entity.type приходит числом (1/2) и маппится
// в "online_store"/"delivery"; неизвестное число — строкой цифр
func TestFromCDEKOrderToInfoType(t *testing.T) {
	m := newDtoMapper()

	tests := []struct {
		name     string
		rawType  string // значение entity.type в JSON; "" — ключа нет
		wantType string
	}{
		{name: "1 → online_store", rawType: `1`, wantType: "online_store"},
		{name: "2 → delivery", rawType: `2`, wantType: "delivery"},
		{name: "неизвестное число — строкой цифр", rawType: `7`, wantType: "7"},
		{name: "строка принимается как есть", rawType: `"delivery"`, wantType: "delivery"},
		{name: "отсутствует — пусто", rawType: ``, wantType: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entityJSON := `{"uuid":"72753031-95f6-4e0a-8dd5-1b125f46a3e5"}`
			if tt.rawType != "" {
				entityJSON = fmt.Sprintf(`{"uuid":"72753031-95f6-4e0a-8dd5-1b125f46a3e5","type":%s}`, tt.rawType)
			}

			info, err := m.fromCDEKOrderToInfo([]byte(`{"entity":` + entityJSON + `}`))
			if err != nil {
				t.Fatalf("fromCDEKOrderToInfo() error = %v", err)
			}
			if info.Type != tt.wantType {
				t.Errorf("Type = %q, ожидалось %q", info.Type, tt.wantType)
			}
		})
	}
}

// TestFromCDEKOrderToInfoRequests: корневой requests[] ответа GET /v2/orders —
// состояние асинхронного создания (SUCCESSFUL/INVALID) и его ошибки;
// date_time первого запроса — CreatedAt
func TestFromCDEKOrderToInfoRequests(t *testing.T) {
	m := newDtoMapper()

	tests := []struct {
		name         string
		requestsJSON string
		want         OrderRequestState
		wantCreated  string
	}{
		{
			name: "INVALID с ошибками",
			requestsJSON: `[{
				"request_uuid": "14e3cafc-95f6-4e0a-8dd5-1b125f46a3e5",
				"type": "CREATE",
				"date_time": "2026-08-19T12:34:56+03:00",
				"state": "INVALID",
				"errors": [
					{"code": "error_validate_receiver_delivery_point_is_empty", "message": "Не задан офис получателя"}
				]
			}]`,
			want: OrderRequestState{
				Type:     "CREATE",
				State:    "INVALID",
				DateTime: "2026-08-19T12:34:56+03:00",
				Errors: []OrderError{{
					Code:    "error_validate_receiver_delivery_point_is_empty",
					Message: "Не задан офис получателя",
				}},
			},
			wantCreated: "2026-08-19T12:34:56+03:00",
		},
		{
			name: "SUCCESSFUL без ошибок",
			requestsJSON: `[{
				"type": "CREATE",
				"date_time": "2026-08-19T12:00:00+03:00",
				"state": "SUCCESSFUL"
			}]`,
			want: OrderRequestState{
				Type:     "CREATE",
				State:    "SUCCESSFUL",
				DateTime: "2026-08-19T12:00:00+03:00",
			},
			wantCreated: "2026-08-19T12:00:00+03:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orderJSON := `{"entity":{"uuid":"72753031-95f6-4e0a-8dd5-1b125f46a3e5","type":2,"statuses":[]},` +
				`"requests":` + tt.requestsJSON + `}`

			info, err := m.fromCDEKOrderToInfo([]byte(orderJSON))
			if err != nil {
				t.Fatalf("fromCDEKOrderToInfo() error = %v", err)
			}
			if len(info.Requests) != 1 {
				t.Fatalf("len(Requests) = %d, ожидался 1", len(info.Requests))
			}
			if !reflect.DeepEqual(info.Requests[0], tt.want) {
				t.Errorf("Requests[0] = %+v, ожидалось %+v", info.Requests[0], tt.want)
			}
			if info.CreatedAt != tt.wantCreated {
				t.Errorf("CreatedAt = %q, ожидалось %q", info.CreatedAt, tt.wantCreated)
			}
		})
	}
}

// TestFromCDEKOrderToInfoFixtures: разбор живых фикстур GET /v2/orders —
// type числом, packages (включая заказ type=2 без items), requests на корне
func TestFromCDEKOrderToInfoFixtures(t *testing.T) {
	m := newDtoMapper()

	loadFixture := func(t *testing.T, name string) []byte {
		t.Helper()
		data, err := os.ReadFile("testdata/" + name) //nolint:gosec // путь фикстуры теста, не пользовательский ввод
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		return data
	}

	t.Run("order_t136_dptrue_sptrue.json: packages с items", func(t *testing.T) {
		info, err := m.fromCDEKOrderToInfo(loadFixture(t, "order_t136_dptrue_sptrue.json"))
		if err != nil {
			t.Fatalf("fromCDEKOrderToInfo() error = %v", err)
		}

		if info.Type != "online_store" {
			t.Errorf("Type = %q, ожидалось online_store (entity.type = 1)", info.Type)
		}
		if info.ShipmentPoint != "MSK553" || info.DeliveryPoint != "ZLN5" {
			t.Errorf("points = %q/%q, ожидались MSK553/ZLN5", info.ShipmentPoint, info.DeliveryPoint)
		}
		if info.FromLocation.Code == nil || *info.FromLocation.Code != 44 {
			t.Errorf("FromLocation.Code = %v, ожидался 44", info.FromLocation.Code)
		}

		if len(info.Packages) != 1 {
			t.Fatalf("len(Packages) = %d, ожидался 1", len(info.Packages))
		}
		pkg := info.Packages[0]
		if pkg.Number != "1" || pkg.Weight != 150 {
			t.Errorf("package number/weight = %q/%d, ожидались 1/150", pkg.Number, pkg.Weight)
		}
		if pkg.Length == nil || *pkg.Length != 12 || pkg.Width == nil || *pkg.Width != 9 || pkg.Height == nil || *pkg.Height != 3 {
			t.Errorf("package L/W/H = %v/%v/%v, ожидались 12/9/3", pkg.Length, pkg.Width, pkg.Height)
		}
		if pkg.Comment == nil {
			t.Error("package comment не разобран")
		}

		if len(pkg.Items) != 1 {
			t.Fatalf("len(Items) = %d, ожидался 1", len(pkg.Items))
		}
		item := pkg.Items[0]
		if item.Name == "" || item.WareKey != "11" {
			t.Errorf("item name/ware_key = %q/%q, ожидался непустой name и ware_key 11", item.Name, item.WareKey)
		}
		if item.Cost != 500 || item.Payment != 0 || item.Weight != 150 || item.Amount != 1 {
			t.Errorf("item cost/payment/weight/amount = %v/%v/%d/%d, ожидались 500/0/150/1",
				item.Cost, item.Payment, item.Weight, item.Amount)
		}

		// В фикстуре requests на корне — пустой массив
		if len(info.Requests) != 0 {
			t.Errorf("len(Requests) = %d, ожидался 0", len(info.Requests))
		}
	})

	t.Run("order_t483_dptrue_sptrue.json: type=2 без items", func(t *testing.T) {
		info, err := m.fromCDEKOrderToInfo(loadFixture(t, "order_t483_dptrue_sptrue.json"))
		if err != nil {
			t.Fatalf("fromCDEKOrderToInfo() error = %v", err)
		}

		if info.Type != "delivery" {
			t.Errorf("Type = %q, ожидалось delivery (entity.type = 2)", info.Type)
		}
		if info.ShipmentPoint != "TAR8" || info.DeliveryPoint != "MSK553" {
			t.Errorf("points = %q/%q, ожидались TAR8/MSK553", info.ShipmentPoint, info.DeliveryPoint)
		}

		if len(info.Packages) != 1 {
			t.Fatalf("len(Packages) = %d, ожидался 1", len(info.Packages))
		}
		pkg := info.Packages[0]
		if pkg.Number != "[P09]12005424607" || pkg.Weight != 150 {
			t.Errorf("package number/weight = %q/%d", pkg.Number, pkg.Weight)
		}
		if pkg.Length == nil || *pkg.Length != 24 || pkg.Width == nil || *pkg.Width != 22 || pkg.Height == nil || *pkg.Height != 1 {
			t.Errorf("package L/W/H = %v/%v/%v, ожидались 24/22/1", pkg.Length, pkg.Width, pkg.Height)
		}
		// У заказов type=2 items могут отсутствовать
		if len(pkg.Items) != 0 {
			t.Errorf("len(Items) = %d, ожидался 0 (delivery без items)", len(pkg.Items))
		}
	})
}
