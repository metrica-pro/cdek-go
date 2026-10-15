package cdek

// client_endpoints_test.go — контрактный smoke сгенерированного oapi-codegen
// клиента: каждый эндпоинт спеки собирает валидный HTTP-запрос (метод+путь
// совпадают со спецификацией) и не падает на разборе ответа. Держит
// суммарное покрытие pkg/cdek с учетом сгенерированного client.go и ловит
// дрейф кодогенерации.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// filledParams возвращает params с заполненными полями-указателями и
// interface{}-полями (заголовки без nil-guard в сгенерированном коде),
// чтобы ветки кодирования параметров выполнялись
func filledParams[P any](t *testing.T, params *P) *P {
	t.Helper()
	elem := reflect.ValueOf(params).Elem()
	if elem.Kind() != reflect.Struct {
		t.Fatalf("filledParams: ожидалась структура, получен %v", elem.Kind())
	}
	for i := 0; i < elem.NumField(); i++ {
		field := elem.Field(i)
		if !field.CanSet() {
			continue
		}
		switch field.Kind() { //nolint:exhaustive // обрабатываются только поля-параметры
		case reflect.Pointer:
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
		case reflect.Interface:
			if field.IsNil() {
				field.Set(reflect.ValueOf(""))
			}
		}
	}
	return params
}

// TestGeneratedClientEndpoints: метод+путь каждого эндпоинта против httptest
func TestGeneratedClientEndpoints(t *testing.T) {
	const uuidPath = "00000000-0000-4000-8000-000000000099"

	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		respondJSON(w, `{}`)
	}))
	t.Cleanup(server.Close)

	client, err := NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses() error = %v", err)
	}
	ctx := context.Background()
	jsonBody := io.NopCloser(strings.NewReader(`{}`))

	tests := []struct {
		name        string
		wantMethod  string
		wantPath    string
		wantPathFmt bool // путь форматируется подстановкой uuid
		call        func() error
	}{
		// Calculator
		{name: "AvailableTariffs", wantMethod: http.MethodGet, wantPath: "/v2/calculator/alltariffs",
			call: func() error {
				_, err := client.AvailableTariffsWithResponse(ctx, filledParams(t, &AvailableTariffsParams{}))
				return err
			}},
		{name: "TariffWithBody", wantMethod: http.MethodPost, wantPath: "/v2/calculator/tariff",
			call: func() error {
				p := filledParams(t, &TariffParams{})
				_, err := client.TariffWithBodyWithResponse(ctx, p, "application/json", strings.NewReader(`{}`))
				return err
			}},
		{name: "TariffWithServicesWithBody", wantMethod: http.MethodPost, wantPath: "/v2/calculator/tariffAndService",
			call: func() error {
				_, err := client.TariffWithServicesWithBodyWithResponse(ctx, filledParams(t, &TariffWithServicesParams{}), "application/json", jsonBody)
				return err
			}},
		{name: "TariffListWithBody", wantMethod: http.MethodPost, wantPath: "/v2/calculator/tarifflist",
			call: func() error {
				_, err := client.TariffListWithBodyWithResponse(ctx, filledParams(t, &TariffListParams{}), "application/json", jsonBody)
				return err
			}},
		// Check
		{name: "GetCheck", wantMethod: http.MethodGet, wantPath: "/v2/check",
			call: func() error {
				_, err := client.GetCheckWithResponse(ctx, filledParams(t, &GetCheckParams{}))
				return err
			}},
		// Delivery (курьерские слоты)
		{name: "CreateDelivery", wantMethod: http.MethodPost, wantPath: "/v2/delivery",
			call: func() error {
				_, err := client.CreateDeliveryWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		{name: "GetEstimatedIntervals", wantMethod: http.MethodPost, wantPath: "/v2/delivery/estimatedIntervals",
			call: func() error {
				_, err := client.GetEstimatedIntervalsWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		{name: "GetIntervals", wantMethod: http.MethodGet, wantPath: "/v2/delivery/intervals",
			call: func() error {
				_, err := client.GetIntervalsWithResponse(ctx, filledParams(t, &GetIntervalsParams{}))
				return err
			}},
		{name: "GetDelivery", wantMethod: http.MethodGet, wantPath: "/v2/delivery/" + uuidPath,
			call: func() error { _, err := client.GetDeliveryWithResponse(ctx, uuidPath); return err }},
		// Delivery points
		{name: "GetDeliverypoints", wantMethod: http.MethodGet, wantPath: "/v2/deliverypoints",
			call: func() error {
				_, err := client.GetDeliverypointsWithResponse(ctx, filledParams(t, &GetDeliverypointsParams{}))
				return err
			}},
		// Intakes
		{name: "ChangeStatus", wantMethod: http.MethodPatch, wantPath: "/v2/intakes",
			call: func() error {
				_, err := client.ChangeStatusWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		{name: "CreateIntake", wantMethod: http.MethodPost, wantPath: "/v2/intakes",
			call: func() error {
				_, err := client.CreateIntakeWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		{name: "GetAvailableDays", wantMethod: http.MethodPost, wantPath: "/v2/intakes/availableDays",
			call: func() error {
				_, err := client.GetAvailableDaysWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		{name: "DeleteIntakeByUuid", wantMethod: http.MethodDelete, wantPath: "/v2/intakes/" + uuidPath,
			call: func() error { _, err := client.DeleteByUuidWithResponse(ctx, uuidPath); return err }},
		{name: "GetIntakeByUuid", wantMethod: http.MethodGet, wantPath: "/v2/intakes/" + uuidPath,
			call: func() error { _, err := client.GetByUuidWithResponse(ctx, uuidPath); return err }},
		{name: "CheckPackagesRestrictions", wantMethod: http.MethodPost, wantPath: "/v2/international/package/restrictions",
			call: func() error {
				_, err := client.CheckPackagesRestrictionsWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		// Location reference
		{name: "Cities", wantMethod: http.MethodGet, wantPath: "/v2/location/cities",
			call: func() error { _, err := client.CitiesWithResponse(ctx, filledParams(t, &CitiesParams{})); return err }},
		{name: "GetCityByCoordinates", wantMethod: http.MethodGet, wantPath: "/v2/location/coordinates",
			call: func() error {
				_, err := client.GetCityByCoordinatesWithResponse(ctx, filledParams(t, &GetCityByCoordinatesParams{}))
				return err
			}},
		{name: "Postalcodes", wantMethod: http.MethodGet, wantPath: "/v2/location/postalcodes",
			call: func() error {
				_, err := client.PostalcodesWithResponse(ctx, filledParams(t, &PostalcodesParams{}))
				return err
			}},
		{name: "Regions", wantMethod: http.MethodGet, wantPath: "/v2/location/regions",
			call: func() error { _, err := client.RegionsWithResponse(ctx, filledParams(t, &RegionsParams{})); return err }},
		{name: "SuggestCities", wantMethod: http.MethodGet, wantPath: "/v2/location/suggest/cities",
			call: func() error {
				_, err := client.SuggestCitiesWithResponse(ctx, filledParams(t, &SuggestCitiesParams{}))
				return err
			}},
		// OAuth
		{name: "GetOAuthTokenWithBody", wantMethod: http.MethodPost, wantPath: "/v2/oauth/token",
			call: func() error {
				_, err := client.GetOAuthTokenWithBodyWithResponse(ctx, "application/x-www-form-urlencoded", strings.NewReader(`grant_type=client_credentials`))
				return err
			}},
		{name: "GetOAuthTokenWithFormdataBody", wantMethod: http.MethodPost, wantPath: "/v2/oauth/token",
			call: func() error {
				_, err := client.GetOAuthTokenWithFormdataBodyWithResponse(ctx, GetOAuthTokenFormdataRequestBody{
					GrantType:    GetOAuthTokenFormdataBodyGrantTypeClientCredentials,
					ClientId:     "id",
					ClientSecret: "secret",
				})
				return err
			}},
		// Orders
		{name: "GetOrders", wantMethod: http.MethodGet, wantPath: "/v2/orders",
			call: func() error {
				_, err := client.GetOrdersWithResponse(ctx, filledParams(t, &GetOrdersParams{}))
				return err
			}},
		{name: "UpdateOrder", wantMethod: http.MethodPatch, wantPath: "/v2/orders",
			call: func() error {
				_, err := client.UpdateWithBodyWithResponse(ctx, filledParams(t, &UpdateParams{}), "application/json", jsonBody)
				return err
			}},
		{name: "CreateOrder", wantMethod: http.MethodPost, wantPath: "/v2/orders",
			call: func() error {
				_, err := client.CreateOrderWithBodyWithResponse(ctx, filledParams(t, &CreateOrderParams{}), "application/json", jsonBody)
				return err
			}},
		{name: "GetIntakesByOrder", wantMethod: http.MethodGet, wantPath: "/v2/orders/" + uuidPath + "/intakes",
			call: func() error {
				_, err := client.GetIntakesWithResponse(ctx, uuidPath, filledParams(t, &GetIntakesParams{}))
				return err
			}},
		{name: "DeleteOrder", wantMethod: http.MethodDelete, wantPath: "/v2/orders/" + uuidPath,
			call: func() error { _, err := client.DeleteWithResponse(ctx, uuidPath); return err }},
		{name: "GetOrder", wantMethod: http.MethodGet, wantPath: "/v2/orders/" + uuidPath,
			call: func() error { _, err := client.GetOrderWithResponse(ctx, uuidPath); return err }},
		{name: "ClientReturn", wantMethod: http.MethodPost, wantPath: "/v2/orders/" + uuidPath + "/clientReturn",
			call: func() error {
				_, err := client.ClientReturnWithBodyWithResponse(ctx, uuidPath, "application/json", jsonBody)
				return err
			}},
		{name: "Refuse", wantMethod: http.MethodPost, wantPath: "/v2/orders/" + uuidPath + "/refusal",
			call: func() error { _, err := client.RefuseWithResponse(ctx, uuidPath); return err }},
		{name: "GetPassport", wantMethod: http.MethodGet, wantPath: "/v2/passport",
			call: func() error {
				_, err := client.GetPassportWithResponse(ctx, filledParams(t, &GetPassportParams{}))
				return err
			}},
		{name: "GetPayment", wantMethod: http.MethodGet, wantPath: "/v2/payment",
			call: func() error {
				_, err := client.GetPaymentWithResponse(ctx, filledParams(t, &GetPaymentParams{}))
				return err
			}},
		// Photo documents / prealert / print
		{name: "GetReadyOrders", wantMethod: http.MethodPost, wantPath: "/v2/photoDocument",
			call: func() error {
				_, err := client.GetReadyOrdersWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		{name: "Register", wantMethod: http.MethodPost, wantPath: "/v2/prealert",
			call: func() error {
				_, err := client.RegisterWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		{name: "GetPrealert", wantMethod: http.MethodGet, wantPath: "/v2/prealert/" + uuidPath,
			call: func() error { _, err := client.GetPrealertWithResponse(ctx, uuidPath); return err }},
		{name: "BarcodePrint", wantMethod: http.MethodPost, wantPath: "/v2/print/barcodes",
			call: func() error {
				_, err := client.BarcodePrintWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		{name: "BarcodeGet", wantMethod: http.MethodGet, wantPath: "/v2/print/barcodes/" + uuidPath,
			call: func() error { _, err := client.BarcodeGetWithResponse(ctx, uuidPath); return err }},
		{name: "BarcodeDownload", wantMethod: http.MethodGet, wantPath: "/v2/print/barcodes/" + uuidPath + ".pdf",
			call: func() error { _, err := client.BarcodeDownloadWithResponse(ctx, uuidPath); return err }},
		{name: "WaybillPrint", wantMethod: http.MethodPost, wantPath: "/v2/print/orders",
			call: func() error {
				_, err := client.WaybillPrintWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		{name: "WaybillGet", wantMethod: http.MethodGet, wantPath: "/v2/print/orders/" + uuidPath,
			call: func() error { _, err := client.WaybillGetWithResponse(ctx, uuidPath); return err }},
		{name: "WaybillDownload", wantMethod: http.MethodGet, wantPath: "/v2/print/orders/" + uuidPath + ".pdf",
			call: func() error { _, err := client.WaybillDownloadWithResponse(ctx, uuidPath); return err }},
		{name: "GetRegistries", wantMethod: http.MethodGet, wantPath: "/v2/registries",
			call: func() error {
				_, err := client.GetRegistriesWithResponse(ctx, filledParams(t, &GetRegistriesParams{}))
				return err
			}},
		{name: "CheckReverseAvailability", wantMethod: http.MethodPost, wantPath: "/v2/reverse/availability",
			call: func() error {
				_, err := client.CheckAvailabilityWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		// Webhooks
		{name: "GetAllWebhooks", wantMethod: http.MethodGet, wantPath: "/v2/webhooks",
			call: func() error { _, err := client.GetAllWithResponse(ctx); return err }},
		{name: "CreateWebhook", wantMethod: http.MethodPost, wantPath: "/v2/webhooks",
			call: func() error {
				_, err := client.CreateV2WebhooksWithBodyWithResponse(ctx, "application/json", jsonBody)
				return err
			}},
		{name: "DeleteWebhook", wantMethod: http.MethodDelete, wantPath: "/v2/webhooks/" + uuidPath,
			call: func() error { _, err := client.DeleteByIdWithResponse(ctx, uuidPath); return err }},
		{name: "GetWebhook", wantMethod: http.MethodGet, wantPath: "/v2/webhooks/" + uuidPath,
			call: func() error { _, err := client.GetByIdWithResponse(ctx, uuidPath); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMethod, gotPath = "", ""
			if err := tt.call(); err != nil {
				t.Fatalf("%s() error = %v", tt.name, err)
			}
			if gotMethod != tt.wantMethod {
				t.Errorf("метод = %s, ожидался %s", gotMethod, tt.wantMethod)
			}
			if gotPath != tt.wantPath {
				t.Errorf("путь = %s, ожидался %s", gotPath, tt.wantPath)
			}
		})
	}
}

// TestGeneratedClientParamsEncoding: заполненные параметры попадают в query
func TestGeneratedClientParamsEncoding(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		respondJSON(w, `[]`)
	}))
	t.Cleanup(server.Close)

	client, err := NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses() error = %v", err)
	}

	t.Run("Cities: все параметры кодируются", func(t *testing.T) {
		params := &CitiesParams{
			CountryCodes: ptrString("RU"),
			RegionCode:   ptrInt32(81),
			PostalCode:   ptrString("109544"),
			Code:         ptrInt32(44),
			City:         ptrString("Москва"),
		}
		filledParams(t, params)
		if _, err := client.CitiesWithResponse(context.Background(), params); err != nil {
			t.Fatalf("CitiesWithResponse() error = %v", err)
		}
		for _, want := range []string{"country_codes=RU", "region_code=81", "postal_code=109544", "code=44", "city=%D0%9C%D0%BE%D1%81%D0%BA%D0%B2%D0%B0"} {
			if !strings.Contains(gotQuery, want) {
				t.Errorf("query %s не содержит %s", gotQuery, want)
			}
		}
	})

	t.Run("Deliverypoints: фильтры кодируются", func(t *testing.T) {
		params := &GetDeliverypointsParams{
			CityCode:       ptrString("44"),
			Type:           ptrString("PVZ"),
			HaveCashless:   ptrBool(true),
			IsDressingRoom: ptrBool(false),
		}
		filledParams(t, params)
		if _, err := client.GetDeliverypointsWithResponse(context.Background(), params); err != nil {
			t.Fatalf("GetDeliverypoints() error = %v", err)
		}
		if !strings.Contains(gotQuery, "city_code=44") || !strings.Contains(gotQuery, "type=PVZ") {
			t.Errorf("query = %s, ожидались city_code и type", gotQuery)
		}
	})
}
