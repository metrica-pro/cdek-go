package cdek

// spec_contract_test.go — контракт тел запросов относительно api/cdek-api.yaml.
// Хелпер читает required-списки схем OrderCreateRequestDto / OrderUpdateRequestDto /
// CalculatorTariffListRequestDto и проверяет, что реальные тела, отправляемые
// Service, содержат обязательные поля. Ловит дрейф тел относительно спеки.

import (
	"context"
	"net/http"
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// specPath — путь до OpenAPI-спецификации относительно пакета
const specPath = "../../api/cdek-api.yaml"

// loadSpec один раз читает и разбирает спецификацию
func loadSpec(t *testing.T) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(specPath) //nolint:gosec // путь спеки зашит в тесте
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}
	return doc
}

// specRequiredFields возвращает required-список схемы из components.schemas
func specRequiredFields(t *testing.T, doc map[string]interface{}, schema string) []string {
	t.Helper()
	components, ok := doc["components"].(map[string]interface{})
	if !ok {
		t.Fatal("в спеке нет components")
	}
	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("в спеке нет components.schemas")
	}
	schemaObj, ok := schemas[schema].(map[string]interface{})
	if !ok {
		t.Fatalf("в спеке нет схемы %s", schema)
	}
	rawRequired, ok := schemaObj["required"].([]interface{})
	if !ok {
		t.Fatalf("у схемы %s нет required-списка", schema)
	}
	required := make([]string, 0, len(rawRequired))
	for _, r := range rawRequired {
		name, ok := r.(string)
		if !ok {
			t.Fatalf("required схемы %s содержит не-строку: %v", schema, r)
		}
		required = append(required, name)
	}
	return required
}

// assertSpecRequired проверяет наличие обязательных полей в теле запроса.
// conditional — поля, обязательность которых зависит от режима доставки
// (shipment_point/from_location, delivery_point/to_location): по спеке CDEK
// обязателен ровно один вариант каждой стороны, но режим неизвестен из тела,
// поэтому такие поля исключаются из строгой проверки (XOR проверяется отдельно).
func assertSpecRequired(t *testing.T, body map[string]interface{}, required []string, conditional map[string]bool) {
	t.Helper()
	for _, field := range required {
		if conditional[field] {
			continue
		}
		if _, ok := body[field]; !ok {
			t.Errorf("обязательное по спеке поле %q отсутствует в теле: %v", field, body)
		}
	}
}

// assertNoPointAddressConflict: код ПВЗ и адрес одной стороны взаимоисключающие
func assertNoPointAddressConflict(t *testing.T, body map[string]interface{}) {
	t.Helper()
	if _, hasPoint := body["shipment_point"]; hasPoint {
		if _, hasLoc := body["from_location"]; hasLoc {
			t.Error("shipment_point и from_location одновременно в теле (спека запрещает)")
		}
	}
	if _, hasPoint := body["delivery_point"]; hasPoint {
		if _, hasLoc := body["to_location"]; hasLoc {
			t.Error("delivery_point и to_location одновременно в теле (спека запрещает)")
		}
	}
}

// captureRequest запускает call против httptest-сервера и возвращает тело запроса
func captureRequestBody(t *testing.T, method, path string, call func(*Service)) map[string]interface{} {
	t.Helper()
	var gotBody map[string]interface{}
	service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == method && r.URL.Path == path {
			gotBody = readJSONBody(t, r)
			respondJSON(w, testOrderResponse)
			return
		}
		t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
	call(service)
	if gotBody == nil {
		t.Fatal("тело запроса не зафиксировано")
	}
	return gotBody
}

// TestSpecContract_SchemasExist: спека содержит ожидаемые required-поля.
// Если кто-то ослабит спеку или хелпер — тест это поймает
func TestSpecContract_SchemasExist(t *testing.T) {
	doc := loadSpec(t)

	want := []struct {
		schema string
		field  string
	}{
		{"OrderCreateRequestDto", "recipient"},
		{"OrderCreateRequestDto", "tariff_code"},
		{"OrderCreateRequestDto", "packages"},
		{"OrderUpdateRequestDto", "recipient"},
		{"OrderUpdateRequestDto", "type"},
		{"CalculatorTariffListRequestDto", "from_location"},
		{"CalculatorTariffListRequestDto", "to_location"},
		{"CalculatorTariffListRequestDto", "packages"},
	}
	for _, w := range want {
		t.Run(w.schema+"."+w.field, func(t *testing.T) {
			required := specRequiredFields(t, doc, w.schema)
			for _, f := range required {
				if f == w.field {
					return
				}
			}
			t.Errorf("required схемы %s не содержит %q: %v", w.schema, w.field, required)
		})
	}
}

// TestSpecContract_CreateOrderBody: POST /v2/orders — обязательные поля
// OrderCreateRequestDto в теле, type числом, XOR «ПВЗ vs адрес»
func TestSpecContract_CreateOrderBody(t *testing.T) {
	doc := loadSpec(t)
	required := specRequiredFields(t, doc, "OrderCreateRequestDto")
	conditional := map[string]bool{
		"shipment_point": true, "from_location": true,
		"delivery_point": true, "to_location": true,
	}

	t.Run("адресная доставка", func(t *testing.T) {
		req := baseOrderRequest()
		req.FromLocation = Location{Code: ptrInt32(44)}
		req.ToLocation = Location{Code: ptrInt32(250), Address: ptrString("Невский пр-т, 1")}

		body := captureRequestBody(t, http.MethodPost, "/v2/orders", func(s *Service) {
			if _, err := s.CreateOrder(context.Background(), req); err != nil {
				t.Errorf("CreateOrder() error = %v", err)
			}
		})

		assertSpecRequired(t, body, required, conditional)
		assertNoPointAddressConflict(t, body)
		// type отправляется числом (format: int32 в спеке)
		if typ, ok := body["type"].(float64); !ok || typ != 1 {
			t.Errorf("type = %#v, ожидалось число 1", body["type"])
		}
	})

	t.Run("доставка через ПВЗ", func(t *testing.T) {
		req := baseOrderRequest()
		req.ShipmentPoint = ptrString("MSK109")
		req.DeliveryPoint = ptrString("SPB12")

		body := captureRequestBody(t, http.MethodPost, "/v2/orders", func(s *Service) {
			if _, err := s.CreateOrder(context.Background(), req); err != nil {
				t.Errorf("CreateOrder() error = %v", err)
			}
		})

		assertSpecRequired(t, body, required, conditional)
		assertNoPointAddressConflict(t, body)
		if body["shipment_point"] != "MSK109" || body["delivery_point"] != "SPB12" {
			t.Errorf("points = %v/%v", body["shipment_point"], body["delivery_point"])
		}
	})
}

// TestSpecContract_UpdateOrderBody: PATCH /v2/orders — обязательные поля
// OrderUpdateRequestDto (type, recipient) в теле, type числом
func TestSpecContract_UpdateOrderBody(t *testing.T) {
	doc := loadSpec(t)
	required := specRequiredFields(t, doc, "OrderUpdateRequestDto")
	conditional := map[string]bool{
		"shipment_point": true, "from_location": true,
		"delivery_point": true, "to_location": true,
	}

	body := captureRequestBody(t, http.MethodPatch, "/v2/orders", func(s *Service) {
		if _, err := s.UpdateOrder(context.Background(), &UpdateOrderRequest{
			OrderUUID:  "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
			Type:       "online_store",
			Recipient:  &Recipient{Contact: Contact{Name: "Петр Петров", Phones: []Phone{{Number: "+79007654321"}}}},
			ToLocation: &Location{Code: ptrInt32(250)},
		}); err != nil {
			t.Errorf("UpdateOrder() error = %v", err)
		}
	})

	assertSpecRequired(t, body, required, conditional)
	assertNoPointAddressConflict(t, body)
	if typ, ok := body["type"].(float64); !ok || typ != 1 {
		t.Errorf("type = %#v, ожидалось число 1", body["type"])
	}
	if body["uuid"] != "72753031-95f6-4e0a-8dd5-1b125f46a3e5" {
		t.Errorf("uuid = %v", body["uuid"])
	}
}

// TestSpecContract_UpdateOrderBodyDrift: документированный дрейф — маппер
// отправляет type/recipient только когда они заданы, спека требует их всегда.
// Тест фиксирует текущее поведение: минимальный PATCH уходит без обязательных
// полей. При добавлении валидации UpdateOrderRequest этот тест надо заменить
// на негативную проверку (ошибка до запроса)
func TestSpecContract_UpdateOrderBodyDrift(t *testing.T) {
	body := captureRequestBody(t, http.MethodPatch, "/v2/orders", func(s *Service) {
		if _, err := s.UpdateOrder(context.Background(), &UpdateOrderRequest{
			OrderUUID: "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
			Comment:   ptrString("x"),
		}); err != nil {
			t.Errorf("UpdateOrder() error = %v", err)
		}
	})

	if _, ok := body["type"]; ok {
		t.Errorf("type неожиданно отправлен: %v", body["type"])
	}
	if _, ok := body["recipient"]; ok {
		t.Errorf("recipient неожиданно отправлен: %v", body["recipient"])
	}
}

// TestSpecContract_CalculatorBody: POST /v2/calculator/tarifflist — обязательные
// поля CalculatorTariffListRequestDto (from_location, to_location, packages)
func TestSpecContract_CalculatorBody(t *testing.T) {
	doc := loadSpec(t)
	required := specRequiredFields(t, doc, "CalculatorTariffListRequestDto")

	var body map[string]interface{}
	service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v2/calculator/tarifflist" {
			body = readJSONBody(t, r)
			respondJSON(w, string(loadFixture(t, "calculator_tarifflist_synthetic.json")))
			return
		}
		t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})

	if _, err := service.CalculateCost(context.Background(), &CostRequest{
		FromCityCode: 44,
		ToCityCode:   250,
		Packages:     []Package{{Weight: 1000, Length: 10, Width: 10, Height: 10}},
	}); err != nil {
		t.Fatalf("CalculateCost() error = %v", err)
	}

	assertSpecRequired(t, body, required, nil)
}
