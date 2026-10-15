package cdek

// dto_mapper_requests_test.go — таблицы по всем полям мапперов запросов:
// toCDEKOrderRequest, toCDEKUpdateOrderRequest, toCDEKCalculatorRequest,
// toCDEKIntakeRequest. Проверяется состав тела, маппинг типов заказа,
// ИНН/паспортные данные контактов, seller, packages/items и XOR «ПВЗ vs адрес».

import (
	"errors"
	"reflect"
	"testing"
)

// ptrInt — указатель на int (для Seller.OwnershipForm)
func ptrInt(i int) *int { return &i }

// firstLengthValue читает габарит CalcPackageRequestDto (interface{} поле)
func firstLengthValue(pkg CalcPackageRequestDto, field string) (int32, bool) {
	var raw interface{}
	switch field {
	case "length":
		raw = pkg.Length
	case "width":
		raw = pkg.Width
	case "height":
		raw = pkg.Height
	}
	v, ok := raw.(*int32)
	if !ok {
		return 0, false
	}
	return *v, true
}

// fullRecipient — контакт со всеми опциональными полями
func fullRecipient() Recipient {
	return Recipient{
		Contact: Contact{
			Company: ptrString("ООО «Ромашка»"),
			Name:    "Иван Иванов",
			Email:   ptrString("ivan@example.com"),
			Phones: []Phone{
				{Number: "+79001112233", Additional: ptrString("123")},
				{Number: "+79004445566"},
			},
		},
		TIN:                  ptrString("7709033880"),
		PassportSeries:       ptrString("4510"),
		PassportNumber:       ptrString("123456"),
		PassportDateOfIssue:  ptrString("2020-01-01"),
		PassportOrganization: ptrString("ОВД Москва"),
		PassportDateOfBirth:  ptrString("1990-05-05"),
	}
}

func TestToCDEKOrderRequest(t *testing.T) {
	m := newDtoMapper()

	t.Run("все поля: ИНН, паспорт, seller, packages с габаритами и items", func(t *testing.T) {
		comment := "оставить у двери"
		req := &OrderRequest{
			Type:       "online_store",
			TariffCode: 137,
			Comment:    &comment,
			Sender: Recipient{
				Contact: Contact{
					Company: ptrString("ООО «Отправитель»"),
					Name:    "Сидор Сидоров",
					Email:   ptrString("sender@example.com"),
					Phones:  []Phone{{Number: "+79007778899", Additional: ptrString("42")}},
				},
				TIN:                  ptrString("7801234567"),
				PassportSeries:       ptrString("4510"),
				PassportNumber:       ptrString("654321"),
				PassportDateOfIssue:  ptrString("2015-06-01"),
				PassportOrganization: ptrString("ОВД Санкт-Петербург"),
				PassportDateOfBirth:  ptrString("1985-03-03"),
			},
			Recipient: fullRecipient(),
			Seller: &Seller{
				Name:          ptrString("ООО «Продавец»"),
				INN:           ptrString("7709033880"),
				Phone:         ptrString("+70000000000"),
				OwnershipForm: ptrInt(63),
				Address:       ptrString("г. Москва, ул. Тестовая, 1"),
			},
			FromLocation: Location{
				Code:        ptrInt32(44),
				FiasGUID:    ptrString("0c5b2444-70a0-4932-980c-b4dc0d3f02df"),
				PostalCode:  ptrString("125167"),
				CountryCode: ptrString("RU"),
				Region:      ptrString("Москва"),
				City:        ptrString("Москва"),
				Address:     ptrString("Ленинградский пр-т, 39"),
			},
			ToLocation: Location{
				Code:    ptrInt32(250),
				City:    ptrString("Санкт-Петербург"),
				Address: ptrString("Невский пр-т, 1"),
			},
			Packages: []OrderPackage{
				{
					Number: "1", Weight: 1500,
					Length: ptrInt32(10), Width: ptrInt32(20), Height: ptrInt32(30),
					Comment: ptrString("хрупкое"),
					Items: []Item{
						{Name: "Товар 1", WareKey: "SKU-1", Payment: 100, Cost: 500, Weight: 700, Amount: 2},
					},
				},
				{Number: "2", Weight: 500},
			},
		}

		got, err := m.toCDEKOrderRequest(req)
		if err != nil {
			t.Fatalf("toCDEKOrderRequest() error = %v", err)
		}

		if got["type"] != 1 {
			t.Errorf("type = %v, ожидался 1 (online_store → число)", got["type"])
		}
		if got["tariff_code"] != 137 {
			t.Errorf("tariff_code = %v, ожидался 137", got["tariff_code"])
		}
		if got["comment"] != comment {
			t.Errorf("comment = %v, ожидался %q", got["comment"], comment)
		}

		sender, ok := got["sender"].(map[string]interface{})
		if !ok {
			t.Fatalf("sender не объект: %v", got["sender"])
		}
		wantSender := map[string]interface{}{ //nolint:gosec // синтетические паспортные данные теста
			"company":                "ООО «Отправитель»",
			"name":                   "Сидор Сидоров",
			"email":                  "sender@example.com",
			"phones":                 []map[string]interface{}{{"number": "+79007778899", "additional": "42"}},
			"tin":                    "7801234567",
			"passport_series":        "4510",
			"passport_number":        "654321",
			"passport_date_of_issue": "2015-06-01",
			"passport_organization":  "ОВД Санкт-Петербург",
			"passport_date_of_birth": "1985-03-03",
		}
		if !reflect.DeepEqual(sender, wantSender) {
			t.Errorf("sender = %v\nожидался  %v", sender, wantSender)
		}

		recipient, ok := got["recipient"].(map[string]interface{})
		if !ok {
			t.Fatalf("recipient не объект: %v", got["recipient"])
		}
		// Тестовые контактные данные, не реальные креды
		wantRecipient := map[string]interface{}{ //nolint:gosec // синтетические паспортные данные фикстуры
			"company":                "ООО «Ромашка»",
			"name":                   "Иван Иванов",
			"email":                  "ivan@example.com",
			"phones":                 []map[string]interface{}{{"number": "+79001112233", "additional": "123"}, {"number": "+79004445566"}},
			"tin":                    "7709033880",
			"passport_series":        "4510",
			"passport_number":        "123456",
			"passport_date_of_issue": "2020-01-01",
			"passport_organization":  "ОВД Москва",
			"passport_date_of_birth": "1990-05-05",
		}
		if !reflect.DeepEqual(recipient, wantRecipient) {
			t.Errorf("recipient = %v\nожидался   %v", recipient, wantRecipient)
		}

		seller, ok := got["seller"].(map[string]interface{})
		if !ok {
			t.Fatalf("seller не объект: %v", got["seller"])
		}
		wantSeller := map[string]interface{}{
			"name":           "ООО «Продавец»",
			"inn":            "7709033880",
			"phone":          "+70000000000",
			"ownership_form": 63,
			"address":        "г. Москва, ул. Тестовая, 1",
		}
		if !reflect.DeepEqual(seller, wantSeller) {
			t.Errorf("seller = %v\nожидался   %v", seller, wantSeller)
		}

		wantFrom := map[string]interface{}{
			"code":         int32(44),
			"fias_guid":    "0c5b2444-70a0-4932-980c-b4dc0d3f02df",
			"postal_code":  "125167",
			"country_code": "RU",
			"region":       "Москва",
			"city":         "Москва",
			"address":      "Ленинградский пр-т, 39",
		}
		if !reflect.DeepEqual(got["from_location"], wantFrom) {
			t.Errorf("from_location = %v\nожидался       %v", got["from_location"], wantFrom)
		}
		wantTo := map[string]interface{}{
			"code":    int32(250),
			"city":    "Санкт-Петербург",
			"address": "Невский пр-т, 1",
		}
		if !reflect.DeepEqual(got["to_location"], wantTo) {
			t.Errorf("to_location = %v\nожидался     %v", got["to_location"], wantTo)
		}

		pkgs, ok := got["packages"].([]map[string]interface{})
		if !ok || len(pkgs) != 2 {
			t.Fatalf("packages = %#v, ожидались 2 упаковки-объекта", got["packages"])
		}
		// comment отправляется когда задан (некоторые договоры CDEK его требуют);
		// пустые items не отправляются (для type=2 они необязательны)
		wantPkg0 := map[string]interface{}{
			"number": "1", "weight": int32(1500),
			"length": int32(10), "width": int32(20), "height": int32(30),
			"comment": "хрупкое",
			"items": []map[string]interface{}{
				{
					"name": "Товар 1", "ware_key": "SKU-1",
					"payment": map[string]interface{}{"value": float64(100)},
					"cost":    float64(500), "weight": int32(700), "amount": int32(2),
				},
			},
		}
		if !reflect.DeepEqual(pkgs[0], wantPkg0) {
			t.Errorf("packages[0] = %v\nожидался      %v", pkgs[0], wantPkg0)
		}
		wantPkg1 := map[string]interface{}{
			"number": "2", "weight": int32(500),
		}
		if !reflect.DeepEqual(pkgs[1], wantPkg1) {
			t.Errorf("packages[1] = %v, ожидался %v", pkgs[1], wantPkg1)
		}
		_ = comment // comment заказа проверяется выше
	})

	t.Run("type маппится в числа: delivery→2, неизвестное→1", func(t *testing.T) {
		for _, tt := range []struct {
			reqType string
			want    int
		}{
			{reqType: "delivery", want: 2},
			{reqType: "2", want: 2},
			{reqType: "online_store", want: 1},
			{reqType: "1", want: 1},
			{reqType: "мусор", want: 1},
			{reqType: "", want: 1},
		} {
			req := baseOrderRequest()
			req.Type = tt.reqType
			got, err := m.toCDEKOrderRequest(req)
			if err != nil {
				t.Fatalf("toCDEKOrderRequest(%q) error = %v", tt.reqType, err)
			}
			if got["type"] != tt.want {
				t.Errorf("type(%q) = %v, ожидался %d", tt.reqType, got["type"], tt.want)
			}
		}
	})

	t.Run("ПВЗ вместо адресов: shipment_point/delivery_point, location-блоков нет", func(t *testing.T) {
		req := baseOrderRequest()
		req.ShipmentPoint = ptrString("MSK109")
		req.DeliveryPoint = ptrString("SPB12")
		req.Recipient = fullRecipient()

		got, err := m.toCDEKOrderRequest(req)
		if err != nil {
			t.Fatalf("toCDEKOrderRequest() error = %v", err)
		}
		if got["shipment_point"] != "MSK109" || got["delivery_point"] != "SPB12" {
			t.Errorf("points = %v/%v", got["shipment_point"], got["delivery_point"])
		}
		if _, ok := got["from_location"]; ok {
			t.Error("from_location отправлен вместе с shipment_point")
		}
		if _, ok := got["to_location"]; ok {
			t.Error("to_location отправлен вместе с delivery_point")
		}
	})

	t.Run("пустой sender не отправляется", func(t *testing.T) {
		req := baseOrderRequest()
		got, err := m.toCDEKOrderRequest(req)
		if err != nil {
			t.Fatalf("toCDEKOrderRequest() error = %v", err)
		}
		if _, ok := got["sender"]; ok {
			t.Error("sender отправлен для пустого контакта")
		}
		if _, ok := got["seller"]; ok {
			t.Error("seller отправлен для nil-контакта")
		}
	})

	t.Run("конфликт ПВЗ+адрес — ErrInvalidRequest", func(t *testing.T) {
		req := baseOrderRequest()
		req.ShipmentPoint = ptrString("MSK109")
		req.FromLocation = Location{Code: ptrInt32(44)}
		if _, err := m.toCDEKOrderRequest(req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("shipment_point+from_location: error = %v, ожидалась обертка ErrInvalidRequest", err)
		}

		req = baseOrderRequest()
		req.DeliveryPoint = ptrString("SPB12")
		req.ToLocation = Location{City: ptrString("Пермь")}
		if _, err := m.toCDEKOrderRequest(req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("delivery_point+to_location: error = %v, ожидалась обертка ErrInvalidRequest", err)
		}
	})
}

func TestToCDEKUpdateOrderRequest(t *testing.T) {
	m := newDtoMapper()

	t.Run("uuid и type всегда согласованы, контактные поля маппятся", func(t *testing.T) {
		comment := "новый комментарий"
		recipient := fullRecipient()
		req := &UpdateOrderRequest{
			OrderUUID: "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
			Type:      "delivery",
			Recipient: &recipient,
			ToLocation: &Location{
				Code:    ptrInt32(250),
				City:    ptrString("Санкт-Петербург"),
				Address: ptrString("Невский пр-т, 1"),
			},
			Comment: &comment,
			Packages: []OrderPackage{
				{Number: "1", Weight: 900, Items: []Item{
					{Name: "Товар", WareKey: "SKU-1", Cost: 300, Weight: 900, Amount: 1},
				}},
			},
		}

		got, err := m.toCDEKUpdateOrderRequest(req)
		if err != nil {
			t.Fatalf("toCDEKUpdateOrderRequest() error = %v", err)
		}

		if got["uuid"] != req.OrderUUID {
			t.Errorf("uuid = %v, ожидался %q", got["uuid"], req.OrderUUID)
		}
		if got["type"] != 2 {
			t.Errorf("type = %v, ожидался 2 (delivery)", got["type"])
		}
		if _, ok := got["recipient"].(map[string]interface{}); !ok {
			t.Fatalf("recipient не объект: %v", got["recipient"])
		}
		wantTo := map[string]interface{}{
			"code":    int32(250),
			"city":    "Санкт-Петербург",
			"address": "Невский пр-т, 1",
		}
		if !reflect.DeepEqual(got["to_location"], wantTo) {
			t.Errorf("to_location = %v, ожидался %v", got["to_location"], wantTo)
		}
		if got["comment"] != comment {
			t.Errorf("comment = %v", got["comment"])
		}
		pkgs, ok := got["packages"].([]map[string]interface{})
		if !ok || len(pkgs) != 1 {
			t.Fatalf("packages = %#v", got["packages"])
		}
		if pkgs[0]["number"] != "1" || pkgs[0]["weight"] != int32(900) {
			t.Errorf("packages[0] = %v", pkgs[0])
		}
	})

	t.Run("минимальный PATCH: только uuid, без type и контактных блоков", func(t *testing.T) {
		got, err := m.toCDEKUpdateOrderRequest(&UpdateOrderRequest{
			OrderUUID: "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
		})
		if err != nil {
			t.Fatalf("toCDEKUpdateOrderRequest() error = %v", err)
		}
		if len(got) != 1 || got["uuid"] == nil {
			t.Errorf("ожидался только uuid, получено %v", got)
		}
	})

	t.Run("sender и seller обновляются, shipment_point без адреса", func(t *testing.T) {
		sender := fullRecipient()
		req := &UpdateOrderRequest{
			OrderUUID: "72753031-95f6-4e0a-8dd5-1b125f46a3e5",
			Type:      "1",
			Sender:    &sender,
			Seller: &Seller{
				Name:          ptrString("ООО «Продавец»"),
				INN:           ptrString("7709033880"),
				Phone:         ptrString("+70000000000"),
				OwnershipForm: ptrInt(63),
				Address:       ptrString("г. Москва, ул. Тестовая, 1"),
			},
			ShipmentPoint: ptrString("MSK109"),
		}
		got, err := m.toCDEKUpdateOrderRequest(req)
		if err != nil {
			t.Fatalf("toCDEKUpdateOrderRequest() error = %v", err)
		}
		if got["type"] != 1 {
			t.Errorf("type = %v, ожидался 1", got["type"])
		}
		if sender, ok := got["sender"].(map[string]interface{}); !ok || sender["tin"] != "7709033880" {
			t.Errorf("sender = %v, ожидался объект с ИНН", got["sender"])
		}
		wantSeller := map[string]interface{}{
			"name":           "ООО «Продавец»",
			"inn":            "7709033880",
			"phone":          "+70000000000",
			"ownership_form": 63,
			"address":        "г. Москва, ул. Тестовая, 1",
		}
		if seller, ok := got["seller"].(map[string]interface{}); !ok || !reflect.DeepEqual(seller, wantSeller) {
			t.Errorf("seller = %v\nожидался  %v", got["seller"], wantSeller)
		}
		if got["shipment_point"] != "MSK109" {
			t.Errorf("shipment_point = %v", got["shipment_point"])
		}
		if _, ok := got["from_location"]; ok {
			t.Error("from_location отправлен вместе с shipment_point")
		}
	})

	t.Run("конфликт ПВЗ+адрес — ErrInvalidRequest", func(t *testing.T) {
		req := &UpdateOrderRequest{
			OrderUUID:     "u",
			DeliveryPoint: ptrString("SPB12"),
			ToLocation:    &Location{Code: ptrInt32(137)},
		}
		if _, err := m.toCDEKUpdateOrderRequest(req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("delivery_point+to_location: error = %v, ожидалась обертка ErrInvalidRequest", err)
		}

		req = &UpdateOrderRequest{
			OrderUUID:     "u",
			ShipmentPoint: ptrString("MSK109"),
			FromLocation:  &Location{Code: ptrInt32(44)},
		}
		if _, err := m.toCDEKUpdateOrderRequest(req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("shipment_point+from_location: error = %v, ожидалась обертка ErrInvalidRequest", err)
		}
	})
}

func TestToCDEKCalculatorRequest(t *testing.T) {
	m := newDtoMapper()

	req := &CostRequest{
		FromCityCode: 44,
		ToCityCode:   250,
		Packages: []Package{
			{Weight: 1500, Length: 10, Width: 20, Height: 30},
			{Weight: 500, Length: 5, Width: 5, Height: 5},
		},
	}

	got := m.toCDEKCalculatorRequest(req)

	// Сгенерированные поля DTO — interface{}; маппер кладет туда *int32
	gotType, ok := got.Type.(*int32)
	if !ok || *gotType != 1 {
		t.Errorf("type = %#v, ожидался *int32(1) — интернет-магазин", got.Type)
	}
	gotCurrency, ok := got.Currency.(*int32)
	if !ok || *gotCurrency != 1 {
		t.Errorf("currency = %#v, ожидался *int32(1) — рубли", got.Currency)
	}
	fromCode, ok := got.FromLocation.Code.(*int32)
	if !ok || *fromCode != 44 {
		t.Errorf("from.code = %#v, ожидался 44", got.FromLocation.Code)
	}
	toCode, ok := got.ToLocation.Code.(*int32)
	if !ok || *toCode != 250 {
		t.Errorf("to.code = %#v, ожидался 250", got.ToLocation.Code)
	}
	pkgs, ok := got.Packages.([]CalcPackageRequestDto)
	if !ok || len(pkgs) != 2 {
		t.Fatalf("packages = %#v, ожидались 2 CalcPackageRequestDto", got.Packages)
	}
	first := pkgs[0]
	if first.Weight != int32(1500) {
		t.Errorf("packages[0].weight = %#v, ожидался 1500", first.Weight)
	}
	for field, want := range map[string]int32{"length": 10, "width": 20, "height": 30} {
		v, ok := firstLengthValue(first, field)
		if !ok || v != want {
			t.Errorf("packages[0].%s = %#v, ожидался %d", field, v, want)
		}
	}
	if pkgs[1].Weight != int32(500) {
		t.Errorf("packages[1].weight = %#v", pkgs[1].Weight)
	}

	t.Run("пустой список упаковок — пустой массив, не nil", func(t *testing.T) {
		got := m.toCDEKCalculatorRequest(&CostRequest{FromCityCode: 1, ToCityCode: 2})
		pkgs, ok := got.Packages.([]CalcPackageRequestDto)
		if !ok || len(pkgs) != 0 {
			t.Errorf("packages = %#v, ожидался пустой срез", got.Packages)
		}
	})
}

func TestToCDEKIntakeRequest(t *testing.T) {
	m := newDtoMapper()

	t.Run("полный запрос", func(t *testing.T) {
		comment := "звонок за час"
		req := &IntakeRequest{
			IntakeDate:     "2026-10-20",
			IntakeTimeFrom: "10:00",
			IntakeTimeTo:   "18:00",
			Comment:        &comment,
			Sender: Contact{
				Name:   "ООО «Отправитель»",
				Phones: []Phone{{Number: "+79007778899"}},
			},
		}
		got, err := m.toCDEKIntakeRequest(req)
		if err != nil {
			t.Fatalf("toCDEKIntakeRequest() error = %v", err)
		}
		want := map[string]interface{}{
			"intake_date":      "2026-10-20",
			"intake_time_from": "10:00",
			"intake_time_to":   "18:00",
			"sender":           map[string]interface{}{"name": "ООО «Отправитель»"},
			"comment":          comment,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got = %v\nwant = %v", got, want)
		}
	})

	t.Run("без комментария — ключа нет", func(t *testing.T) {
		got, err := m.toCDEKIntakeRequest(&IntakeRequest{
			IntakeDate:     "2026-10-20",
			IntakeTimeFrom: "10:00",
			IntakeTimeTo:   "18:00",
			Sender:         Contact{Name: "Иван"},
		})
		if err != nil {
			t.Fatalf("toCDEKIntakeRequest() error = %v", err)
		}
		if _, ok := got["comment"]; ok {
			t.Error("comment отправлен для nil")
		}
	})
}
