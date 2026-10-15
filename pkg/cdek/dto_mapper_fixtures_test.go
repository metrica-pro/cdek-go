package cdek

// dto_mapper_fixtures_test.go — контрактные тесты мапперов ответов на
// санитизированных реальных фикстурах CDEK (pkg/cdek/testdata/*.json).
// Фикстуры не редактируются; недостающие варианты живут в *_synthetic.json.

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// loadFixture читает тестовую фикстуру из testdata
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name) //nolint:gosec // имя фикстуры зашито в тесте, не пользовательский ввод
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// ========================
// fromCDEKOrderToInfo на живых фикстурах GET /v2/orders
// ========================

// TestFromCDEKOrderToInfoContract: каждая реальная фикстура разбирается без
// ошибки; проверяются type (число → строка), tariff_code, коды ПВЗ,
// from/to_location, packages+items, статусы, cdek_number
func TestFromCDEKOrderToInfoContract(t *testing.T) {
	m := newDtoMapper()

	tests := []struct {
		name string
		// ожидания
		uuid            string
		wantNumber      string // cdek_number
		wantType        string
		wantTariff      int
		wantShipment    string
		wantDelivery    string
		wantFromCode    int32
		wantToCode      int32
		wantPkgWeight   int32
		wantPkgNumber   string
		wantItems       int
		wantStatuses    int
		wantDeliveryDay string // entity.delivery_detail.date, "" — отсутствует
	}{
		{
			name:            "order_t136_dptrue_sptrue.json",
			uuid:            "00000000-0000-4000-8000-000000000004",
			wantNumber:      "10000000001",
			wantType:        "online_store",
			wantTariff:      136,
			wantShipment:    "MSK553",
			wantDelivery:    "ZLN5",
			wantFromCode:    44,
			wantToCode:      184,
			wantPkgNumber:   "1",
			wantPkgWeight:   150,
			wantItems:       1,
			wantStatuses:    2,
			wantDeliveryDay: "",
		},
		{
			name:            "order_by_uuid_t136_dptrue_sptrue.json",
			uuid:            "00000000-0000-4000-8000-000000000004",
			wantNumber:      "10000000001",
			wantType:        "online_store",
			wantTariff:      136,
			wantShipment:    "MSK553",
			wantDelivery:    "ZLN5",
			wantFromCode:    44,
			wantToCode:      184,
			wantPkgNumber:   "1",
			wantPkgWeight:   150,
			wantItems:       1,
			wantStatuses:    2,
			wantDeliveryDay: "",
		},
		{
			name:            "order_t136_dptrue_spfalse.json",
			uuid:            "00000000-0000-4000-8000-000000000015",
			wantNumber:      "10000000001",
			wantType:        "online_store",
			wantTariff:      136,
			wantShipment:    "",
			wantDelivery:    "KLG18",
			wantFromCode:    44,
			wantToCode:      142,
			wantPkgNumber:   "1",
			wantPkgWeight:   1460,
			wantItems:       1,
			wantStatuses:    23,
			wantDeliveryDay: "2026-07-08",
		},
		{
			name:            "order_t137_dpfalse_sptrue.json",
			uuid:            "00000000-0000-4000-8000-000000000009",
			wantNumber:      "10000000001",
			wantType:        "online_store",
			wantTariff:      137,
			wantShipment:    "MSK553",
			wantDelivery:    "",
			wantFromCode:    44,
			wantToCode:      1661,
			wantPkgNumber:   "1",
			wantPkgWeight:   1600,
			wantItems:       1,
			wantStatuses:    23,
			wantDeliveryDay: "2026-09-03",
		},
		{
			name:            "order_t137_dptrue_sptrue.json",
			uuid:            "00000000-0000-4000-8000-000000000020",
			wantNumber:      "10000000001",
			wantType:        "online_store",
			wantTariff:      137,
			wantShipment:    "MSK553",
			wantDelivery:    "MSK1887",
			wantFromCode:    44,
			wantToCode:      44,
			wantPkgNumber:   "1",
			wantPkgWeight:   3920,
			wantItems:       1,
			wantStatuses:    28,
			wantDeliveryDay: "2026-06-23",
		},
		{
			name:            "order_t138_dptrue_spfalse.json",
			uuid:            "00000000-0000-4000-8000-000000000024",
			wantNumber:      "10000000001",
			wantType:        "online_store",
			wantTariff:      138,
			wantShipment:    "",
			wantDelivery:    "RZN10",
			wantFromCode:    44,
			wantToCode:      159,
			wantPkgNumber:   "1",
			wantPkgWeight:   20000,
			wantItems:       1,
			wantStatuses:    14,
			wantDeliveryDay: "2026-05-07",
		},
		{
			name:          "order_t483_dptrue_sptrue.json",
			uuid:          "00000000-0000-4000-8000-000000000005",
			wantNumber:    "10000000001",
			wantType:      "delivery",
			wantTariff:    483,
			wantShipment:  "TAR8",
			wantDelivery:  "MSK553",
			wantFromCode:  1939,
			wantToCode:    44,
			wantPkgNumber: "[P09]12005424607",
			wantPkgWeight: 150,
			wantItems:     0, // type=2 (доставка) — items могут отсутствовать
			wantStatuses:  20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := m.fromCDEKOrderToInfo(loadFixture(t, tt.name))
			if err != nil {
				t.Fatalf("fromCDEKOrderToInfo() error = %v", err)
			}

			if info.UUID != tt.uuid {
				t.Errorf("UUID = %q, ожидался %q", info.UUID, tt.uuid)
			}
			if info.Number == nil || *info.Number != tt.wantNumber {
				t.Errorf("Number = %v, ожидался %q", info.Number, tt.wantNumber)
			}
			if info.Type != tt.wantType {
				t.Errorf("Type = %q, ожидался %q", info.Type, tt.wantType)
			}
			if info.TariffCode != tt.wantTariff {
				t.Errorf("TariffCode = %d, ожидался %d", info.TariffCode, tt.wantTariff)
			}
			if info.ShipmentPoint != tt.wantShipment {
				t.Errorf("ShipmentPoint = %q, ожидался %q", info.ShipmentPoint, tt.wantShipment)
			}
			if info.DeliveryPoint != tt.wantDelivery {
				t.Errorf("DeliveryPoint = %q, ожидался %q", info.DeliveryPoint, tt.wantDelivery)
			}
			if info.FromLocation.Code == nil || *info.FromLocation.Code != tt.wantFromCode {
				t.Errorf("FromLocation.Code = %v, ожидался %d", info.FromLocation.Code, tt.wantFromCode)
			}
			if info.ToLocation.Code == nil || *info.ToLocation.Code != tt.wantToCode {
				t.Errorf("ToLocation.Code = %v, ожидался %d", info.ToLocation.Code, tt.wantToCode)
			}
			if len(info.Packages) != 1 {
				t.Fatalf("len(Packages) = %d, ожидался 1", len(info.Packages))
			}
			pkg := info.Packages[0]
			if pkg.Number != tt.wantPkgNumber || pkg.Weight != tt.wantPkgWeight {
				t.Errorf("package number/weight = %q/%d, ожидались %q/%d",
					pkg.Number, pkg.Weight, tt.wantPkgNumber, tt.wantPkgWeight)
			}
			if len(pkg.Items) != tt.wantItems {
				t.Errorf("len(Items) = %d, ожидался %d", len(pkg.Items), tt.wantItems)
			}
			if len(info.Statuses) != tt.wantStatuses {
				t.Errorf("len(Statuses) = %d, ожидался %d", len(info.Statuses), tt.wantStatuses)
			}
			if len(info.Statuses) > 0 && info.Statuses[0].Code == "" {
				t.Error("Statuses[0].Code не разобран")
			}
			if tt.wantDeliveryDay == "" {
				if info.EstimatedDelivery != nil {
					t.Errorf("EstimatedDelivery = %v, ожидался nil", info.EstimatedDelivery)
				}
			} else if info.EstimatedDelivery == nil || *info.EstimatedDelivery != tt.wantDeliveryDay {
				t.Errorf("EstimatedDelivery = %v, ожидался %q", info.EstimatedDelivery, tt.wantDeliveryDay)
			}
		})
	}
}

// TestFromCDEKOrderToInfoItems: детали items фикстуры — payment как объект,
// cost/weight/amount по единице товара
func TestFromCDEKOrderToInfoItems(t *testing.T) {
	m := newDtoMapper()

	t.Run("order_t136_dptrue_sptrue.json", func(t *testing.T) {
		info, err := m.fromCDEKOrderToInfo(loadFixture(t, "order_t136_dptrue_sptrue.json"))
		if err != nil {
			t.Fatalf("fromCDEKOrderToInfo() error = %v", err)
		}
		item := info.Packages[0].Items[0]
		if item.WareKey != "11" || item.Cost != 500 || item.Payment != 0 || item.Weight != 150 || item.Amount != 1 {
			t.Errorf("item = %+v, ожидались ware_key=11 cost=500 payment=0 weight=150 amount=1", item)
		}
		if item.Name == "" {
			t.Error("item.Name не разобран")
		}
		// seller присутствует в фикстуре с ИНН
		if info.Seller == nil || info.Seller.INN == nil || *info.Seller.INN != "7709033880" {
			t.Errorf("Seller.INN = %v, ожидался 7709033880", info.Seller)
		}
		// реальные ответы CDEK отдают ownership_form строкой — маппер принимает
		// только число, поле остается nil (см. SellerDto в api/cdek-api.yaml)
		if info.Seller != nil && info.Seller.OwnershipForm != nil {
			t.Errorf("Seller.OwnershipForm = %v, ожидался nil для строкового значения", info.Seller.OwnershipForm)
		}
	})

	t.Run("order_t137_dptrue_sptrue.json: amount=2", func(t *testing.T) {
		info, err := m.fromCDEKOrderToInfo(loadFixture(t, "order_t137_dptrue_sptrue.json"))
		if err != nil {
			t.Fatalf("fromCDEKOrderToInfo() error = %v", err)
		}
		item := info.Packages[0].Items[0]
		if item.Amount != 2 || item.Weight != 1900 {
			t.Errorf("item amount/weight = %d/%d, ожидались 2/1900", item.Amount, item.Weight)
		}
	})
}

// TestFromCDEKOrderToInfoFullContacts: полные контакты (ИНН, паспорт, email,
// добавочный телефон) и seller с числовой формой собственности
func TestFromCDEKOrderToInfoFullContacts(t *testing.T) {
	m := newDtoMapper()

	info, err := m.fromCDEKOrderToInfo([]byte(`{"entity":{
		"uuid": "u-full",
		"sender": {
			"company": "ООО «Отправитель»",
			"name": "Сидор Сидоров",
			"email": "sender@example.com",
			"phones": [{"number": "+79007778899", "additional": "42"}],
			"tin": "7801234567",
			"passport_series": "4510",
			"passport_number": "654321",
			"passport_date_of_issue": "2015-06-01",
			"passport_organization": "ОВД Санкт-Петербург",
			"passport_date_of_birth": "1985-03-03"
		},
		"recipient": {
			"name": "Иван Иванов",
			"email": "ivan@example.com",
			"phones": [{"number": "+79001112233", "additional": "123"}],
			"tin": "7709033880",
			"passport_series": "4510",
			"passport_number": "123456",
			"passport_date_of_issue": "2020-01-01",
			"passport_organization": "ОВД Москва",
			"passport_date_of_birth": "1990-05-05"
		},
		"seller": {
			"name": "ООО «Продавец»",
			"inn": "7709033880",
			"phone": "+70000000000",
			"ownership_form": 63,
			"address": "г. Москва, ул. Тестовая, 1"
		},
		"statuses": []
	}}`))
	if err != nil {
		t.Fatalf("fromCDEKOrderToInfo() error = %v", err)
	}

	wantSender := fullRecipient()
	wantSender.Company = ptrString("ООО «Отправитель»")
	wantSender.Name = "Сидор Сидоров"
	wantSender.Email = ptrString("sender@example.com")
	wantSender.Phones = []Phone{{Number: "+79007778899", Additional: ptrString("42")}}
	wantSender.TIN = ptrString("7801234567")
	wantSender.PassportSeries = ptrString("4510")
	wantSender.PassportNumber = ptrString("654321")
	wantSender.PassportDateOfIssue = ptrString("2015-06-01")
	wantSender.PassportOrganization = ptrString("ОВД Санкт-Петербург")
	wantSender.PassportDateOfBirth = ptrString("1985-03-03")
	if !reflect.DeepEqual(info.Sender, wantSender) {
		t.Errorf("Sender = %+v\nожидался  %+v", info.Sender, wantSender)
	}

	if info.Recipient.TIN == nil || *info.Recipient.TIN != "7709033880" {
		t.Errorf("Recipient.TIN = %v", info.Recipient.TIN)
	}
	if len(info.Recipient.Phones) != 1 || info.Recipient.Phones[0].Additional == nil || *info.Recipient.Phones[0].Additional != "123" {
		t.Errorf("Recipient.Phones = %+v, ожидался добавочный 123", info.Recipient.Phones)
	}
	if info.Recipient.PassportNumber == nil || *info.Recipient.PassportNumber != "123456" {
		t.Errorf("Recipient.PassportNumber = %v", info.Recipient.PassportNumber)
	}
	if info.Recipient.Email == nil || *info.Recipient.Email != "ivan@example.com" {
		t.Errorf("Recipient.Email = %v", info.Recipient.Email)
	}

	if info.Seller == nil {
		t.Fatal("Seller не разобран")
	}
	ownership := 63
	if info.Seller.OwnershipForm == nil || *info.Seller.OwnershipForm != ownership {
		t.Errorf("Seller.OwnershipForm = %v, ожидался 63 (число)", info.Seller.OwnershipForm)
	}
	if info.Seller.Phone == nil || *info.Seller.Phone != "+70000000000" {
		t.Errorf("Seller.Phone = %v", info.Seller.Phone)
	}
	if info.Seller.Address == nil || *info.Seller.Address != "г. Москва, ул. Тестовая, 1" {
		t.Errorf("Seller.Address = %v", info.Seller.Address)
	}
}

// TestFromCDEKOrderToInfoInvalidRequests: фикстура с requests[].state=INVALID
// и errors[] — состояние асинхронной обработки разбирается полностью
func TestFromCDEKOrderToInfoInvalidRequests(t *testing.T) {
	m := newDtoMapper()

	info, err := m.fromCDEKOrderToInfo(loadFixture(t, "order_requests_invalid_synthetic.json"))
	if err != nil {
		t.Fatalf("fromCDEKOrderToInfo() error = %v", err)
	}

	if len(info.Requests) != 2 {
		t.Fatalf("len(Requests) = %d, ожидался 2", len(info.Requests))
	}

	create := info.Requests[0]
	if create.Type != "CREATE" || create.State != "INVALID" || create.DateTime != "2026-10-15T11:53:42+0000" {
		t.Errorf("Requests[0] = %+v, ожидался CREATE/INVALID с date_time", create)
	}
	wantErrs := []OrderError{
		{Code: "error_validate_receiver_delivery_point_is_empty", Message: "Не задан офис получателя"},
		{Code: "error_validate_tariff_requires_pvz", Message: "Тариф требует доставки до ПВЗ"},
	}
	if !reflect.DeepEqual(create.Errors, wantErrs) {
		t.Errorf("Requests[0].Errors = %+v, ожидались %+v", create.Errors, wantErrs)
	}

	if info.Requests[1].State != "SUCCESSFUL" || info.Requests[1].Errors != nil {
		t.Errorf("Requests[1] = %+v, ожидался SUCCESSFUL без ошибок", info.Requests[1])
	}

	// date_time первого запроса — CreatedAt
	if info.CreatedAt != "2026-10-15T11:53:42+0000" {
		t.Errorf("CreatedAt = %q, ожидался date_time первого запроса", info.CreatedAt)
	}
}

// TestFromCDEKOrderToInfoErrors: негативные разборы
func TestFromCDEKOrderToInfoErrors(t *testing.T) {
	m := newDtoMapper()

	tests := []struct {
		name string
		json string
	}{
		{name: "битый JSON", json: `{"entity":`},
		{name: "нет entity", json: `{"requests":[]}`},
		{name: "пустой ответ", json: ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := m.fromCDEKOrderToInfo([]byte(tt.json)); err == nil {
				t.Fatal("fromCDEKOrderToInfo() ожидалась ошибка, получен nil")
			}
		})
	}
}

// ========================
// fromCDEKOrderToTracking
// ========================

// TestFromCDEKOrderToTrackingContract: живая фикстура — история статусов,
// текущий статус (последний элемент массива), номер, плановая дата
func TestFromCDEKOrderToTrackingContract(t *testing.T) {
	m := newDtoMapper()

	t.Run("order_by_uuid_t136_dptrue_sptrue.json", func(t *testing.T) {
		tracking, err := m.fromCDEKOrderToTracking(loadFixture(t, "order_by_uuid_t136_dptrue_sptrue.json"))
		if err != nil {
			t.Fatalf("fromCDEKOrderToTracking() error = %v", err)
		}
		if tracking.UUID != "00000000-0000-4000-8000-000000000004" {
			t.Errorf("UUID = %q", tracking.UUID)
		}
		if tracking.Number == nil || *tracking.Number != "10000000001" {
			t.Errorf("Number = %v, ожидался 10000000001", tracking.Number)
		}
		if len(tracking.StatusHistory) != 2 {
			t.Fatalf("len(StatusHistory) = %d, ожидался 2", len(tracking.StatusHistory))
		}
		// Текущий статус — последний элемент списка статусов ответа
		if tracking.CurrentStatus.Code != "ACCEPTED" || tracking.CurrentStatus.DateTime != "2026-10-15T11:53:42+0000" {
			t.Errorf("CurrentStatus = %+v, ожидался ACCEPTED с date_time 11:53:42", tracking.CurrentStatus)
		}
		if tracking.StatusHistory[0].Code != "CREATED" {
			t.Errorf("StatusHistory[0].Code = %q, ожидался CREATED", tracking.StatusHistory[0].Code)
		}
		if tracking.EstimatedDelivery != nil {
			t.Errorf("EstimatedDelivery = %v, ожидался nil (delivery_detail.date нет)", tracking.EstimatedDelivery)
		}
	})

	t.Run("order_t136_dptrue_spfalse.json: плановая дата доставки", func(t *testing.T) {
		tracking, err := m.fromCDEKOrderToTracking(loadFixture(t, "order_t136_dptrue_spfalse.json"))
		if err != nil {
			t.Fatalf("fromCDEKOrderToTracking() error = %v", err)
		}
		if tracking.EstimatedDelivery == nil || *tracking.EstimatedDelivery != "2026-07-08" {
			t.Errorf("EstimatedDelivery = %v, ожидался 2026-07-08", tracking.EstimatedDelivery)
		}
		// Статусы в ответе идут от свежих к старым; текущий — последний элемент
		if tracking.CurrentStatus.Code != "ACCEPTED" || tracking.CurrentStatus.DateTime != "2026-06-30T14:03:09+0000" {
			t.Errorf("CurrentStatus = %+v, ожидался ACCEPTED (последний элемент массива)", tracking.CurrentStatus)
		}
		if tracking.StatusHistory[0].Code != "DELIVERED" {
			t.Errorf("StatusHistory[0].Code = %q, ожидался DELIVERED (свежий статус первым)", tracking.StatusHistory[0].Code)
		}
	})

	t.Run("негативные разборы", func(t *testing.T) {
		tests := []struct {
			name string
			json string
		}{
			{name: "битый JSON", json: `{`},
			{name: "нет entity", json: `{}`},
			{name: "entity без статусов", json: `{"entity":{"uuid":"u1"}}`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tracking, err := m.fromCDEKOrderToTracking([]byte(tt.json))
				if tt.name == "entity без статусов" {
					// Без статусов ошибки нет — пустой TrackingInfo с UUID
					if err != nil {
						t.Fatalf("fromCDEKOrderToTracking() error = %v", err)
					}
					if tracking.UUID != "u1" || len(tracking.StatusHistory) != 0 {
						t.Errorf("tracking = %+v, ожидался пустой с UUID", tracking)
					}
					return
				}
				if err == nil {
					t.Fatal("fromCDEKOrderToTracking() ожидалась ошибка, получен nil")
				}
			})
		}
	})
}

// ========================
// fromCDEKDeliveryPoints на живых фикстурах
// ========================

// TestFromCDEKDeliveryPointsContract: фикстура deliverypoints_msk — city_code,
// address_full, флаги, вес в кг, телефоны, фото офиса
func TestFromCDEKDeliveryPointsContract(t *testing.T) {
	m := newDtoMapper()

	t.Run("deliverypoints_msk.json", func(t *testing.T) {
		points, err := m.fromCDEKDeliveryPoints(loadFixture(t, "deliverypoints_msk.json"))
		if err != nil {
			t.Fatalf("fromCDEKDeliveryPoints() error = %v", err)
		}
		if len(points) != 2 {
			t.Fatalf("len(points) = %d, ожидался 2", len(points))
		}

		p := points[0]
		if p.Code != "MSK2" || p.Type != "PVZ" || p.OwnerCode != "CDEK" {
			t.Errorf("code/type/owner = %q/%q/%q, ожидались MSK2/PVZ/CDEK", p.Code, p.Type, p.OwnerCode)
		}
		if p.Name == "" || p.WorkTime == "" || p.Email == nil || p.Note == nil {
			t.Errorf("name/work_time/email/note не разобраны: %+v", p)
		}

		// Флаги офиса
		if !p.HaveCashless || !p.HaveCash || !p.AllowedCOD || !p.IsDressingRoom {
			t.Errorf("флаги = cashless:%v cash:%v cod:%v dress:%v, ожидались все true",
				p.HaveCashless, p.HaveCash, p.AllowedCOD, p.IsDressingRoom)
		}

		// Вес приема/выдачи в килограммах
		if p.WeightMin == nil || *p.WeightMin != 0 {
			t.Errorf("WeightMin = %v, ожидался указатель на 0 (поле есть в ответе)", p.WeightMin)
		}
		if p.WeightMax == nil || *p.WeightMax != 35 {
			t.Errorf("WeightMax = %v, ожидался 35 кг", p.WeightMax)
		}
		if points[1].WeightMax == nil || *points[1].WeightMax != 30 {
			t.Errorf("points[1].WeightMax = %v, ожидался 30 кг", points[1].WeightMax)
		}

		// Локация: city_code, address_full, координаты
		loc := p.Location
		if loc.CityCode != 44 || loc.City != "Москва" || loc.RegionCode != 81 {
			t.Errorf("location city/city_code/region_code = %q/%d/%d, ожидались Москва/44/81",
				loc.City, loc.CityCode, loc.RegionCode)
		}
		// В фикстуре location без ключа country (только country_code) —
		// Country остается пустым, ошибки нет
		if loc.Country != "" {
			t.Errorf("location.country = %q, ожидалась пустая (ключа нет в фикстуре)", loc.Country)
		}
		if loc.Region != "Москва" {
			t.Errorf("location.region = %q, ожидалась Москва", loc.Region)
		}
		if loc.AddressFull == "" || loc.Address == "" || loc.PostalCode != "109544" {
			t.Errorf("address/address_full/postal_code не разобраны: %+v", loc)
		}
		if loc.Latitude != 55.74564 || loc.Longitude != 37.68482 {
			t.Errorf("координаты = %v/%v, ожидались 55.74564/37.68482", loc.Latitude, loc.Longitude)
		}

		// Телефоны и первое фото офиса
		if len(p.Phones) != 2 || p.Phones[0].Number != "+70000000000" {
			t.Errorf("phones = %+v, ожидались 2 телефона", p.Phones)
		}
		if p.OfficeImage == nil || !strings.HasPrefix(*p.OfficeImage, "https://gateway.cdek.ru/") {
			t.Errorf("OfficeImage = %v, ожидался URL первого фото", p.OfficeImage)
		}
	})

	t.Run("deliverypoints_by_code.json", func(t *testing.T) {
		points, err := m.fromCDEKDeliveryPoints(loadFixture(t, "deliverypoints_by_code.json"))
		if err != nil {
			t.Fatalf("fromCDEKDeliveryPoints() error = %v", err)
		}
		if len(points) != 1 || points[0].Code != "MSK2" {
			t.Fatalf("points = %+v, ожидался один MSK2", points)
		}
		if points[0].Location.CityCode != 44 {
			t.Errorf("city_code = %d, ожидался 44", points[0].Location.CityCode)
		}
	})

	t.Run("пустой список", func(t *testing.T) {
		points, err := m.fromCDEKDeliveryPoints([]byte(`[]`))
		if err != nil {
			t.Fatalf("fromCDEKDeliveryPoints() error = %v", err)
		}
		if len(points) != 0 {
			t.Errorf("len(points) = %d, ожидался 0", len(points))
		}
	})

	t.Run("битый JSON", func(t *testing.T) {
		if _, err := m.fromCDEKDeliveryPoints([]byte(`[{`)); err == nil {
			t.Fatal("fromCDEKDeliveryPoints() ожидалась ошибка, получен nil")
		}
	})
}

// ========================
// fromCDEKCities / fromCDEKRegions на живых фикстурах
// ========================
func TestFromCDEKCitiesContract(t *testing.T) {
	m := newDtoMapper()

	cities, err := m.fromCDEKCities(loadFixture(t, "cities_ru.json"))
	if err != nil {
		t.Fatalf("fromCDEKCities() error = %v", err)
	}
	if len(cities) != 2 {
		t.Fatalf("len(cities) = %d, ожидался 2", len(cities))
	}

	want := []City{
		{Code: 5, City: "Усинск", Region: "Республика Коми", Country: "Россия", CountryCode: "RU"},
		{Code: 6, City: "Алтайское", Region: "Алтайский край", Country: "Россия", CountryCode: "RU"},
	}
	for i, w := range want {
		if cities[i].Code != w.Code || cities[i].City != w.City ||
			cities[i].Region != w.Region || cities[i].Country != w.Country || cities[i].CountryCode != w.CountryCode {
			t.Errorf("cities[%d] = %+v, ожидался %+v", i, cities[i], w)
		}
	}

	t.Run("негативные разборы", func(t *testing.T) {
		if _, err := m.fromCDEKCities([]byte(`{`)); err == nil {
			t.Error("fromCDEKCities() ожидалась ошибка для битого JSON")
		}
		if cities, err := m.fromCDEKCities([]byte(`[]`)); err != nil || len(cities) != 0 {
			t.Errorf("fromCDEKCities(`[]`) = %v, %v; ожидался пустой список без ошибки", cities, err)
		}
	})
}

func TestFromCDEKRegionsContract(t *testing.T) {
	m := newDtoMapper()

	regions, err := m.fromCDEKRegions(loadFixture(t, "regions_ru.json"))
	if err != nil {
		t.Fatalf("fromCDEKRegions() error = %v", err)
	}
	if len(regions) != 2 {
		t.Fatalf("len(regions) = %d, ожидался 2", len(regions))
	}

	// Мапперы регионов не читают country_code/fias_region_guid (есть в фикстуре,
	// нет в разборе) — CountryCode/FiasGUID остаются нулевыми
	want := []Region{
		{Code: 80, Region: "Карачаево-Черкесская Республика", Country: "Россия"},
		{Code: 66, Region: "Архангельская область", Country: "Россия"},
	}
	for i, w := range want {
		if regions[i].Code != w.Code || regions[i].Region != w.Region ||
			regions[i].Country != w.Country || regions[i].CountryCode != w.CountryCode {
			t.Errorf("regions[%d] = %+v, ожидался %+v", i, regions[i], w)
		}
	}

	t.Run("негативные разборы", func(t *testing.T) {
		if _, err := m.fromCDEKRegions([]byte(`{`)); err == nil {
			t.Error("fromCDEKRegions() ожидалась ошибка для битого JSON")
		}
		if regions, err := m.fromCDEKRegions([]byte(`[]`)); err != nil || len(regions) != 0 {
			t.Errorf("fromCDEKRegions(`[]`) = %v, %v; ожидался пустой список без ошибки", regions, err)
		}
	})
}

// ========================
// fromCDEKCalculatorResponse
// ========================

// TestFromCDEKCalculatorResponseContract: успешный расчет (synthetic-вариант
// с tariff_codes) и реальный ответ с ошибками валидации
func TestFromCDEKCalculatorResponseContract(t *testing.T) {
	m := newDtoMapper()

	t.Run("успех: два тарифа", func(t *testing.T) {
		resp, err := m.fromCDEKCalculatorResponse(loadFixture(t, "calculator_tarifflist_synthetic.json"))
		if err != nil {
			t.Fatalf("fromCDEKCalculatorResponse() error = %v", err)
		}
		if len(resp.Tariffs) != 2 {
			t.Fatalf("len(Tariffs) = %d, ожидался 2", len(resp.Tariffs))
		}

		first := TariffOption{
			TariffCode:   136,
			TariffName:   "Экспресс склад-склад",
			DeliveryMode: 4,
			DeliverySum:  225,
			PeriodMin:    1,
			PeriodMax:    2,
		}
		if !reflect.DeepEqual(resp.Tariffs[0], first) {
			t.Errorf("Tariffs[0] = %+v, ожидался %+v", resp.Tariffs[0], first)
		}
		if resp.Tariffs[1].TariffCode != 483 || resp.Tariffs[1].DeliverySum != 530 ||
			resp.Tariffs[1].PeriodMin != 2 || resp.Tariffs[1].PeriodMax != 3 {
			t.Errorf("Tariffs[1] = %+v, ожидались код=483 сумма=530 сроки=2-3", resp.Tariffs[1])
		}
	})

	t.Run("реальный ответ с ошибками: тарифов нет", func(t *testing.T) {
		_, err := m.fromCDEKCalculatorResponse(loadFixture(t, "calculator_tarifflist.json"))
		if err == nil {
			t.Fatal("fromCDEKCalculatorResponse() ожидалась ошибка, получен nil")
		}
		if !strings.Contains(err.Error(), "no tariffs available") {
			t.Errorf("error = %v, ожидалась no tariffs available", err)
		}
	})

	t.Run("негативные разборы", func(t *testing.T) {
		if _, err := m.fromCDEKCalculatorResponse([]byte(`{`)); err == nil {
			t.Error("ожидалась ошибка для битого JSON")
		}
		if _, err := m.fromCDEKCalculatorResponse([]byte(`{}`)); err == nil {
			t.Error("ожидалась ошибка для ответа без tariff_codes")
		}
		if _, err := m.fromCDEKCalculatorResponse([]byte(`{"tariff_codes":[]}`)); err == nil {
			t.Error("ожидалась ошибка для пустого списка тарифов")
		}
		// Элемент-не-объект пропускается без ошибки
		resp, err := m.fromCDEKCalculatorResponse([]byte(`{"tariff_codes":["skip",{"tariff_code":1}]}`))
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if len(resp.Tariffs) != 1 || resp.Tariffs[0].TariffCode != 1 {
			t.Errorf("Tariffs = %+v, ожидался один тариф с кодом 1", resp.Tariffs)
		}
	})
}

// ========================
// fromCDEKOrderResponse (CreateOrder/UpdateOrder)
// ========================

// TestFromCDEKOrderResponseContract: варианты ответов создания/обновления
func TestFromCDEKOrderResponseContract(t *testing.T) {
	m := newDtoMapper()

	t.Run("успех: entity с uuid, cdek_number, tariff_code и статусами", func(t *testing.T) {
		resp, err := m.fromCDEKOrderResponse([]byte(`{
			"entity": {
				"uuid": "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
				"cdek_number": "11000000001",
				"tariff_code": 136,
				"statuses": [{"code": "ACCEPTED", "name": "Принят", "date_time": "2026-10-15T10:00:00+03:00", "city": "Москва"}],
				"requests": [{"date_time": "2026-10-15T09:00:00+03:00"}]
			}}`))
		if err != nil {
			t.Fatalf("fromCDEKOrderResponse() error = %v", err)
		}
		if resp.UUID != "72753031-95f6-4e0a-8dd5-1b125f46a3e5" {
			t.Errorf("UUID = %q", resp.UUID)
		}
		if resp.Number == nil || *resp.Number != "11000000001" {
			t.Errorf("Number = %v, ожидался 11000000001", resp.Number)
		}
		if resp.TariffCode != 136 {
			t.Errorf("TariffCode = %d, ожидался 136", resp.TariffCode)
		}
		if len(resp.Statuses) != 1 || resp.Statuses[0].Code != "ACCEPTED" || resp.Statuses[0].City == nil {
			t.Errorf("Statuses = %+v", resp.Statuses)
		}
		if resp.CreatedAt != "2026-10-15T09:00:00+03:00" {
			t.Errorf("CreatedAt = %q, ожидался date_time из entity.requests[0]", resp.CreatedAt)
		}
	})

	t.Run("202 Accepted: uuid из related_entities, дата из корневого requests", func(t *testing.T) {
		resp, err := m.fromCDEKOrderResponse([]byte(`{
			"related_entities": [
				{"type": "order", "uuid": "d3h6be25-2d77-4c56-b2cd-2b61b5f8c8da"}
			],
			"requests": [
				{"type": "CREATE", "state": "ACCEPTED", "date_time": "2026-10-15T09:00:00+03:00"}
			]}`))
		if err != nil {
			t.Fatalf("fromCDEKOrderResponse() error = %v", err)
		}
		if resp.UUID != "d3h6be25-2d77-4c56-b2cd-2b61b5f8c8da" {
			t.Errorf("UUID = %q, ожидался из related_entities", resp.UUID)
		}
		if resp.CreatedAt != "2026-10-15T09:00:00+03:00" {
			t.Errorf("CreatedAt = %q, ожидался из корневого requests[0]", resp.CreatedAt)
		}
	})

	t.Run("негативные разборы", func(t *testing.T) {
		if _, err := m.fromCDEKOrderResponse([]byte(`{`)); err == nil {
			t.Error("ожидалась ошибка для битого JSON")
		}
		if _, err := m.fromCDEKOrderResponse([]byte(`{}`)); err == nil {
			t.Error("ожидалась ошибка: uuid не найден")
		}
		// related_entities с не-объектами не ломают разбор, но без uuid — ошибка
		if _, err := m.fromCDEKOrderResponse([]byte(`{"related_entities":["x"]}`)); err == nil {
			t.Error("ожидалась ошибка: uuid не найден")
		}
	})
}

// TestFromCDEKIntakeMappersErrors: битый JSON у мапперов заявок на забор
func TestFromCDEKIntakeMappersErrors(t *testing.T) {
	m := newDtoMapper()

	if _, err := m.fromCDEKIntakeResponse([]byte(`{`)); err == nil {
		t.Error("fromCDEKIntakeResponse() ожидалась ошибка для битого JSON")
	}
	if _, err := m.fromCDEKIntakeInfo([]byte(`{`)); err == nil {
		t.Error("fromCDEKIntakeInfo() ожидалась ошибка для битого JSON")
	}

	// Ответ без entity — нулевой результат без ошибки
	resp, err := m.fromCDEKIntakeResponse([]byte(`{}`))
	if err != nil || resp.UUID != "" {
		t.Errorf("fromCDEKIntakeResponse(`{}`) = %+v, %v; ожидался пустой без ошибки", resp, err)
	}
	info, err := m.fromCDEKIntakeInfo([]byte(`{}`))
	if err != nil || info.UUID != "" {
		t.Errorf("fromCDEKIntakeInfo(`{}`) = %+v, %v; ожидался пустой без ошибки", info, err)
	}
}
