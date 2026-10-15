package cdek

// service_methods_test.go — httptest-покрытие всех методов Service: успех,
// HTTP 400/401/404/500 → wrapHTTPError с типом ошибки, пустое тело, битый JSON,
// асинхронный 202, проверка тел запросов, пагинация списочных методов,
// цепочка «печать → скачивание PDF».

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// statusSentinel сопоставляет HTTP-статус ответа с ожидаемой ошибкой-сентинелом
func statusSentinel(code int) error {
	switch code {
	case http.StatusBadRequest:
		return ErrInvalidRequest
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return ErrServerError
	case http.StatusTooManyRequests:
		return ErrRateLimitExceeded
	default:
		return nil
	}
}

// errorStatusCodes — набор статусов для негативных проверок каждого метода
var errorStatusCodes = []int{400, 401, 404, 500}

// respondError отдает JSON-ошибку CDEK с указанным статусом
func respondError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"errors":[{"code":"v2_test_error","message":"тестовая ошибка"}]}`))
}

// respondJSON пишет успешный JSON-ответ
func respondJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

// assertSentinel проверяет, что ошибка несет ожидаемый тип
func assertSentinel(t *testing.T, err error, want error) {
	t.Helper()
	if want == nil {
		if err == nil {
			return
		}
		t.Fatalf("ошибка = %v, ожидался nil", err)
	}
	if !errors.Is(err, want) {
		t.Fatalf("ошибка = %v, ожидался сентинел %v", err, want)
	}
}

// readJSONBody читает и разбирает тело запроса
func readJSONBody(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal request body %q: %v", string(body), err)
	}
	return parsed
}

// ========================
// CalculateCost
// ========================

func TestService_CalculateCost(t *testing.T) {
	validReq := &CostRequest{
		FromCityCode: 44,
		ToCityCode:   250,
		Packages:     []Package{{Weight: 1500, Length: 10, Width: 20, Height: 30}},
	}

	t.Run("успех: тарифы разбираются", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/v2/calculator/tarifflist" {
				respondJSON(w, string(loadFixture(t, "calculator_tarifflist_synthetic.json")))
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		resp, err := service.CalculateCost(context.Background(), validReq)
		if err != nil {
			t.Fatalf("CalculateCost() error = %v", err)
		}
		if len(resp.Tariffs) != 2 || resp.Tariffs[0].TariffCode != 136 || resp.Tariffs[0].DeliverySum != 225 {
			t.Errorf("Tariffs = %+v, ожидались 2 тарифа (136 за 225)", resp.Tariffs)
		}
	})

	t.Run("тело запроса: from/to_location, packages, type=1, валюта=1", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/v2/calculator/tarifflist" {
				body := readJSONBody(t, r)
				if body["type"] != float64(1) || body["currency"] != float64(1) {
					t.Errorf("type/currency = %v/%v, ожидались 1/1", body["type"], body["currency"])
				}
				from, _ := body["from_location"].(map[string]interface{})
				to, _ := body["to_location"].(map[string]interface{})
				if from == nil || from["code"] != float64(44) {
					t.Errorf("from_location = %v, ожидался code 44", body["from_location"])
				}
				if to == nil || to["code"] != float64(250) {
					t.Errorf("to_location = %v, ожидался code 250", body["to_location"])
				}
				pkgs, _ := body["packages"].([]interface{})
				if len(pkgs) != 1 {
					t.Fatalf("packages = %v, ожидалась 1 упаковка", body["packages"])
				}
				pkg, _ := pkgs[0].(map[string]interface{})
				if pkg["weight"] != float64(1500) || pkg["length"] != float64(10) {
					t.Errorf("package = %v, ожидались weight=1500 length=10", pkg)
				}
				respondJSON(w, string(loadFixture(t, "calculator_tarifflist_synthetic.json")))
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		if _, err := service.CalculateCost(context.Background(), validReq); err != nil {
			t.Fatalf("CalculateCost() error = %v", err)
		}
	})

	t.Run("нет тарифов на маршруте — ошибка", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
			respondJSON(w, string(loadFixture(t, "calculator_tarifflist.json")))
		})
		_, err := service.CalculateCost(context.Background(), validReq)
		if err == nil || !strings.Contains(err.Error(), "no tariffs available") {
			t.Errorf("error = %v, ожидалась no tariffs available", err)
		}
	})

	t.Run("невалидный запрос не уходит в API", func(t *testing.T) {
		var requestSent bool
		service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
			requestSent = true
			respondError(w, http.StatusInternalServerError)
		})
		_, err := service.CalculateCost(context.Background(), &CostRequest{FromCityCode: 0, ToCityCode: 1})
		assertSentinel(t, err, ErrInvalidRequest)
		if requestSent {
			t.Error("запрос ушел в API несмотря на ошибку валидации")
		}
	})

	t.Run("HTTP-статусы → типы ошибок", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				_, err := service.CalculateCost(context.Background(), validReq)
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})

	t.Run("пустое тело ответа", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		if _, err := service.CalculateCost(context.Background(), validReq); err == nil {
			t.Error("ожидалась ошибка для пустого тела")
		}
	})

	t.Run("битый JSON", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
			respondJSON(w, `{"tariff_codes":`)
		})
		if _, err := service.CalculateCost(context.Background(), validReq); err == nil {
			t.Error("ожидалась ошибка для битого JSON")
		}
	})
}

// ========================
// TrackOrder / GetOrderByCDEKNumber
// ========================

func TestService_TrackOrder(t *testing.T) {
	t.Run("успех: живая фикстура", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/v2/orders/72753031-95f6-4e0a-8dd5-1b125f46a3e5" {
				respondJSON(w, string(loadFixture(t, "order_by_uuid_t136_dptrue_sptrue.json")))
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		tracking, err := service.TrackOrder(context.Background(), "72753031-95f6-4e0a-8dd5-1b125f46a3e5")
		if err != nil {
			t.Fatalf("TrackOrder() error = %v", err)
		}
		if tracking.UUID != "00000000-0000-4000-8000-000000000004" {
			t.Errorf("UUID = %q", tracking.UUID)
		}
		if tracking.Number == nil || *tracking.Number != "10000000001" {
			t.Errorf("Number = %v", tracking.Number)
		}
		if tracking.CurrentStatus.Code != "ACCEPTED" {
			t.Errorf("CurrentStatus.Code = %q, ожидался ACCEPTED", tracking.CurrentStatus.Code)
		}
	})

	t.Run("нет entity — ошибка", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
			respondJSON(w, `{"requests":[]}`)
		})
		if _, err := service.TrackOrder(context.Background(), "u1"); err == nil {
			t.Error("ожидалась ошибка для ответа без entity")
		}
	})

	t.Run("HTTP-статусы → типы ошибок", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				_, err := service.TrackOrder(context.Background(), "u1")
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})

	t.Run("пустое тело и битый JSON", func(t *testing.T) {
		for name, body := range map[string]string{"пустое": "", "битый": "{"} {
			t.Run(name, func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondJSON(w, body)
				})
				if _, err := service.TrackOrder(context.Background(), "u1"); err == nil {
					t.Error("ожидалась ошибка")
				}
			})
		}
	})
}

func TestService_GetOrderByCDEKNumber(t *testing.T) {
	t.Run("успех: cdek_number в query, заказ разобран", func(t *testing.T) {
		var gotQuery string
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/v2/orders" {
				gotQuery = r.URL.RawQuery
				respondJSON(w, string(loadFixture(t, "order_t483_dptrue_sptrue.json")))
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		info, err := service.GetOrderByCDEKNumber(context.Background(), "10278482691")
		if err != nil {
			t.Fatalf("GetOrderByCDEKNumber() error = %v", err)
		}
		if gotQuery != "cdek_number=10278482691" {
			t.Errorf("query = %q, ожидался cdek_number=10278482691", gotQuery)
		}
		if info.Type != "delivery" || info.DeliveryPoint != "MSK553" {
			t.Errorf("Type/DeliveryPoint = %q/%q, ожидались delivery/MSK553", info.Type, info.DeliveryPoint)
		}
	})

	t.Run("спецсимволы номера экранируются", func(t *testing.T) {
		var gotQuery string
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			respondJSON(w, string(loadFixture(t, "order_t483_dptrue_sptrue.json")))
		})
		if _, err := service.GetOrderByCDEKNumber(context.Background(), "12 34&x"); err != nil {
			t.Fatalf("GetOrderByCDEKNumber() error = %v", err)
		}
		if !strings.Contains(gotQuery, "cdek_number=12+34%26x") {
			t.Errorf("query = %q, ожидалось экранирование спецсимволов", gotQuery)
		}
	})

	t.Run("HTTP-статусы → типы ошибок", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				_, err := service.GetOrderByCDEKNumber(context.Background(), "1")
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})

	t.Run("пустое тело и битый JSON", func(t *testing.T) {
		for name, body := range map[string]string{"пустое": "", "битый": "{"} {
			t.Run(name, func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondJSON(w, body)
				})
				if _, err := service.GetOrderByCDEKNumber(context.Background(), "1"); err == nil {
					t.Error("ожидалась ошибка")
				}
			})
		}
	})
}

// ========================
// CreateOrder: асинхронный 202
// ========================

func TestService_CreateOrderAccepted(t *testing.T) {
	t.Run("202: uuid из related_entities", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/v2/orders" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{
					"related_entities": [{"type": "order", "uuid": "d3h6be25-2d77-4c56-b2cd-2b61b5f8c8da"}],
					"requests": [{"type": "CREATE", "state": "ACCEPTED", "date_time": "2026-10-15T09:00:00+03:00"}]}`))
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		resp, err := service.CreateOrder(context.Background(), baseOrderRequest())
		if err != nil {
			t.Fatalf("CreateOrder() error = %v", err)
		}
		if resp.UUID != "d3h6be25-2d77-4c56-b2cd-2b61b5f8c8da" {
			t.Errorf("UUID = %q, ожидался из related_entities", resp.UUID)
		}
		if resp.CreatedAt != "2026-10-15T09:00:00+03:00" {
			t.Errorf("CreatedAt = %q", resp.CreatedAt)
		}
	})
}

// ========================
// CancelOrder
// ========================

func TestService_CancelOrder(t *testing.T) {
	t.Run("успех: DELETE /v2/orders/{uuid}", func(t *testing.T) {
		var gotMethod, gotPath string
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			respondJSON(w, `{"requests":[{"state":"SUCCESSFUL"}]}`)
		})

		if err := service.CancelOrder(context.Background(), "72753031-95f6-4e0a-8dd5-1b125f46a3e5"); err != nil {
			t.Fatalf("CancelOrder() error = %v", err)
		}
		if gotMethod != http.MethodDelete || gotPath != "/v2/orders/72753031-95f6-4e0a-8dd5-1b125f46a3e5" {
			t.Errorf("запрос = %s %s, ожидался DELETE /v2/orders/{uuid}", gotMethod, gotPath)
		}
	})

	t.Run("успех с пустым телом", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		if err := service.CancelOrder(context.Background(), "u1"); err != nil {
			t.Fatalf("CancelOrder() error = %v", err)
		}
	})

	t.Run("HTTP-статусы → типы ошибок", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				err := service.CancelOrder(context.Background(), "u1")
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})
}

// ========================
// ListCities / ListRegions
// ========================

func TestService_ListCities(t *testing.T) {
	t.Run("успех: фикстура городов + фильтры в query", func(t *testing.T) {
		var gotQuery url.Values
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/v2/location/cities" {
				gotQuery = r.URL.Query()
				respondJSON(w, string(loadFixture(t, "cities_ru.json")))
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		cities, err := service.ListCities(context.Background(), &CitiesRequest{
			CountryCode: ptrString("RU"),
			RegionCode:  ptrInt(81),
			PostalCode:  ptrString("109544"),
			Code:        ptrInt(44),
			City:        ptrString("Москва"),
			Size:        ptrInt(100),
			Page:        ptrInt(1),
		})
		if err != nil {
			t.Fatalf("ListCities() error = %v", err)
		}
		if len(cities) != 2 || cities[0].Code != 5 || cities[0].City != "Усинск" {
			t.Errorf("cities = %+v, ожидались 2 города (первый Усинск/5)", cities)
		}
		for key, want := range map[string]string{
			"country_codes": "RU", "region_code": "81", "postal_code": "109544",
			"code": "44", "city": "Москва", "size": "100", "page": "1",
		} {
			if got := gotQuery.Get(key); got != want {
				t.Errorf("query[%s] = %q, ожидался %q", key, got, want)
			}
		}
	})

	t.Run("пустой фильтр — query пустая, пустой список без ошибки", func(t *testing.T) {
		var gotRawQuery string
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			gotRawQuery = r.URL.RawQuery
			respondJSON(w, `[]`)
		})
		cities, err := service.ListCities(context.Background(), &CitiesRequest{})
		if err != nil {
			t.Fatalf("ListCities() error = %v", err)
		}
		if len(cities) != 0 {
			t.Errorf("len(cities) = %d, ожидался 0", len(cities))
		}
		if gotRawQuery != "" {
			t.Errorf("query = %q, ожидалась пустая", gotRawQuery)
		}
	})

	t.Run("HTTP-статусы → типы ошибок", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				_, err := service.ListCities(context.Background(), &CitiesRequest{})
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})

	t.Run("пустое тело и битый JSON", func(t *testing.T) {
		for name, body := range map[string]string{"пустое": "", "битый": "["} {
			t.Run(name, func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondJSON(w, body)
				})
				if _, err := service.ListCities(context.Background(), &CitiesRequest{}); err == nil {
					t.Error("ожидалась ошибка")
				}
			})
		}
	})
}

func TestService_ListRegions(t *testing.T) {
	t.Run("успех: фикстура регионов + пагинация в query", func(t *testing.T) {
		var gotQuery url.Values
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/v2/location/regions" {
				gotQuery = r.URL.Query()
				respondJSON(w, string(loadFixture(t, "regions_ru.json")))
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		regions, err := service.ListRegions(context.Background(), &RegionsRequest{
			CountryCode: ptrString("RU"),
			Size:        ptrInt(50),
			Page:        ptrInt(2),
		})
		if err != nil {
			t.Fatalf("ListRegions() error = %v", err)
		}
		if len(regions) != 2 || regions[0].Code != 80 {
			t.Errorf("regions = %+v, ожидались 2 региона (первый 80)", regions)
		}
		for key, want := range map[string]string{
			"country_codes": "RU", "size": "50", "page": "2",
		} {
			if got := gotQuery.Get(key); got != want {
				t.Errorf("query[%s] = %q, ожидался %q", key, got, want)
			}
		}
	})

	t.Run("пустой список", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
			respondJSON(w, `[]`)
		})
		regions, err := service.ListRegions(context.Background(), &RegionsRequest{})
		if err != nil {
			t.Fatalf("ListRegions() error = %v", err)
		}
		if len(regions) != 0 {
			t.Errorf("len(regions) = %d, ожидался 0", len(regions))
		}
	})

	t.Run("HTTP-статусы → типы ошибок", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				_, err := service.ListRegions(context.Background(), &RegionsRequest{})
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})

	t.Run("пустое тело и битый JSON", func(t *testing.T) {
		for name, body := range map[string]string{"пустое": "", "битый": "["} {
			t.Run(name, func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondJSON(w, body)
				})
				if _, err := service.ListRegions(context.Background(), &RegionsRequest{}); err == nil {
					t.Error("ожидалась ошибка")
				}
			})
		}
	})
}

// ========================
// Печать: PrintBarcode/PrintWaybill → Download
// ========================

// testPDF — минимальное содержимое PDF для проверки байтов
var testPDF = []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n1 0 obj<</Type/Catalog>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF")

func TestService_PrintAndDownloadBarcode(t *testing.T) {
	const printUUID = "a3a08b46-8004-4b69-a86e-3c074a4d3f1e"

	t.Run("цепочка: create print → download PDF", func(t *testing.T) {
		var printBody map[string]interface{}
		var downloadPath string
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/v2/print/barcodes":
				printBody = readJSONBody(t, r)
				respondJSON(w, `{"entity":{"uuid":"`+printUUID+`","url":"https://api.cdek.ru/v2/print/barcodes/`+printUUID+`.pdf"}}`)
			case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/print/barcodes/") && strings.HasSuffix(r.URL.Path, ".pdf"):
				downloadPath = r.URL.Path
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write(testPDF)
			default:
				t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		})

		printResp, err := service.PrintBarcode(context.Background(), &PrintBarcodeRequest{
			Orders: []PrintOrder{{OrderUUID: "72753031-95f6-4e0a-8dd5-1b125f46a3e5"}, {OrderUUID: "0d4c6d51-9dbe-4f1c-a2c6-1f57e91b0d7a"}},
			Copy:   ptrInt(2),
			Format: ptrString("A6"),
		})
		if err != nil {
			t.Fatalf("PrintBarcode() error = %v", err)
		}
		if printResp.UUID != printUUID {
			t.Errorf("UUID = %q, ожидался %q", printResp.UUID, printUUID)
		}
		if printResp.URL == "" {
			t.Error("URL не разобран")
		}

		// Тело задания печати: orders[], copy_count, format
		orders, _ := printBody["orders"].([]interface{})
		if len(orders) != 2 {
			t.Fatalf("orders = %v, ожидались 2 заказа", printBody["orders"])
		}
		first, _ := orders[0].(map[string]interface{})
		if first["order_uuid"] != "72753031-95f6-4e0a-8dd5-1b125f46a3e5" {
			t.Errorf("orders[0] = %v", first)
		}
		if printBody["copy_count"] != float64(2) || printBody["format"] != "A6" {
			t.Errorf("copy_count/format = %v/%v, ожидались 2/A6", printBody["copy_count"], printBody["format"])
		}

		pdf, err := service.DownloadBarcode(context.Background(), printUUID)
		if err != nil {
			t.Fatalf("DownloadBarcode() error = %v", err)
		}
		if downloadPath != "/v2/print/barcodes/"+printUUID+".pdf" {
			t.Errorf("download path = %q", downloadPath)
		}
		if string(pdf) != string(testPDF) {
			t.Errorf("PDF = %q, ожидался %q", string(pdf), string(testPDF))
		}
	})

	t.Run("минимальное задание: только orders", func(t *testing.T) {
		var printBody map[string]interface{}
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			printBody = readJSONBody(t, r)
			respondJSON(w, `{"entity":{"uuid":"`+printUUID+`"}}`)
		})
		if _, err := service.PrintBarcode(context.Background(), &PrintBarcodeRequest{
			Orders: []PrintOrder{{OrderUUID: "u1"}},
		}); err != nil {
			t.Fatalf("PrintBarcode() error = %v", err)
		}
		if _, ok := printBody["copy_count"]; ok {
			t.Error("copy_count отправлен для nil")
		}
		if _, ok := printBody["format"]; ok {
			t.Error("format отправлен для nil")
		}
	})

	t.Run("print: HTTP-статусы → типы ошибок", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				_, err := service.PrintBarcode(context.Background(), &PrintBarcodeRequest{
					Orders: []PrintOrder{{OrderUUID: "u1"}},
				})
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})

	t.Run("download: HTTP-статусы → типы ошибок", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				_, err := service.DownloadBarcode(context.Background(), printUUID)
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})

	t.Run("print: пустое тело и битый JSON", func(t *testing.T) {
		for name, body := range map[string]string{"пустое": "", "битый": "{"} {
			t.Run(name, func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondJSON(w, body)
				})
				if _, err := service.PrintBarcode(context.Background(), &PrintBarcodeRequest{
					Orders: []PrintOrder{{OrderUUID: "u1"}},
				}); err == nil {
					t.Error("ожидалась ошибка")
				}
			})
		}
	})
}

func TestService_PrintAndDownloadWaybill(t *testing.T) {
	const printUUID = "bf9c1a63-3872-4c9c-9c94-2d32b1f0f34d"

	t.Run("цепочка: create print → download PDF", func(t *testing.T) {
		var printBody map[string]interface{}
		var downloadPath string
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/v2/print/orders":
				printBody = readJSONBody(t, r)
				respondJSON(w, `{"entity":{"uuid":"`+printUUID+`","url":"https://api.cdek.ru/v2/print/orders/`+printUUID+`.pdf"}}`)
			case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/print/orders/") && strings.HasSuffix(r.URL.Path, ".pdf"):
				downloadPath = r.URL.Path
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write(testPDF)
			default:
				t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		})

		printResp, err := service.PrintWaybill(context.Background(), &PrintWaybillRequest{
			Orders: []PrintOrder{{OrderUUID: "72753031-95f6-4e0a-8dd5-1b125f46a3e5"}},
			Copy:   ptrInt(1),
			Format: ptrString("A4"),
		})
		if err != nil {
			t.Fatalf("PrintWaybill() error = %v", err)
		}
		if printResp.UUID != printUUID {
			t.Errorf("UUID = %q, ожидался %q", printResp.UUID, printUUID)
		}

		orders, _ := printBody["orders"].([]interface{})
		if len(orders) != 1 {
			t.Fatalf("orders = %v, ожидался 1 заказ", printBody["orders"])
		}
		if printBody["copy_count"] != float64(1) || printBody["format"] != "A4" {
			t.Errorf("copy_count/format = %v/%v", printBody["copy_count"], printBody["format"])
		}

		pdf, err := service.DownloadWaybill(context.Background(), printUUID)
		if err != nil {
			t.Fatalf("DownloadWaybill() error = %v", err)
		}
		if downloadPath != "/v2/print/orders/"+printUUID+".pdf" {
			t.Errorf("download path = %q", downloadPath)
		}
		if string(pdf) != string(testPDF) {
			t.Error("PDF-байты не совпали")
		}
	})

	t.Run("print: HTTP-статусы → типы ошибок", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				_, err := service.PrintWaybill(context.Background(), &PrintWaybillRequest{
					Orders: []PrintOrder{{OrderUUID: "u1"}},
				})
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})

	t.Run("download: HTTP-статусы → типы ошибок", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				_, err := service.DownloadWaybill(context.Background(), printUUID)
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})

	t.Run("print: пустое тело и битый JSON", func(t *testing.T) {
		for name, body := range map[string]string{"пустое": "", "битый": "{"} {
			t.Run(name, func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondJSON(w, body)
				})
				if _, err := service.PrintWaybill(context.Background(), &PrintWaybillRequest{
					Orders: []PrintOrder{{OrderUUID: "u1"}},
				}); err == nil {
					t.Error("ожидалась ошибка")
				}
			})
		}
	})
}

// ========================
// Intakes: Create / Get / Delete
// ========================

func TestService_Intakes(t *testing.T) {
	const intakeUUID = "d0a25b10-6f59-4a2d-8e1f-0a72c1d1a7c5"
	const intakePath = "/v2/intakes/" + intakeUUID

	newIntakeRequest := func() *IntakeRequest {
		return &IntakeRequest{
			IntakeDate:     "2026-10-20",
			IntakeTimeFrom: "10:00",
			IntakeTimeTo:   "18:00",
			Sender:         Contact{Name: "ООО «Отправитель»", Phones: []Phone{{Number: "+79007778899"}}},
		}
	}

	t.Run("CreateIntake: тело и uuid ответа", func(t *testing.T) {
		var gotMethod, gotPath string
		var gotBody map[string]interface{}
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/v2/intakes" {
				gotMethod, gotPath = r.Method, r.URL.Path
				gotBody = readJSONBody(t, r)
				respondJSON(w, `{"entity":{"uuid":"`+intakeUUID+`"}}`)
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		resp, err := service.CreateIntake(context.Background(), newIntakeRequest())
		if err != nil {
			t.Fatalf("CreateIntake() error = %v", err)
		}
		if resp.UUID != intakeUUID {
			t.Errorf("UUID = %q, ожидался %q", resp.UUID, intakeUUID)
		}
		_ = gotMethod
		_ = gotPath
		if gotBody["intake_date"] != "2026-10-20" || gotBody["intake_time_from"] != "10:00" || gotBody["intake_time_to"] != "18:00" {
			t.Errorf("тело = %v", gotBody)
		}
		sender, _ := gotBody["sender"].(map[string]interface{})
		if sender == nil || sender["name"] != "ООО «Отправитель»" {
			t.Errorf("sender = %v", gotBody["sender"])
		}
	})

	t.Run("GetIntake: GET /v2/intakes/{uuid}", func(t *testing.T) {
		var gotPath string
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == intakePath {
				gotPath = r.URL.Path
				respondJSON(w, `{"entity":{"uuid":"`+intakeUUID+`"}}`)
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		info, err := service.GetIntake(context.Background(), intakeUUID)
		if err != nil {
			t.Fatalf("GetIntake() error = %v", err)
		}
		if gotPath != intakePath {
			t.Errorf("path = %q, ожидался %q", gotPath, intakePath)
		}
		if info.UUID != intakeUUID {
			t.Errorf("UUID = %q", info.UUID)
		}
	})

	t.Run("DeleteIntake: DELETE /v2/intakes/{uuid}", func(t *testing.T) {
		var gotMethod, gotPath string
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete && r.URL.Path == intakePath {
				gotMethod, gotPath = r.Method, r.URL.Path
				respondJSON(w, `{"requests":[{"state":"SUCCESSFUL"}]}`)
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		if err := service.DeleteIntake(context.Background(), intakeUUID); err != nil {
			t.Fatalf("DeleteIntake() error = %v", err)
		}
		if gotMethod != http.MethodDelete || gotPath != intakePath {
			t.Errorf("запрос = %s %s", gotMethod, gotPath)
		}
	})

	t.Run("HTTP-статусы → типы ошибок для всех трех методов", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})

				_, err := service.CreateIntake(context.Background(), newIntakeRequest())
				assertSentinel(t, err, statusSentinel(code))

				// Каждый вызов должен пройти через собственный breaker
				service2 := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				_, err = service2.GetIntake(context.Background(), intakeUUID)
				assertSentinel(t, err, statusSentinel(code))

				service3 := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
					respondError(w, code)
				})
				err = service3.DeleteIntake(context.Background(), intakeUUID)
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})
}

// ========================
// Ошибки конвейера: токен, транспорт, обрыв тела
// ========================

// newTokenErrorService поднимает сервер, у которого падает только токен-эндпоинт
func newTokenErrorService(t *testing.T) *Service {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		respondError(w, http.StatusInternalServerError)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	config := &AccountConfig{
		Name: "test", ClientID: "id", ClientSecret: "secret",
		BaseURL: server.URL, Timeout: 5 * time.Second,
	}
	client, err := NewAuthenticatedClient(config)
	if err != nil {
		t.Fatalf("NewAuthenticatedClient() error = %v", err)
	}
	return NewService(client, nil)
}

// validCostRequest — валидный запрос тарифов
func validCostRequest() *CostRequest {
	return &CostRequest{
		FromCityCode: 44,
		ToCityCode:   250,
		Packages:     []Package{{Weight: 1500, Length: 10, Width: 20, Height: 30}},
	}
}

// TestService_TokenFailure: отказ токен-эндпоинта → каждый метод возвращает
// ошибку «get token», запрос в API не уходит (отдельный breaker на метод)
func TestService_TokenFailure(t *testing.T) {
	ctx := context.Background()
	calls := map[string]func(s *Service) error{
		"CalculateCost": func(s *Service) error {
			_, err := s.CalculateCost(ctx, validCostRequest())
			return err
		},
		"CreateOrder": func(s *Service) error {
			_, err := s.CreateOrder(ctx, baseOrderRequest())
			return err
		},
		"TrackOrder": func(s *Service) error {
			_, err := s.TrackOrder(ctx, "u1")
			return err
		},
		"ListDeliveryPoints": func(s *Service) error {
			_, err := s.ListDeliveryPoints(ctx, &DeliveryPointsRequest{CityCode: "44"})
			return err
		},
		"PrintBarcode": func(s *Service) error {
			_, err := s.PrintBarcode(ctx, &PrintBarcodeRequest{Orders: []PrintOrder{{OrderUUID: "u1"}}})
			return err
		},
		"PrintWaybill": func(s *Service) error {
			_, err := s.PrintWaybill(ctx, &PrintWaybillRequest{Orders: []PrintOrder{{OrderUUID: "u1"}}})
			return err
		},
		"GetOrder": func(s *Service) error {
			_, err := s.GetOrder(ctx, "u1")
			return err
		},
		"GetOrderByCDEKNumber": func(s *Service) error {
			_, err := s.GetOrderByCDEKNumber(ctx, "1")
			return err
		},
		"UpdateOrder": func(s *Service) error {
			_, err := s.UpdateOrder(ctx, &UpdateOrderRequest{OrderUUID: "u1"})
			return err
		},
		"CancelOrder": func(s *Service) error {
			return s.CancelOrder(ctx, "u1")
		},
		"DownloadBarcode": func(s *Service) error {
			_, err := s.DownloadBarcode(ctx, "p1")
			return err
		},
		"DownloadWaybill": func(s *Service) error {
			_, err := s.DownloadWaybill(ctx, "p1")
			return err
		},
		"ListCities": func(s *Service) error {
			_, err := s.ListCities(ctx, &CitiesRequest{})
			return err
		},
		"ListRegions": func(s *Service) error {
			_, err := s.ListRegions(ctx, &RegionsRequest{})
			return err
		},
		"CreateIntake": func(s *Service) error {
			_, err := s.CreateIntake(ctx, &IntakeRequest{
				IntakeDate: "2026-10-20", IntakeTimeFrom: "10:00", IntakeTimeTo: "18:00",
				Sender: Contact{Name: "ООО «Отправитель»"},
			})
			return err
		},
		"GetIntake": func(s *Service) error {
			_, err := s.GetIntake(ctx, "i1")
			return err
		},
		"DeleteIntake": func(s *Service) error {
			return s.DeleteIntake(ctx, "i1")
		},
		"CreateWebhook": func(s *Service) error {
			_, err := s.CreateWebhook(ctx, &WebhookRequest{URL: "https://x", Type: "ORDER_STATUS"})
			return err
		},
		"ListWebhooks": func(s *Service) error {
			_, err := s.ListWebhooks(ctx)
			return err
		},
		"GetWebhook": func(s *Service) error {
			_, err := s.GetWebhook(ctx, "w1")
			return err
		},
		"DeleteWebhook": func(s *Service) error {
			return s.DeleteWebhook(ctx, "w1")
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			// Свежий Service на каждый метод: собственный circuit breaker
			err := call(newTokenErrorService(t))
			if err == nil {
				t.Fatal("ожидалась ошибка токена, получен nil")
			}
			if !strings.Contains(err.Error(), "get token") {
				t.Errorf("error = %v, ожидалась обертка get token", err)
			}
		})
	}
}

// TestService_TransportFailure: токен взят из кеша, сервер закрыт → ошибка
// «api call» без обращения к сети
func TestService_TransportFailure(t *testing.T) {
	ctx := context.Background()

	// Живой сервер только для кеша токена, затем закрываем
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		respondError(w, http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	config := &AccountConfig{
		Name: "test", ClientID: "id", ClientSecret: "secret",
		BaseURL: server.URL, Timeout: 5 * time.Second,
	}
	client, err := NewAuthenticatedClient(config)
	if err != nil {
		t.Fatalf("NewAuthenticatedClient() error = %v", err)
	}
	if _, err := client.GetToken(ctx); err != nil {
		t.Fatalf("GetToken() warmup error = %v", err)
	}
	server.Close() // дальше все вызовы падают на транспорте

	calls := map[string]func(s *Service) error{
		"CalculateCost": func(s *Service) error {
			_, err := s.CalculateCost(ctx, validCostRequest())
			return err
		},
		"CreateOrder": func(s *Service) error {
			_, err := s.CreateOrder(ctx, baseOrderRequest())
			return err
		},
		"TrackOrder": func(s *Service) error {
			_, err := s.TrackOrder(ctx, "u1")
			return err
		},
		"GetOrderByCDEKNumber": func(s *Service) error {
			_, err := s.GetOrderByCDEKNumber(ctx, "1")
			return err
		},
		"GetOrder": func(s *Service) error {
			_, err := s.GetOrder(ctx, "u1")
			return err
		},
		"UpdateOrder": func(s *Service) error {
			_, err := s.UpdateOrder(ctx, &UpdateOrderRequest{OrderUUID: "u1"})
			return err
		},
		"CancelOrder": func(s *Service) error {
			return s.CancelOrder(ctx, "u1")
		},
		"ListDeliveryPoints": func(s *Service) error {
			_, err := s.ListDeliveryPoints(ctx, &DeliveryPointsRequest{CityCode: "44"})
			return err
		},
		"ListCities": func(s *Service) error {
			_, err := s.ListCities(ctx, &CitiesRequest{})
			return err
		},
		"ListRegions": func(s *Service) error {
			_, err := s.ListRegions(ctx, &RegionsRequest{})
			return err
		},
		"PrintBarcode": func(s *Service) error {
			_, err := s.PrintBarcode(ctx, &PrintBarcodeRequest{Orders: []PrintOrder{{OrderUUID: "u1"}}})
			return err
		},
		"PrintWaybill": func(s *Service) error {
			_, err := s.PrintWaybill(ctx, &PrintWaybillRequest{Orders: []PrintOrder{{OrderUUID: "u1"}}})
			return err
		},
		"DownloadBarcode": func(s *Service) error {
			_, err := s.DownloadBarcode(ctx, "p1")
			return err
		},
		"DownloadWaybill": func(s *Service) error {
			_, err := s.DownloadWaybill(ctx, "p1")
			return err
		},
		"CreateIntake": func(s *Service) error {
			_, err := s.CreateIntake(ctx, &IntakeRequest{
				IntakeDate: "2026-10-20", IntakeTimeFrom: "10:00", IntakeTimeTo: "18:00",
				Sender: Contact{Name: "ООО «Отправитель»"},
			})
			return err
		},
		"GetIntake": func(s *Service) error {
			_, err := s.GetIntake(ctx, "i1")
			return err
		},
		"DeleteIntake": func(s *Service) error {
			return s.DeleteIntake(ctx, "i1")
		},
		"CreateWebhook": func(s *Service) error {
			_, err := s.CreateWebhook(ctx, &WebhookRequest{URL: "https://x", Type: "ORDER_STATUS"})
			return err
		},
		"ListWebhooks": func(s *Service) error {
			_, err := s.ListWebhooks(ctx)
			return err
		},
		"GetWebhook": func(s *Service) error {
			_, err := s.GetWebhook(ctx, "w1")
			return err
		},
		"DeleteWebhook": func(s *Service) error {
			return s.DeleteWebhook(ctx, "w1")
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call(NewService(client, nil))
			if err == nil {
				t.Fatal("ожидалась транспортная ошибка, получен nil")
			}
			if !strings.Contains(err.Error(), "api call") {
				t.Errorf("error = %v, ожидалась обертка api call", err)
			}
		})
	}
}

// TestService_TruncatedBody: сервер рвет соединение посреди тела с
// Content-Length больше факта → ошибка чтения тела у методов с сырым ответом
func TestService_TruncatedBody(t *testing.T) {
	ctx := context.Background()

	newTruncatingService := func(t *testing.T) *Service {
		t.Helper()
		mux := http.NewServeMux()
		mux.HandleFunc("/v2/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
			respondJSON(w, `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`)
		})
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			// Заявлен Content-Length 1000, отправлено 5 байт, соединение закрыто
			_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\nshort"))
			_ = conn.Close()
		})
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		config := &AccountConfig{
			Name: "test", ClientID: "id", ClientSecret: "secret",
			BaseURL: server.URL, Timeout: 5 * time.Second,
		}
		client, err := NewAuthenticatedClient(config)
		if err != nil {
			t.Fatalf("NewAuthenticatedClient() error = %v", err)
		}
		return NewService(client, nil)
	}

	calls := map[string]func(s *Service) error{
		"CalculateCost": func(s *Service) error {
			_, err := s.CalculateCost(ctx, validCostRequest())
			return err
		},
		"CreateOrder": func(s *Service) error {
			_, err := s.CreateOrder(ctx, baseOrderRequest())
			return err
		},
		"TrackOrder": func(s *Service) error {
			_, err := s.TrackOrder(ctx, "u1")
			return err
		},
		"ListDeliveryPoints": func(s *Service) error {
			_, err := s.ListDeliveryPoints(ctx, &DeliveryPointsRequest{CityCode: "44"})
			return err
		},
		"GetOrderByCDEKNumber": func(s *Service) error {
			_, err := s.GetOrderByCDEKNumber(ctx, "1")
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call(newTruncatingService(t))
			if err == nil {
				t.Fatal("ожидалась ошибка чтения тела, получен nil")
			}
		})
	}
}

// TestService_CreateUpdateOrderStatuses: 4xx/5xx у создания и обновления
// (ветка пересборки http.Response из прочитанного тела)
func TestService_CreateUpdateOrderStatuses(t *testing.T) {
	for _, code := range errorStatusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
				respondError(w, code)
			})
			_, err := service.CreateOrder(context.Background(), baseOrderRequest())
			assertSentinel(t, err, statusSentinel(code))

			service2 := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
				respondError(w, code)
			})
			_, err = service2.UpdateOrder(context.Background(), &UpdateOrderRequest{
				OrderUUID: "u1", Type: "1",
				Recipient: &Recipient{Contact: Contact{Name: "П", Phones: []Phone{{Number: "+79000000000"}}}},
			})
			assertSentinel(t, err, statusSentinel(code))
		})
	}
}

// TestService_GetOrderStatuses: 4xx/5xx у GetOrder
func TestService_GetOrderStatuses(t *testing.T) {
	for _, code := range errorStatusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
				respondError(w, code)
			})
			_, err := service.GetOrder(context.Background(), "u1")
			assertSentinel(t, err, statusSentinel(code))
		})
	}
}

// TestService_ListDeliveryPointsStatuses: сырой путь Do → wrapHTTPError
func TestService_ListDeliveryPointsStatuses(t *testing.T) {
	for _, code := range errorStatusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
				respondError(w, code)
			})
			_, err := service.ListDeliveryPoints(context.Background(), &DeliveryPointsRequest{CityCode: "44"})
			assertSentinel(t, err, statusSentinel(code))
		})
	}
}

// ========================
// Webhooks: Create / List / Get / Delete
// ========================

func TestService_Webhooks(t *testing.T) {
	const webhookUUID = "f55d7a77-3f57-4a48-a2bb-c1e77b6a0eb9"
	const webhookPath = "/v2/webhooks/" + webhookUUID

	t.Run("CreateWebhook: url и type в теле, uuid в ответе", func(t *testing.T) {
		var gotBody map[string]interface{}
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/v2/webhooks" {
				gotBody = readJSONBody(t, r)
				respondJSON(w, `{"entity":{"uuid":"`+webhookUUID+`"}}`)
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		resp, err := service.CreateWebhook(context.Background(), &WebhookRequest{
			URL:  "https://example.com/cdek/webhook",
			Type: WebhookTypeOrderStatus,
		})
		if err != nil {
			t.Fatalf("CreateWebhook() error = %v", err)
		}
		if resp.UUID != webhookUUID {
			t.Errorf("UUID = %q", resp.UUID)
		}
		if gotBody["url"] != "https://example.com/cdek/webhook" || gotBody["type"] != "ORDER_STATUS" {
			t.Errorf("тело = %v", gotBody)
		}
	})

	t.Run("ListWebhooks: список", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/v2/webhooks" {
				respondJSON(w, `[
					{"uuid":"`+webhookUUID+`","url":"https://example.com/cdek/webhook","type":"ORDER_STATUS"},
					{"uuid":"2c5e7a9c-1d34-4a25-9e70-9e2f6e2a5a10","url":"https://example.com/cdek/print","type":"PRINT_FORM"}
				]`)
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		webhooks, err := service.ListWebhooks(context.Background())
		if err != nil {
			t.Fatalf("ListWebhooks() error = %v", err)
		}
		if len(webhooks) != 2 || webhooks[0].UUID != webhookUUID || webhooks[0].Type != "ORDER_STATUS" {
			t.Errorf("webhooks = %+v", webhooks)
		}
		if webhooks[1].Type != "PRINT_FORM" {
			t.Errorf("webhooks[1].Type = %q", webhooks[1].Type)
		}
	})

	t.Run("ListWebhooks: пустой список", func(t *testing.T) {
		service := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
			respondJSON(w, `[]`)
		})
		webhooks, err := service.ListWebhooks(context.Background())
		if err != nil {
			t.Fatalf("ListWebhooks() error = %v", err)
		}
		if len(webhooks) != 0 {
			t.Errorf("len(webhooks) = %d, ожидался 0", len(webhooks))
		}
	})

	t.Run("GetWebhook: entity с url и type", func(t *testing.T) {
		var gotPath string
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == webhookPath {
				gotPath = r.URL.Path
				respondJSON(w, `{"entity":{"uuid":"`+webhookUUID+`","url":"https://example.com/cdek/webhook","type":"ORDER_STATUS"}}`)
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		webhook, err := service.GetWebhook(context.Background(), webhookUUID)
		if err != nil {
			t.Fatalf("GetWebhook() error = %v", err)
		}
		if gotPath != webhookPath {
			t.Errorf("path = %q", gotPath)
		}
		if webhook.UUID != webhookUUID || webhook.URL != "https://example.com/cdek/webhook" || webhook.Type != "ORDER_STATUS" {
			t.Errorf("webhook = %+v", webhook)
		}
	})

	t.Run("DeleteWebhook: DELETE /v2/webhooks/{uuid}", func(t *testing.T) {
		var gotMethod, gotPath string
		service := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete && r.URL.Path == webhookPath {
				gotMethod, gotPath = r.Method, r.URL.Path
				respondJSON(w, `{"requests":[{"state":"SUCCESSFUL"}]}`)
				return
			}
			t.Errorf("неожидаемый запрос: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})

		if err := service.DeleteWebhook(context.Background(), webhookUUID); err != nil {
			t.Fatalf("DeleteWebhook() error = %v", err)
		}
		if gotMethod != http.MethodDelete || gotPath != webhookPath {
			t.Errorf("запрос = %s %s", gotMethod, gotPath)
		}
	})

	t.Run("HTTP-статусы → типы ошибок для всех четырех методов", func(t *testing.T) {
		for _, code := range errorStatusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				newSvc := func() *Service {
					return newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
						respondError(w, code)
					})
				}

				_, err := newSvc().CreateWebhook(context.Background(), &WebhookRequest{URL: "https://x", Type: "ORDER_STATUS"})
				assertSentinel(t, err, statusSentinel(code))

				_, err = newSvc().ListWebhooks(context.Background())
				assertSentinel(t, err, statusSentinel(code))

				_, err = newSvc().GetWebhook(context.Background(), webhookUUID)
				assertSentinel(t, err, statusSentinel(code))

				err = newSvc().DeleteWebhook(context.Background(), webhookUUID)
				assertSentinel(t, err, statusSentinel(code))
			})
		}
	})
}
