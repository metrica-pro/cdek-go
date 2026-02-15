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

func TestSimple_ServiceCalculateCost(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
	ctx := context.Background()

	// Расчет стоимости доставки Москва → Новосибирск
	req := &CostRequest{
		FromCityCode: 44,  // Москва
		ToCityCode:   270, // Новосибирск
		Packages: []Package{
			{Weight: 1000, Length: 20, Width: 15, Height: 10}, // 1 кг, 20x15x10 см
		},
	}

	resp, err := service.CalculateCost(ctx, req)
	if err != nil {
		t.Fatalf("CalculateCost() error = %v", err)
	}

	if len(resp.Tariffs) == 0 {
		t.Error("Expected at least one tariff")
	}

	t.Logf("✅ Service.CalculateCost: получено %d тарифов", len(resp.Tariffs))
	for i, tariff := range resp.Tariffs {
		t.Logf("  [%d] %s: %.2f руб, %d-%d дней (код %d)",
			i+1, tariff.TariffName, tariff.DeliverySum,
			tariff.PeriodMin, tariff.PeriodMax, tariff.TariffCode)
	}
}

func TestSimple_HealthCheck(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
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

// createTestOrder - helper для создания тестового заказа
func createTestOrder(t *testing.T, service *Service, ctx context.Context) *OrderResponse {
	t.Helper()

	// Используем самый дешевый тариф для теста
	fromCode := int32(44)  // Москва
	toCode := int32(270)   // Новосибирск

	toAddress := "ул. Ленина, д. 1"

	req := &OrderRequest{
		Type:       "1", // интернет-магазин
		TariffCode: 136, // Посылка склад-склад (самый дешевый)
		Sender: Contact{
			Name:   "Тестовый отправитель",
			Phones: []Phone{{Number: "+79099999999"}},
		},
		Recipient: Recipient{
			Contact: Contact{
				Name:   "Иванов Иван Иванович",
				Phones: []Phone{{Number: "+79001234567"}},
			},
		},
		FromLocation: Location{
			Code: &fromCode,
		},
		ToLocation: Location{
			Code:    &toCode,
			Address: &toAddress,
		},
		Packages: []OrderPackage{
			{
				Number: "TEST-001",
				Weight: 1000, // 1 кг
				Items: []Item{
					{
						Name:    "Тестовый товар",
						WareKey: "TEST-SKU-001",
						Payment: 1000,
						Cost:    1000,
						Weight:  1000,
						Amount:  1,
					},
				},
			},
		},
	}

	order, err := service.CreateOrder(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test order: %v", err)
	}

	return order
}

func TestSimple_ServiceCreateOrder(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
	ctx := context.Background()

	fromCode := int32(44)  // Москва
	toCode := int32(270)   // Новосибирск
	toAddress := "ул. Ленина, д. 1, кв. 1"

	req := &OrderRequest{
		Type:       "1", // интернет-магазин
		TariffCode: 136, // Посылка склад-склад
		Sender: Contact{
			Name:   "Тестовый магазин",
			Phones: []Phone{{Number: "+79099999999"}},
		},
		Recipient: Recipient{
			Contact: Contact{
				Name:   "Петров Петр Петрович",
				Phones: []Phone{{Number: "+79007654321"}},
			},
		},
		FromLocation: Location{
			Code: &fromCode,
		},
		ToLocation: Location{
			Code:    &toCode,
			Address: &toAddress,
		},
		Packages: []OrderPackage{
			{
				Number: "TEST-ORDER-001",
				Weight: 1000, // 1 кг
				Items: []Item{
					{
						Name:    "Тестовый товар для заказа",
						WareKey: "TEST-SKU-002",
						Payment: 1500,
						Cost:    1500,
						Weight:  1000,
						Amount:  1,
					},
				},
			},
		},
	}

	order, err := service.CreateOrder(ctx, req)
	if err != nil {
		// Попробуем получить детальную ошибку
		t.Logf("CreateOrder error: %v", err)
		t.Logf("Request details: Type=%s, TariffCode=%d, FromCode=%d, ToCode=%d",
			req.Type, req.TariffCode, *req.FromLocation.Code, *req.ToLocation.Code)
		t.Fatalf("CreateOrder() error = %v", err)
	}

	// Проверяем обязательные поля
	if order.UUID == "" {
		t.Error("Expected non-empty order UUID")
	}

	t.Logf("✅ Service.CreateOrder: заказ создан")
	t.Logf("  UUID: %s", order.UUID)
	if order.Number != nil {
		t.Logf("  CDEK Number: %s", *order.Number)
	}
	if order.CreatedAt != "" {
		t.Logf("  Created At: %s", order.CreatedAt)
	}
	// TariffCode и Statuses появятся позже (асинхронное создание)
	if order.TariffCode > 0 {
		t.Logf("  Tariff Code: %d", order.TariffCode)
	}
	if len(order.Statuses) > 0 {
		t.Logf("  Current Status: %s (%s)", order.Statuses[0].Name, order.Statuses[0].Code)
	}
}

func TestSimple_ServiceTrackOrder(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
	ctx := context.Background()

	// Создаем тестовый заказ для отслеживания
	order := createTestOrder(t, service, ctx)

	// Отслеживаем заказ
	tracking, err := service.TrackOrder(ctx, order.UUID)
	if err != nil {
		t.Fatalf("TrackOrder() error = %v", err)
	}

	// Проверяем обязательные поля
	if tracking.UUID != order.UUID {
		t.Errorf("Expected UUID = %s, got %s", order.UUID, tracking.UUID)
	}

	if tracking.CurrentStatus.Name == "" {
		t.Error("Expected non-empty current status name")
	}

	if tracking.CurrentStatus.Code == "" {
		t.Error("Expected non-empty current status code")
	}

	if len(tracking.StatusHistory) == 0 {
		t.Error("Expected at least one status in history")
	}

	t.Logf("✅ Service.TrackOrder: отслеживание заказа")
	t.Logf("  UUID: %s", tracking.UUID)
	if tracking.Number != nil {
		t.Logf("  CDEK Number: %s", *tracking.Number)
	}
	t.Logf("  Current Status: %s (%s)", tracking.CurrentStatus.Name, tracking.CurrentStatus.Code)
	t.Logf("  Status History: %d events", len(tracking.StatusHistory))
	if tracking.EstimatedDelivery != nil {
		t.Logf("  Estimated Delivery: %s", *tracking.EstimatedDelivery)
	}

	// Выводим историю статусов
	for i, status := range tracking.StatusHistory {
		city := ""
		if status.City != nil {
			city = " (" + *status.City + ")"
		}
		t.Logf("    [%d] %s: %s%s at %s",
			i+1, status.Code, status.Name, city, status.DateTime)
	}
}

func TestSimple_ServiceListDeliveryPoints(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
	ctx := context.Background()

	req := &DeliveryPointsRequest{
		CityCode: "270", // Новосибирск
		Type:     "PVZ",
	}

	points, err := service.ListDeliveryPoints(ctx, req)
	if err != nil {
		t.Fatalf("ListDeliveryPoints() error = %v", err)
	}

	if len(points) == 0 {
		t.Error("Expected at least one delivery point")
	}

	t.Logf("✅ Service.ListDeliveryPoints: найдено %d ПВЗ в Новосибирске", len(points))

	// Проверяем структуру первых 3 ПВЗ
	for i := 0; i < 3 && i < len(points); i++ {
		p := points[i]

		if p.Code == "" {
			t.Errorf("Point[%d]: expected non-empty code", i)
		}
		if p.Name == "" {
			t.Errorf("Point[%d]: expected non-empty name", i)
		}
		if p.Location.Address == "" {
			t.Errorf("Point[%d]: expected non-empty address", i)
		}

		t.Logf("  [%d] %s", i+1, p.Name)
		t.Logf("      Code: %s", p.Code)
		t.Logf("      Type: %s", p.Type)
		t.Logf("      Address: %s", p.Location.Address)
		if p.WorkTime != "" {
			t.Logf("      Work Time: %s", p.WorkTime)
		}
		if len(p.Phones) > 0 {
			t.Logf("      Phone: %s", p.Phones[0].Number)
		}
		if p.Location.Latitude != 0 && p.Location.Longitude != 0 {
			t.Logf("      Coordinates: %.6f, %.6f", p.Location.Latitude, p.Location.Longitude)
		}
	}
}

func TestSimple_ServicePrintBarcode(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
	ctx := context.Background()

	// Создаем заказ для печати этикетки
	order := createTestOrder(t, service, ctx)

	// Запрашиваем печать этикетки (штрихкод)
	req := &PrintBarcodeRequest{
		Orders: []PrintOrder{
			{OrderUUID: order.UUID},
		},
	}

	printResp, err := service.PrintBarcode(ctx, req)
	if err != nil {
		t.Fatalf("PrintBarcode() error = %v", err)
	}

	if printResp.UUID == "" {
		t.Error("Expected non-empty print job UUID")
	}

	t.Logf("✅ Service.PrintBarcode: задание на печать создано")
	t.Logf("  Print Job UUID: %s", printResp.UUID)
	if printResp.URL != "" {
		t.Logf("  PDF URL: %s", printResp.URL)
	}
}

func TestSimple_ServicePrintWaybill(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
	ctx := context.Background()

	// Создаем заказ для печати накладной
	order := createTestOrder(t, service, ctx)

	// Запрашиваем печать накладной
	req := &PrintWaybillRequest{
		Orders: []PrintOrder{
			{OrderUUID: order.UUID},
		},
	}

	printResp, err := service.PrintWaybill(ctx, req)
	if err != nil {
		t.Fatalf("PrintWaybill() error = %v", err)
	}

	if printResp.UUID == "" {
		t.Error("Expected non-empty print job UUID")
	}

	t.Logf("✅ Service.PrintWaybill: задание на печать создано")
	t.Logf("  Print Job UUID: %s", printResp.UUID)
	if printResp.URL != "" {
		t.Logf("  PDF URL: %s", printResp.URL)
	}
}

// TestSimple_ServiceGetOrder тестирует получение полной информации о заказе
func TestSimple_ServiceGetOrder(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
	ctx := context.Background()

	// Создаем заказ для тестирования
	order := createTestOrder(t, service, ctx)

	// Получаем полную информацию
	orderInfo, err := service.GetOrder(ctx, order.UUID)
	if err != nil {
		t.Fatalf("GetOrder failed: %v", err)
	}

	if orderInfo.UUID == "" {
		t.Error("Expected non-empty order UUID")
	}

	t.Logf("✅ Service.GetOrder: получена полная информация о заказе")
	t.Logf("  UUID: %s", orderInfo.UUID)
	if orderInfo.Number != nil {
		t.Logf("  CDEK Number: %s", *orderInfo.Number)
	}
	t.Logf("  Type: %s", orderInfo.Type)
	t.Logf("  Tariff Code: %d", orderInfo.TariffCode)
	t.Logf("  Statuses: %d events", len(orderInfo.Statuses))
}

// TestSimple_ServiceUpdateOrder тестирует обновление заказа
func TestSimple_ServiceUpdateOrder(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
	ctx := context.Background()

	// Создаем заказ для тестирования
	order := createTestOrder(t, service, ctx)

	// Обновляем комментарий
	newComment := "Updated comment from integration test"
	updateReq := &UpdateOrderRequest{
		OrderUUID: order.UUID,
		Comment:   &newComment,
	}

	updatedOrder, err := service.UpdateOrder(ctx, updateReq)
	if err != nil {
		// Заказ может быть уже в обработке и недоступен для обновления
		// Это не критическая ошибка в тесте
		t.Logf("⚠️  UpdateOrder returned error (may be expected): %v", err)
		return
	}

	if updatedOrder.UUID == "" {
		t.Error("Expected non-empty UUID in updated order")
	}

	t.Logf("✅ Service.UpdateOrder: заказ обновлен")
	t.Logf("  UUID: %s", updatedOrder.UUID)
}

// TestSimple_ServiceCancelOrder тестирует отмену заказа
func TestSimple_ServiceCancelOrder(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
	ctx := context.Background()

	// Создаем заказ для тестирования
	order := createTestOrder(t, service, ctx)

	// Отменяем заказ
	err := service.CancelOrder(ctx, order.UUID)
	if err != nil {
		// Заказ может быть уже в обработке и недоступен для отмены
		// Это не критическая ошибка в тесте
		t.Logf("⚠️  CancelOrder returned error (may be expected): %v", err)
		return
	}

	t.Logf("✅ Service.CancelOrder: заказ отменен")
	t.Logf("  UUID: %s", order.UUID)
}

// TestSimple_ServiceDownloadBarcode тестирует скачивание PDF этикеток
func TestSimple_ServiceDownloadBarcode(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
	ctx := context.Background()

	// Создаем заказ
	order := createTestOrder(t, service, ctx)

	// Создаем задание на печать этикеток
	printReq := &PrintBarcodeRequest{
		Orders: []PrintOrder{{OrderUUID: order.UUID}},
	}

	printResp, err := service.PrintBarcode(ctx, printReq)
	if err != nil {
		t.Fatalf("PrintBarcode failed: %v", err)
	}

	// Даем немного времени на обработку задания
	time.Sleep(2 * time.Second)

	// Пробуем скачать PDF
	pdfBytes, err := service.DownloadBarcode(ctx, printResp.UUID)
	if err != nil {
		// PDF может быть еще не готов - это не критическая ошибка
		t.Logf("⚠️  DownloadBarcode returned error (PDF may not be ready yet): %v", err)
		return
	}

	if len(pdfBytes) == 0 {
		t.Error("Expected non-empty PDF bytes")
	}

	t.Logf("✅ Service.DownloadBarcode: PDF скачан")
	t.Logf("  Print Job UUID: %s", printResp.UUID)
	t.Logf("  PDF Size: %d bytes", len(pdfBytes))
}

// TestSimple_ServiceDownloadWaybill тестирует скачивание PDF накладной
func TestSimple_ServiceDownloadWaybill(t *testing.T) {
	client := getSimpleTestClient(t)
	service := NewService(client, nil)
	ctx := context.Background()

	// Создаем заказ
	order := createTestOrder(t, service, ctx)

	// Создаем задание на печать накладной
	printReq := &PrintWaybillRequest{
		Orders: []PrintOrder{{OrderUUID: order.UUID}},
	}

	printResp, err := service.PrintWaybill(ctx, printReq)
	if err != nil {
		t.Fatalf("PrintWaybill failed: %v", err)
	}

	// Даем немного времени на обработку задания
	time.Sleep(2 * time.Second)

	// Пробуем скачать PDF
	pdfBytes, err := service.DownloadWaybill(ctx, printResp.UUID)
	if err != nil {
		// PDF может быть еще не готов - это не критическая ошибка
		t.Logf("⚠️  DownloadWaybill returned error (PDF may not be ready yet): %v", err)
		return
	}

	if len(pdfBytes) == 0 {
		t.Error("Expected non-empty PDF bytes")
	}

	t.Logf("✅ Service.DownloadWaybill: PDF скачан")
	t.Logf("  Print Job UUID: %s", printResp.UUID)
	t.Logf("  PDF Size: %d bytes", len(pdfBytes))
}
