//go:build integration
// +build integration

package cdek

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// E2E-сценарий AAI-32 «ПВЗ → ПВЗ» против живого CDEK API:
// подбор ПВЗ (is_reception Москва / is_handout СПб) → CreateOrder тариф 136
// type=delivery → опрос GET до SUCCESSFUL/INVALID → отмена, пока CREATED.
//
// Креды только из env: CDEK_TEST_CLIENT_ID / CDEK_TEST_CLIENT_SECRET,
// опционально CDEK_TEST_BASE_URL (по умолчанию тестовый контур URLSandbox — тест создаёт заказ,
// поэтому на боевой API он идёт только по явному CDEK_TEST_BASE_URL=https://api.cdek.ru),
// CDEK_TEST_ORDER_TYPE (по умолчанию online_store — тип боевого договора ИМ),
// CDEK_TEST_EXPECT_SUCCESS=1 — требовать requests[0].state=SUCCESSFUL (иначе INVALID тестового
// аккаунта только логируется).
// Запуск: go test -tags=integration -run TestIntegration_PVZ -v ./pkg/cdek/

func TestIntegration_PVZOrderLifecycle(t *testing.T) {
	clientID := os.Getenv("CDEK_TEST_CLIENT_ID")
	clientSecret := os.Getenv("CDEK_TEST_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		t.Skip("CDEK_TEST_CLIENT_ID or CDEK_TEST_CLIENT_SECRET not set")
	}

	baseURL := os.Getenv("CDEK_TEST_BASE_URL")
	if baseURL == "" {
		baseURL = URLSandbox
	}

	config := &AccountConfig{
		Name:         "pvz-e2e",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		BaseURL:      baseURL,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
	}
	client, err := NewAuthenticatedClient(config)
	if err != nil {
		t.Fatalf("NewAuthenticatedClient() error = %v", err)
	}
	service := NewService(client, nil)
	ctx := context.Background()

	// ПВЗ сдачи: is_reception в Москве (код города 44)
	mskPoints, err := service.ListDeliveryPoints(ctx, &DeliveryPointsRequest{
		CityCode:    "44",
		Type:        "PVZ",
		IsReception: ptrBool(true),
		Size:        ptrInt32(10),
	})
	if errors.Is(err, ErrServerError) {
		t.Skipf("API CDEK недоступен (5xx на подготовительном шаге): %v", err)
	}
	if err != nil {
		t.Fatalf("ListDeliveryPoints(Moscow is_reception) error = %v", err)
	}
	if len(mskPoints) == 0 {
		t.Fatal("no reception PVZ found in Moscow (city 44)")
	}
	shipmentPoint := mskPoints[0].Code

	// ПВЗ выдачи: is_handout в Санкт-Петербурге (код города 137)
	spbPoints, err := service.ListDeliveryPoints(ctx, &DeliveryPointsRequest{
		CityCode:  "137",
		Type:      "PVZ",
		IsHandout: ptrBool(true),
		Size:      ptrInt32(10),
	})
	if err != nil {
		t.Fatalf("ListDeliveryPoints(SPb is_handout) error = %v", err)
	}
	if len(spbPoints) == 0 {
		t.Fatal("no handout PVZ found in Saint Petersburg (city 137)")
	}
	deliveryPoint := spbPoints[0].Code

	t.Logf("прием: %s (%s), выдача: %s (%s)", shipmentPoint, mskPoints[0].Name, deliveryPoint, spbPoints[0].Name)

	// Тариф 136 «Посылка склад-склад»: обе стороны — коды ПВЗ, адресов нет.
	// type=delivery: items не обязательны; sender.company и packages[].comment
	// обязательны договором тестового аккаунта (живая проба 2026-08-19)
	comment := "E2E TEST - cancelled immediately"
	packageComment := "E2E TEST - cancelled immediately"
	senderCompany := "Test Sender LLC"
	orderType := os.Getenv("CDEK_TEST_ORDER_TYPE")
	if orderType == "" {
		orderType = "online_store"
	}
	req := &OrderRequest{
		Type:          orderType,
		TariffCode:    136,
		Comment:       &comment,
		ShipmentPoint: &shipmentPoint,
		DeliveryPoint: &deliveryPoint,
		Sender: Recipient{
			Contact: Contact{
				Name:    "E2E Test Sender",
				Company: &senderCompany,
				Phones:  []Phone{{Number: "+79099999999"}},
			},
		},
		Recipient: Recipient{
			Contact: Contact{
				Name:   "Иванов Иван Иванович",
				Phones: []Phone{{Number: "+79001234567"}},
			},
		},
		Packages: []OrderPackage{{
			Number:  "E2E-PVZ-001",
			Weight:  1000, // 1 кг
			Length:  ptrInt32(10),
			Width:   ptrInt32(10),
			Height:  ptrInt32(10),
			Comment: &packageComment,
			Items:   []Item{{Name: "E2E item", WareKey: "e2e-1", Cost: 100, Weight: 1000, Amount: 1}},
		}},
	}

	order, err := service.CreateOrder(ctx, req)
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if order.UUID == "" {
		t.Fatal("CreateOrder() returned empty UUID")
	}
	t.Logf("заказ создан (async), UUID: %s", order.UUID)

	// Отмена в cleanup — только пока заказ еще CREATED (statuses[0] — актуальный)
	t.Cleanup(func() {
		info, err := service.GetOrder(ctx, order.UUID)
		if err != nil {
			t.Logf("cleanup: GetOrder() error = %v, cancel skipped", err)
			return
		}
		if len(info.Statuses) == 0 || info.Statuses[0].Code != "CREATED" {
			status := ""
			if len(info.Statuses) > 0 {
				status = info.Statuses[0].Code
			}
			t.Logf("cleanup: статус заказа %q — не CREATED, отмена не выполняется", status)
			return
		}
		if err := service.CancelOrder(ctx, order.UUID); err != nil {
			t.Logf("cleanup: CancelOrder() error = %v", err)
			return
		}
		t.Logf("cleanup: заказ %s отменен", order.UUID)
	})

	// Создание асинхронное: HTTP 202/ACCEPTED ≠ заказ создан. Опрашиваем GET
	// /v2/orders до терминального состояния requests[0].state
	deadline := time.Now().Add(30 * time.Second)
	var info *OrderInfo
	for {
		info, err = service.GetOrder(ctx, order.UUID)
		if err != nil {
			t.Fatalf("GetOrder() error = %v", err)
		}
		if len(info.Requests) > 0 {
			state := info.Requests[0].State
			if state == "SUCCESSFUL" || state == "INVALID" {
				break
			}
		}
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(3 * time.Second)
	}

	if len(info.Requests) == 0 {
		t.Fatalf("requests[] пуст после 30 с опроса; статусы: %+v", info.Statuses)
	}
	requestState := info.Requests[0].State
	if requestState != "SUCCESSFUL" && os.Getenv("CDEK_TEST_EXPECT_SUCCESS") == "1" {
		t.Fatalf("requests[0].state = %s, ожидался SUCCESSFUL; errors: %+v",
			requestState, info.Requests[0].Errors)
	}

	if requestState != "SUCCESSFUL" {
		t.Logf("requests[0].state = %s (errors: %+v) — SUCCESSFUL не требуется без CDEK_TEST_EXPECT_SUCCESS=1", requestState, info.Requests[0].Errors)
	}

	if info.ShipmentPoint != shipmentPoint {
		t.Errorf("ShipmentPoint = %q, ожидался %q", info.ShipmentPoint, shipmentPoint)
	}
	if info.DeliveryPoint != deliveryPoint {
		t.Errorf("DeliveryPoint = %q, ожидался %q", info.DeliveryPoint, deliveryPoint)
	}
	if info.Number == nil {
		t.Error("Number = nil, ожидался номер CDEK после SUCCESSFUL")
	} else {
		t.Logf("SUCCESSFUL: номер CDEK %s, %s → %s", *info.Number, info.ShipmentPoint, info.DeliveryPoint)
	}
}
