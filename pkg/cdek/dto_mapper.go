package cdek

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// dtoMapper - компонент преобразования между Service API и автогенерированными CDEK типами (camelCase по регламенту 4.6)
type dtoMapper struct{}

// newDtoMapper создает новый маппер DTO
func newDtoMapper() *dtoMapper {
	return &dtoMapper{}
}

// ========================
// Calculator (Cost Estimation)
// ========================

// toCDEKCalculatorRequest преобразует CostRequest → CalculatorTariffListRequestDto
func (m *dtoMapper) toCDEKCalculatorRequest(req *CostRequest) CalculatorTariffListRequestDto {
	// Тип услуги: 1 = интернет-магазин
	serviceType := int32(1)
	// Валюта: 1 = рубли
	currency := int32(1)

	// Преобразование упаковок
	packages := make([]CalcPackageRequestDto, len(req.Packages))
	for i, pkg := range req.Packages {
		packages[i] = CalcPackageRequestDto{
			Weight: pkg.Weight,
			Length: &pkg.Length,
			Width:  &pkg.Width,
			Height: &pkg.Height,
		}
	}

	return CalculatorTariffListRequestDto{
		Type:     &serviceType,
		Currency: &currency,
		FromLocation: CalculatorLocationDto{
			Code: &req.FromCityCode,
		},
		ToLocation: CalculatorLocationDto{
			Code: &req.ToCityCode,
		},
		Packages: packages,
	}
}

// fromCDEKCalculatorResponse преобразует CalculatorTariffListResponseDto → CostResponse
func (m *dtoMapper) fromCDEKCalculatorResponse(data []byte) (*CostResponse, error) {
	// Парсим как map т.к. автогенерированные типы используют interface{}
	var rawResp map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal calculator response: %w", err)
	}

	// Проверяем наличие тарифов
	tariffCodes, ok := rawResp["tariff_codes"].([]interface{})
	if !ok || len(tariffCodes) == 0 {
		return nil, fmt.Errorf("no tariffs available for this route")
	}

	// Преобразование тарифов
	tariffs := make([]TariffOption, 0, len(tariffCodes))
	for _, t := range tariffCodes {
		tariffMap, ok := t.(map[string]interface{})
		if !ok {
			continue
		}

		tariff := TariffOption{}

		if code, ok := tariffMap["tariff_code"].(float64); ok {
			tariff.TariffCode = int(code)
		}
		if name, ok := tariffMap["tariff_name"].(string); ok {
			tariff.TariffName = name
		}
		if mode, ok := tariffMap["delivery_mode"].(float64); ok {
			tariff.DeliveryMode = int(mode)
		}
		if sum, ok := tariffMap["delivery_sum"].(float64); ok {
			tariff.DeliverySum = sum
		}
		if periodMin, ok := tariffMap["period_min"].(float64); ok {
			tariff.PeriodMin = int(periodMin)
		}
		if periodMax, ok := tariffMap["period_max"].(float64); ok {
			tariff.PeriodMax = int(periodMax)
		}

		tariffs = append(tariffs, tariff)
	}

	return &CostResponse{
		Tariffs: tariffs,
	}, nil
}

// ========================
// Orders
// ========================

// locationToMap преобразует Location → map для API.
// Возвращает nil если ни одно поле не задано — пустой адресный блок не отправляется
func (m *dtoMapper) locationToMap(loc *Location) map[string]interface{} {
	if loc == nil {
		return nil
	}
	l := make(map[string]interface{})
	if loc.Code != nil {
		l["code"] = *loc.Code
	}
	if loc.FiasGUID != nil {
		l["fias_guid"] = *loc.FiasGUID
	}
	if loc.PostalCode != nil {
		l["postal_code"] = *loc.PostalCode
	}
	if loc.CountryCode != nil {
		l["country_code"] = *loc.CountryCode
	}
	if loc.Region != nil {
		l["region"] = *loc.Region
	}
	if loc.City != nil {
		l["city"] = *loc.City
	}
	if loc.Address != nil {
		l["address"] = *loc.Address
	}
	if len(l) == 0 {
		return nil
	}
	return l
}

// pointCode возвращает код ПВЗ или "" если он не задан
func pointCode(p *string) string {
	if p == nil || *p == "" {
		return ""
	}
	return *p
}

// locationFilled сообщает, задано ли хотя бы одно адресное поле Location
// (пустой адресный блок эквивалентен отсутствию адреса)
func locationFilled(loc *Location) bool {
	return loc != nil && (loc.Code != nil || loc.FiasGUID != nil || loc.PostalCode != nil ||
		loc.CountryCode != nil || loc.Region != nil || loc.City != nil || loc.Address != nil)
}

// pointAddressConflictErr builds the XOR error: a PVZ code and an address
// for the same side of the route are mutually exclusive in the CDEK API
func pointAddressConflictErr(pointField, locField, side string) error {
	return fmt.Errorf("%s and %s are mutually exclusive: pass either the %s PVZ code or the %s address, not both: %w",
		pointField, locField, side, side, ErrInvalidRequest)
}

// orderTypeCode преобразует строковый тип заказа в числительный код CDEK API:
// 1 - "интернет-магазин" ("online_store"/"1"), 2 - "доставка" ("delivery"/"2").
// Пустая строка и неизвестное значение трактуются как 1 (как раньше в создании заказа)
func orderTypeCode(t string) int {
	switch t {
	case "delivery", "2":
		return 2
	case "online_store", "1":
		return 1
	default:
		return 1
	}
}

// orderTypeString обратен orderTypeCode: численный тип заказа из ответа CDEK
// (1/2) в строковый. Неизвестные числа возвращаются строкой цифр как есть
func orderTypeString(code int) string {
	switch code {
	case 1:
		return "online_store"
	case 2:
		return "delivery"
	default:
		return strconv.Itoa(code)
	}
}

// toCDEKOrderRequest преобразует OrderRequest → map для OrderCreateRequestDto
// Используем map[string]interface{} т.к. автогенерированные типы используют interface{}
func (m *dtoMapper) toCDEKOrderRequest(req *OrderRequest) (map[string]interface{}, error) {
	order := make(map[string]interface{})

	// Обязательные поля — CDEK v2 expects integer type: 1=online_store, 2=delivery
	order["type"] = orderTypeCode(req.Type)
	order["tariff_code"] = req.TariffCode

	// Опциональный комментарий
	if req.Comment != nil {
		order["comment"] = *req.Comment
	}

	// Отправитель (теперь с поддержкой ИНН и паспортных данных)
	if req.Sender.Name != "" {
		sender := map[string]interface{}{
			"name": req.Sender.Name,
		}
		if req.Sender.Company != nil {
			sender["company"] = *req.Sender.Company
		}
		if req.Sender.Email != nil {
			sender["email"] = *req.Sender.Email
		}
		if len(req.Sender.Phones) > 0 {
			phones := make([]map[string]interface{}, len(req.Sender.Phones))
			for i, phone := range req.Sender.Phones {
				p := map[string]interface{}{"number": phone.Number}
				if phone.Additional != nil {
					p["additional"] = *phone.Additional
				}
				phones[i] = p
			}
			sender["phones"] = phones
		}
		// ИНН для отправителя (компания или ИП)
		if req.Sender.TIN != nil {
			sender["tin"] = *req.Sender.TIN
		}
		// Паспортные данные для отправителя (физлицо)
		if req.Sender.PassportSeries != nil {
			sender["passport_series"] = *req.Sender.PassportSeries
		}
		if req.Sender.PassportNumber != nil {
			sender["passport_number"] = *req.Sender.PassportNumber
		}
		if req.Sender.PassportDateOfIssue != nil {
			sender["passport_date_of_issue"] = *req.Sender.PassportDateOfIssue
		}
		if req.Sender.PassportOrganization != nil {
			sender["passport_organization"] = *req.Sender.PassportOrganization
		}
		if req.Sender.PassportDateOfBirth != nil {
			sender["passport_date_of_birth"] = *req.Sender.PassportDateOfBirth
		}
		order["sender"] = sender
	}

	// Получатель (обязательный)
	recipient := map[string]interface{}{
		"name": req.Recipient.Name,
	}
	if req.Recipient.Company != nil {
		recipient["company"] = *req.Recipient.Company
	}
	if req.Recipient.Email != nil {
		recipient["email"] = *req.Recipient.Email
	}
	if len(req.Recipient.Phones) > 0 {
		phones := make([]map[string]interface{}, len(req.Recipient.Phones))
		for i, phone := range req.Recipient.Phones {
			p := map[string]interface{}{"number": phone.Number}
			if phone.Additional != nil {
				p["additional"] = *phone.Additional
			}
			phones[i] = p
		}
		recipient["phones"] = phones
	}
	// ИНН (для юридических лиц и ИП)
	if req.Recipient.TIN != nil {
		recipient["tin"] = *req.Recipient.TIN
	}
	// Паспортные данные (для физических лиц)
	if req.Recipient.PassportSeries != nil {
		recipient["passport_series"] = *req.Recipient.PassportSeries
	}
	if req.Recipient.PassportNumber != nil {
		recipient["passport_number"] = *req.Recipient.PassportNumber
	}
	if req.Recipient.PassportDateOfIssue != nil {
		recipient["passport_date_of_issue"] = *req.Recipient.PassportDateOfIssue
	}
	if req.Recipient.PassportOrganization != nil {
		recipient["passport_organization"] = *req.Recipient.PassportOrganization
	}
	if req.Recipient.PassportDateOfBirth != nil {
		recipient["passport_date_of_birth"] = *req.Recipient.PassportDateOfBirth
	}
	order["recipient"] = recipient

	// Истинный продавец (третье лицо) - только для интернет-магазинов
	if req.Seller != nil {
		seller := make(map[string]interface{})
		if req.Seller.Name != nil {
			seller["name"] = *req.Seller.Name
		}
		if req.Seller.INN != nil {
			seller["inn"] = *req.Seller.INN
		}
		if req.Seller.Phone != nil {
			seller["phone"] = *req.Seller.Phone
		}
		if req.Seller.OwnershipForm != nil {
			seller["ownership_form"] = *req.Seller.OwnershipForm
		}
		if req.Seller.Address != nil {
			seller["address"] = *req.Seller.Address
		}
		order["seller"] = seller
	}

	// Сторона отправителя: код ПВЗ и адрес взаимоисключающие (XOR).
	// Конфликт отвергается orderValidator при CreateOrder; здесь защита от прямого вызова маппера
	if code := pointCode(req.ShipmentPoint); code != "" {
		if locationFilled(&req.FromLocation) {
			return nil, pointAddressConflictErr("shipment_point", "from_location", "sender")
		}
		order["shipment_point"] = code
	} else if fromLoc := m.locationToMap(&req.FromLocation); fromLoc != nil {
		order["from_location"] = fromLoc
	}

	// Сторона получателя: код ПВЗ и адрес взаимоисключающие (XOR)
	if code := pointCode(req.DeliveryPoint); code != "" {
		if locationFilled(&req.ToLocation) {
			return nil, pointAddressConflictErr("delivery_point", "to_location", "recipient")
		}
		order["delivery_point"] = code
	} else if toLoc := m.locationToMap(&req.ToLocation); toLoc != nil {
		order["to_location"] = toLoc
	}

	// Упаковки (обязательно)
	packages := make([]map[string]interface{}, len(req.Packages))
	for i, pkg := range req.Packages {
		packages[i] = orderPackageToMap(pkg)
	}
	order["packages"] = packages

	return order, nil
}

// fromCDEKOrderResponse преобразует ответ API → OrderResponse
func (m *dtoMapper) fromCDEKOrderResponse(data []byte) (*OrderResponse, error) {
	var rawResp map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal order response: %w", err)
	}

	orderResp := &OrderResponse{}

	// CreateOrder возвращает либо entity (успех), либо requests (создание запроса)
	// Пробуем entity сначала (для успешного создания)
	if entity, ok := rawResp["entity"].(map[string]interface{}); ok {
		// UUID (обязательный)
		if uuid, ok := entity["uuid"].(string); ok {
			orderResp.UUID = uuid
		}

		// Номер заказа CDEK (может быть null)
		if cdekNum, ok := entity["cdek_number"].(string); ok {
			orderResp.Number = &cdekNum
		}

		// Код тарифа
		if tariff, ok := entity["tariff_code"].(float64); ok {
			orderResp.TariffCode = int(tariff)
		}

		// Статусы
		if statuses, ok := entity["statuses"].([]interface{}); ok {
			orderResp.Statuses = make([]StatusEvent, 0, len(statuses))
			for _, s := range statuses {
				if status, ok := s.(map[string]interface{}); ok {
					event := StatusEvent{}
					if code, ok := status["code"].(string); ok {
						event.Code = code
					}
					if name, ok := status["name"].(string); ok {
						event.Name = name
					}
					if dt, ok := status["date_time"].(string); ok {
						event.DateTime = dt
					}
					if city, ok := status["city"].(string); ok {
						event.City = &city
					}
					orderResp.Statuses = append(orderResp.Statuses, event)
				}
			}
		}

		// Дата создания из requests внутри entity
		if requests, ok := entity["requests"].([]interface{}); ok && len(requests) > 0 {
			if firstReq, ok := requests[0].(map[string]interface{}); ok {
				if dt, ok := firstReq["date_time"].(string); ok {
					orderResp.CreatedAt = dt
				}
			}
		}
	}

	// Если нет entity, используем requests на верхнем уровне
	// Это происходит при асинхронном создании заказа
	if orderResp.UUID == "" {
		// Ищем UUID в related_entities
		if relatedEntities, ok := rawResp["related_entities"].([]interface{}); ok && len(relatedEntities) > 0 {
			for _, rel := range relatedEntities {
				if relMap, ok := rel.(map[string]interface{}); ok {
					if uuid, ok := relMap["uuid"].(string); ok {
						orderResp.UUID = uuid
						break
					}
				}
			}
		}

		// Дата создания из requests на верхнем уровне
		if requests, ok := rawResp["requests"].([]interface{}); ok && len(requests) > 0 {
			if firstReq, ok := requests[0].(map[string]interface{}); ok {
				if dt, ok := firstReq["date_time"].(string); ok {
					orderResp.CreatedAt = dt
				}
			}
		}
	}

	if orderResp.UUID == "" {
		return nil, fmt.Errorf("uuid not found in response")
	}

	return orderResp, nil
}

// fromCDEKOrderToTracking преобразует GetOrder ответ → TrackingInfo
func (m *dtoMapper) fromCDEKOrderToTracking(data []byte) (*TrackingInfo, error) {
	var rawResp map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal tracking response: %w", err)
	}

	// Проверяем наличие entity
	entity, ok := rawResp["entity"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("entity not found in response")
	}

	tracking := &TrackingInfo{}

	// UUID (обязательный)
	if uuid, ok := entity["uuid"].(string); ok {
		tracking.UUID = uuid
	}

	// Номер заказа
	if cdekNum, ok := entity["cdek_number"].(string); ok {
		tracking.Number = &cdekNum
	}

	// История статусов
	if statuses, ok := entity["statuses"].([]interface{}); ok && len(statuses) > 0 {
		tracking.StatusHistory = make([]StatusEvent, 0, len(statuses))
		for _, s := range statuses {
			if status, ok := s.(map[string]interface{}); ok {
				event := StatusEvent{}
				if code, ok := status["code"].(string); ok {
					event.Code = code
				}
				if name, ok := status["name"].(string); ok {
					event.Name = name
				}
				if dt, ok := status["date_time"].(string); ok {
					event.DateTime = dt
				}
				if city, ok := status["city"].(string); ok {
					event.City = &city
				}
				tracking.StatusHistory = append(tracking.StatusHistory, event)
			}
		}

		// Текущий статус - последний в списке
		if len(tracking.StatusHistory) > 0 {
			tracking.CurrentStatus = tracking.StatusHistory[len(tracking.StatusHistory)-1]
		}
	}

	// Плановая дата доставки
	if delivery, ok := entity["delivery_detail"].(map[string]interface{}); ok {
		if date, ok := delivery["date"].(string); ok {
			tracking.EstimatedDelivery = &date
		}
	}

	return tracking, nil
}

// ========================
// Delivery Points
// ========================

// fromCDEKDeliveryPoints преобразует GetDeliverypoints ответ → []DeliveryPoint
func (m *dtoMapper) fromCDEKDeliveryPoints(data []byte) ([]DeliveryPoint, error) {
	var rawResp []map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal delivery points: %w", err)
	}

	points := make([]DeliveryPoint, 0, len(rawResp))
	for _, p := range rawResp {
		point := DeliveryPoint{}

		if code, ok := p["code"].(string); ok {
			point.Code = code
		}
		if name, ok := p["name"].(string); ok {
			point.Name = name
		}
		if pvzType, ok := p["type"].(string); ok {
			point.Type = pvzType
		}
		if workTime, ok := p["work_time"].(string); ok {
			point.WorkTime = workTime
		}
		if email, ok := p["email"].(string); ok {
			point.Email = &email
		}
		if note, ok := p["note"].(string); ok {
			point.Note = &note
		}
		// Числовые поля приходят как float64 (encoding/json для interface{})
		if v, ok := p["have_cashless"].(bool); ok {
			point.HaveCashless = v
		}
		if v, ok := p["have_cash"].(bool); ok {
			point.HaveCash = v
		}
		if v, ok := p["allowed_cod"].(bool); ok {
			point.AllowedCOD = v
		}
		if v, ok := p["is_dressing_room"].(bool); ok {
			point.IsDressingRoom = v
		}
		// nil = поля нет в ответе CDEK; единицы — килограммы (см. спеку, OfficeDto)
		if v, ok := p["weight_min"].(float64); ok {
			weightMin := v
			point.WeightMin = &weightMin
		}
		if v, ok := p["weight_max"].(float64); ok {
			weightMax := v
			point.WeightMax = &weightMax
		}
		if v, ok := p["owner_code"].(string); ok {
			point.OwnerCode = v
		}

		// Телефоны
		if phones, ok := p["phones"].([]interface{}); ok {
			point.Phones = make([]Phone, 0, len(phones))
			for _, ph := range phones {
				if phone, ok := ph.(map[string]interface{}); ok {
					phoneObj := Phone{}
					if number, ok := phone["number"].(string); ok {
						phoneObj.Number = number
					}
					if add, ok := phone["additional"].(string); ok {
						phoneObj.Additional = &add
					}
					point.Phones = append(point.Phones, phoneObj)
				}
			}
		}

		// Местоположение
		if loc, ok := p["location"].(map[string]interface{}); ok {
			if country, ok := loc["country"].(string); ok {
				point.Location.Country = country
			}
			if region, ok := loc["region"].(string); ok {
				point.Location.Region = region
			}
			if city, ok := loc["city"].(string); ok {
				point.Location.City = city
			}
			if address, ok := loc["address"].(string); ok {
				point.Location.Address = address
			}
			if postal, ok := loc["postal_code"].(string); ok {
				point.Location.PostalCode = postal
			}
			// Числовые коды приходят как float64 (encoding/json для interface{});
			// вне диапазона int32 остается 0 — конверсия с проверкой границ
			if v, ok := loc["city_code"].(float64); ok && v >= math.MinInt32 && v <= math.MaxInt32 {
				point.Location.CityCode = int32(v)
			}
			if v, ok := loc["region_code"].(float64); ok && v >= math.MinInt32 && v <= math.MaxInt32 {
				point.Location.RegionCode = int32(v)
			}
			if addressFull, ok := loc["address_full"].(string); ok {
				point.Location.AddressFull = addressFull
			}
			if lat, ok := loc["latitude"].(float64); ok {
				point.Location.Latitude = lat
			}
			if lon, ok := loc["longitude"].(float64); ok {
				point.Location.Longitude = lon
			}
		}

		// Изображения офиса
		if images, ok := p["office_image_list"].([]interface{}); ok && len(images) > 0 {
			if firstImg, ok := images[0].(map[string]interface{}); ok {
				if url, ok := firstImg["url"].(string); ok {
					point.OfficeImage = &url
				}
			}
		}

		points = append(points, point)
	}

	return points, nil
}

// ========================
// Order Info (GetOrder)
// ========================

// locationFromMap разбирает объект entity.from_location / entity.to_location из ответа
// GET /v2/orders в Location. Ключи объекта: code, fias_guid, postal_code, country_code,
// region, region_code, city, city_uuid, address, sub_region, latitude, longitude
// (region_code, city_uuid, sub_region, latitude, longitude в Location нет — игнорируются).
// Отсутствие объекта или поля ошибкой не является
func locationFromMap(loc map[string]interface{}) Location {
	var out Location
	// Числовые коды приходят как float64 (encoding/json для interface{});
	// вне диапазона int32 поле не заполняется
	if v, ok := loc["code"].(float64); ok && v >= math.MinInt32 && v <= math.MaxInt32 {
		code := int32(v)
		out.Code = &code
	}
	if v, ok := loc["fias_guid"].(string); ok && v != "" {
		out.FiasGUID = &v
	}
	if v, ok := loc["postal_code"].(string); ok && v != "" {
		out.PostalCode = &v
	}
	if v, ok := loc["country_code"].(string); ok && v != "" {
		out.CountryCode = &v
	}
	if v, ok := loc["region"].(string); ok && v != "" {
		out.Region = &v
	}
	if v, ok := loc["city"].(string); ok && v != "" {
		out.City = &v
	}
	if v, ok := loc["address"].(string); ok && v != "" {
		out.Address = &v
	}
	return out
}

// int32FromCDEK конвертирует JSON-число (через interface{} всегда float64) в int32;
// вне диапазона int32 остается 0
func int32FromCDEK(v float64) int32 {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0
	}
	return int32(v)
}

// orderPackagesFromCDEK разбирает entity.packages ответа GET /v2/orders в []OrderPackage.
// Отсутствующие ключи оставляют нулевые значения (для type=2/delivery items может не быть)
func orderPackagesFromCDEK(pkgs []interface{}) []OrderPackage {
	out := make([]OrderPackage, 0, len(pkgs))
	for _, p := range pkgs {
		pkgMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		pkg := OrderPackage{}
		if v, ok := pkgMap["number"].(string); ok {
			pkg.Number = v
		}
		if v, ok := pkgMap["weight"].(float64); ok {
			pkg.Weight = int32FromCDEK(v)
		}
		if v, ok := pkgMap["length"].(float64); ok {
			length := int32FromCDEK(v)
			pkg.Length = &length
		}
		if v, ok := pkgMap["width"].(float64); ok {
			width := int32FromCDEK(v)
			pkg.Width = &width
		}
		if v, ok := pkgMap["height"].(float64); ok {
			height := int32FromCDEK(v)
			pkg.Height = &height
		}
		if v, ok := pkgMap["comment"].(string); ok {
			pkg.Comment = &v
		}
		if items, ok := pkgMap["items"].([]interface{}); ok {
			pkg.Items = make([]Item, 0, len(items))
			for _, it := range items {
				itemMap, ok := it.(map[string]interface{})
				if !ok {
					continue
				}
				item := Item{}
				if v, ok := itemMap["name"].(string); ok {
					item.Name = v
				}
				if v, ok := itemMap["ware_key"].(string); ok {
					item.WareKey = v
				}
				// payment — объект {value, ...}
				if payment, ok := itemMap["payment"].(map[string]interface{}); ok {
					if v, ok := payment["value"].(float64); ok {
						item.Payment = v
					}
				}
				if v, ok := itemMap["cost"].(float64); ok {
					item.Cost = v
				}
				if v, ok := itemMap["weight"].(float64); ok {
					item.Weight = int32FromCDEK(v)
				}
				if v, ok := itemMap["amount"].(float64); ok {
					item.Amount = int32FromCDEK(v)
				}
				pkg.Items = append(pkg.Items, item)
			}
		}
		out = append(out, pkg)
	}
	return out
}

// orderRequestsFromCDEK разбирает корневой массив requests[] ответа GET /v2/orders
// (ResponseDtoOrderResponseDto → RequestDto1: type, state, date_time, errors).
// Состояние асинхронного создания/обновления (ACCEPTED/WAITING/SUCCESSFUL/INVALID)
// лежит именно здесь, в entity.requests его нет (спека и живые фикстуры)
func orderRequestsFromCDEK(requests []interface{}) []OrderRequestState {
	out := make([]OrderRequestState, 0, len(requests))
	for _, r := range requests {
		reqMap, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		state := OrderRequestState{}
		if v, ok := reqMap["type"].(string); ok {
			state.Type = v
		}
		if v, ok := reqMap["state"].(string); ok {
			state.State = v
		}
		if v, ok := reqMap["date_time"].(string); ok {
			state.DateTime = v
		}
		if errs, ok := reqMap["errors"].([]interface{}); ok {
			state.Errors = make([]OrderError, 0, len(errs))
			for _, e := range errs {
				errMap, ok := e.(map[string]interface{})
				if !ok {
					continue
				}
				orderErr := OrderError{}
				if v, ok := errMap["code"].(string); ok {
					orderErr.Code = v
				}
				if v, ok := errMap["message"].(string); ok {
					orderErr.Message = v
				}
				state.Errors = append(state.Errors, orderErr)
			}
		}
		out = append(out, state)
	}
	return out
}

// fromCDEKOrderToInfo преобразует GetOrder ответ → OrderInfo (полная информация)
func (m *dtoMapper) fromCDEKOrderToInfo(data []byte) (*OrderInfo, error) {
	var rawResp map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal order info: %w", err)
	}

	// Проверяем наличие entity
	entity, ok := rawResp["entity"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("entity not found in response")
	}

	info := &OrderInfo{}

	// UUID (обязательный)
	if uuid, ok := entity["uuid"].(string); ok {
		info.UUID = uuid
	}

	// Номер заказа
	if cdekNum, ok := entity["cdek_number"].(string); ok {
		info.Number = &cdekNum
	}

	// Тип заказа: CDEK отдает число (1=интернет-магазин, 2=доставка);
	// обратный маппинг orderTypeString, неизвестное число — строкой цифр.
	// Строковое значение (если появится) принимается как есть
	if orderType, ok := entity["type"].(float64); ok {
		info.Type = orderTypeString(int(orderType))
	} else if orderType, ok := entity["type"].(string); ok {
		info.Type = orderType
	}

	// Код тарифа
	if tariff, ok := entity["tariff_code"].(float64); ok {
		info.TariffCode = int(tariff)
	}

	// Коды ПВЗ отправителя/получателя — верхнеуровневые строки ответа (OrderResponseDto),
	// заданы только для заказов «от склада»/«до склада»
	if code, ok := entity["shipment_point"].(string); ok {
		info.ShipmentPoint = code
	}
	if code, ok := entity["delivery_point"].(string); ok {
		info.DeliveryPoint = code
	}

	// Локации отправителя/получателя (entity.from_location / entity.to_location)
	if loc, ok := entity["from_location"].(map[string]interface{}); ok {
		info.FromLocation = locationFromMap(loc)
	}
	if loc, ok := entity["to_location"].(map[string]interface{}); ok {
		info.ToLocation = locationFromMap(loc)
	}

	// Упаковки (entity.packages)
	if pkgs, ok := entity["packages"].([]interface{}); ok {
		info.Packages = orderPackagesFromCDEK(pkgs)
	}

	// Отправитель (теперь Recipient с поддержкой ИНН и паспорта)
	if sender, ok := entity["sender"].(map[string]interface{}); ok {
		if name, ok := sender["name"].(string); ok {
			info.Sender.Name = name
		}
		if company, ok := sender["company"].(string); ok {
			info.Sender.Company = &company
		}
		if email, ok := sender["email"].(string); ok {
			info.Sender.Email = &email
		}
		if phones, ok := sender["phones"].([]interface{}); ok {
			info.Sender.Phones = make([]Phone, 0, len(phones))
			for _, ph := range phones {
				if phone, ok := ph.(map[string]interface{}); ok {
					phoneObj := Phone{}
					if number, ok := phone["number"].(string); ok {
						phoneObj.Number = number
					}
					if add, ok := phone["additional"].(string); ok {
						phoneObj.Additional = &add
					}
					info.Sender.Phones = append(info.Sender.Phones, phoneObj)
				}
			}
		}
		// ИНН (для юридических лиц)
		if tin, ok := sender["tin"].(string); ok {
			info.Sender.TIN = &tin
		}
		// Паспортные данные (для физических лиц)
		if passportSeries, ok := sender["passport_series"].(string); ok {
			info.Sender.PassportSeries = &passportSeries
		}
		if passportNumber, ok := sender["passport_number"].(string); ok {
			info.Sender.PassportNumber = &passportNumber
		}
		if passportDateOfIssue, ok := sender["passport_date_of_issue"].(string); ok {
			info.Sender.PassportDateOfIssue = &passportDateOfIssue
		}
		if passportOrg, ok := sender["passport_organization"].(string); ok {
			info.Sender.PassportOrganization = &passportOrg
		}
		if passportDOB, ok := sender["passport_date_of_birth"].(string); ok {
			info.Sender.PassportDateOfBirth = &passportDOB
		}
	}

	// Получатель
	if recipient, ok := entity["recipient"].(map[string]interface{}); ok {
		if name, ok := recipient["name"].(string); ok {
			info.Recipient.Name = name
		}
		if company, ok := recipient["company"].(string); ok {
			info.Recipient.Company = &company
		}
		if email, ok := recipient["email"].(string); ok {
			info.Recipient.Email = &email
		}
		if phones, ok := recipient["phones"].([]interface{}); ok {
			info.Recipient.Phones = make([]Phone, 0, len(phones))
			for _, ph := range phones {
				if phone, ok := ph.(map[string]interface{}); ok {
					phoneObj := Phone{}
					if number, ok := phone["number"].(string); ok {
						phoneObj.Number = number
					}
					if add, ok := phone["additional"].(string); ok {
						phoneObj.Additional = &add
					}
					info.Recipient.Phones = append(info.Recipient.Phones, phoneObj)
				}
			}
		}
		// ИНН (для юридических лиц)
		if tin, ok := recipient["tin"].(string); ok {
			info.Recipient.TIN = &tin
		}
		// Паспортные данные (для физических лиц)
		if passportSeries, ok := recipient["passport_series"].(string); ok {
			info.Recipient.PassportSeries = &passportSeries
		}
		if passportNumber, ok := recipient["passport_number"].(string); ok {
			info.Recipient.PassportNumber = &passportNumber
		}
		if passportDateOfIssue, ok := recipient["passport_date_of_issue"].(string); ok {
			info.Recipient.PassportDateOfIssue = &passportDateOfIssue
		}
		if passportOrg, ok := recipient["passport_organization"].(string); ok {
			info.Recipient.PassportOrganization = &passportOrg
		}
		if passportDOB, ok := recipient["passport_date_of_birth"].(string); ok {
			info.Recipient.PassportDateOfBirth = &passportDOB
		}
	}

	// Продавец (третье лицо, для интернет-магазинов)
	if seller, ok := entity["seller"].(map[string]interface{}); ok {
		info.Seller = &Seller{}
		if name, ok := seller["name"].(string); ok {
			info.Seller.Name = &name
		}
		if inn, ok := seller["inn"].(string); ok {
			info.Seller.INN = &inn
		}
		if phone, ok := seller["phone"].(string); ok {
			info.Seller.Phone = &phone
		}
		if ownership, ok := seller["ownership_form"].(float64); ok {
			ownershipInt := int(ownership)
			info.Seller.OwnershipForm = &ownershipInt
		}
		if address, ok := seller["address"].(string); ok {
			info.Seller.Address = &address
		}
	}

	// Статусы
	if statuses, ok := entity["statuses"].([]interface{}); ok && len(statuses) > 0 {
		info.Statuses = make([]StatusEvent, 0, len(statuses))
		for _, s := range statuses {
			if status, ok := s.(map[string]interface{}); ok {
				event := StatusEvent{}
				if code, ok := status["code"].(string); ok {
					event.Code = code
				}
				if name, ok := status["name"].(string); ok {
					event.Name = name
				}
				if dt, ok := status["date_time"].(string); ok {
					event.DateTime = dt
				}
				if city, ok := status["city"].(string); ok {
					event.City = &city
				}
				info.Statuses = append(info.Statuses, event)
			}
		}
	}

	// Асинхронные запросы над заказом: корневой requests[] ответа (не entity),
	// date_time первого — дата создания заказа
	if requests, ok := rawResp["requests"].([]interface{}); ok {
		info.Requests = orderRequestsFromCDEK(requests)
		if len(info.Requests) > 0 {
			info.CreatedAt = info.Requests[0].DateTime
		}
	}

	// Плановая дата доставки
	if delivery, ok := entity["delivery_detail"].(map[string]interface{}); ok {
		if date, ok := delivery["date"].(string); ok {
			info.EstimatedDelivery = &date
		}
	}

	return info, nil
}

// ========================
// Update Order
// ========================

// orderPackageToMap преобразует OrderPackage → map для API (create и update).
// comment отправляется всегда, когда задан: некоторые договоры CDEK требуют packages[].comment
// (v2_field_is_empty). items не отправляются, если пусты (для type=2 они необязательны)
func orderPackageToMap(pkg OrderPackage) map[string]interface{} {
	p := map[string]interface{}{
		"number": pkg.Number,
		"weight": pkg.Weight,
	}
	if pkg.Length != nil {
		p["length"] = *pkg.Length
	}
	if pkg.Width != nil {
		p["width"] = *pkg.Width
	}
	if pkg.Height != nil {
		p["height"] = *pkg.Height
	}
	if pkg.Comment != nil && *pkg.Comment != "" {
		p["comment"] = *pkg.Comment
	}
	if len(pkg.Items) > 0 {
		items := make([]map[string]interface{}, len(pkg.Items))
		for j, item := range pkg.Items {
			items[j] = map[string]interface{}{
				"name":     item.Name,
				"ware_key": item.WareKey,
				"payment":  map[string]interface{}{"value": item.Payment},
				"cost":     item.Cost,
				"weight":   item.Weight,
				"amount":   item.Amount,
			}
		}
		p["items"] = items
	}
	return p
}

// toCDEKUpdateOrderRequest преобразует UpdateOrderRequest → map для Update API
func (m *dtoMapper) toCDEKUpdateOrderRequest(req *UpdateOrderRequest) (map[string]interface{}, error) {
	update := make(map[string]interface{})

	// UUID обязательный для обновления
	update["uuid"] = req.OrderUUID

	// Тип заказа (по спеке PATCH требует type и recipient; отправляем только если задан,
	// чтобы не менять поведение существующих вызовов без Type)
	if req.Type != "" {
		update["type"] = orderTypeCode(req.Type)
	}

	// Получатель (если указан)
	if req.Recipient != nil {
		recipient := map[string]interface{}{
			"name": req.Recipient.Name,
		}
		if req.Recipient.Company != nil {
			recipient["company"] = *req.Recipient.Company
		}
		if req.Recipient.Email != nil {
			recipient["email"] = *req.Recipient.Email
		}
		if len(req.Recipient.Phones) > 0 {
			phones := make([]map[string]interface{}, len(req.Recipient.Phones))
			for i, phone := range req.Recipient.Phones {
				p := map[string]interface{}{"number": phone.Number}
				if phone.Additional != nil {
					p["additional"] = *phone.Additional
				}
				phones[i] = p
			}
			recipient["phones"] = phones
		}
		// ИНН для получателя
		if req.Recipient.TIN != nil {
			recipient["tin"] = *req.Recipient.TIN
		}
		// Паспортные данные для получателя
		if req.Recipient.PassportSeries != nil {
			recipient["passport_series"] = *req.Recipient.PassportSeries
		}
		if req.Recipient.PassportNumber != nil {
			recipient["passport_number"] = *req.Recipient.PassportNumber
		}
		if req.Recipient.PassportDateOfIssue != nil {
			recipient["passport_date_of_issue"] = *req.Recipient.PassportDateOfIssue
		}
		if req.Recipient.PassportOrganization != nil {
			recipient["passport_organization"] = *req.Recipient.PassportOrganization
		}
		if req.Recipient.PassportDateOfBirth != nil {
			recipient["passport_date_of_birth"] = *req.Recipient.PassportDateOfBirth
		}
		update["recipient"] = recipient
	}

	// Отправитель (если указан) - теперь с ИНН и паспортными данными
	if req.Sender != nil {
		sender := map[string]interface{}{
			"name": req.Sender.Name,
		}
		if req.Sender.Company != nil {
			sender["company"] = *req.Sender.Company
		}
		if req.Sender.Email != nil {
			sender["email"] = *req.Sender.Email
		}
		if len(req.Sender.Phones) > 0 {
			phones := make([]map[string]interface{}, len(req.Sender.Phones))
			for i, phone := range req.Sender.Phones {
				p := map[string]interface{}{"number": phone.Number}
				if phone.Additional != nil {
					p["additional"] = *phone.Additional
				}
				phones[i] = p
			}
			sender["phones"] = phones
		}
		// ИНН для отправителя
		if req.Sender.TIN != nil {
			sender["tin"] = *req.Sender.TIN
		}
		// Паспортные данные для отправителя
		if req.Sender.PassportSeries != nil {
			sender["passport_series"] = *req.Sender.PassportSeries
		}
		if req.Sender.PassportNumber != nil {
			sender["passport_number"] = *req.Sender.PassportNumber
		}
		if req.Sender.PassportDateOfIssue != nil {
			sender["passport_date_of_issue"] = *req.Sender.PassportDateOfIssue
		}
		if req.Sender.PassportOrganization != nil {
			sender["passport_organization"] = *req.Sender.PassportOrganization
		}
		if req.Sender.PassportDateOfBirth != nil {
			sender["passport_date_of_birth"] = *req.Sender.PassportDateOfBirth
		}
		update["sender"] = sender
	}

	// Истинный продавец (третье лицо) - если указан
	if req.Seller != nil {
		seller := make(map[string]interface{})
		if req.Seller.Name != nil {
			seller["name"] = *req.Seller.Name
		}
		if req.Seller.INN != nil {
			seller["inn"] = *req.Seller.INN
		}
		if req.Seller.Phone != nil {
			seller["phone"] = *req.Seller.Phone
		}
		if req.Seller.OwnershipForm != nil {
			seller["ownership_form"] = *req.Seller.OwnershipForm
		}
		if req.Seller.Address != nil {
			seller["address"] = *req.Seller.Address
		}
		update["seller"] = seller
	}

	// Сторона получателя: код ПВЗ и адрес взаимоисключающие (XOR),
	// конфликт — ошибка, адрес не отбрасывается молча
	if code := pointCode(req.DeliveryPoint); code != "" {
		if locationFilled(req.ToLocation) {
			return nil, pointAddressConflictErr("delivery_point", "to_location", "recipient")
		}
		update["delivery_point"] = code
	} else if toLoc := m.locationToMap(req.ToLocation); toLoc != nil {
		update["to_location"] = toLoc
	}

	// Сторона отправителя: код ПВЗ и адрес взаимоисключающие (XOR),
	// конфликт — ошибка, адрес не отбрасывается молча
	if code := pointCode(req.ShipmentPoint); code != "" {
		if locationFilled(req.FromLocation) {
			return nil, pointAddressConflictErr("shipment_point", "from_location", "sender")
		}
		update["shipment_point"] = code
	} else if fromLoc := m.locationToMap(req.FromLocation); fromLoc != nil {
		update["from_location"] = fromLoc
	}

	// Комментарий (если указан)
	if req.Comment != nil {
		update["comment"] = *req.Comment
	}

	// Упаковки (если указаны)
	if len(req.Packages) > 0 {
		packages := make([]map[string]interface{}, len(req.Packages))
		for i, pkg := range req.Packages {
			packages[i] = orderPackageToMap(pkg)
		}
		update["packages"] = packages
	}

	return update, nil
}

// ========================
// Location Reference (Cities/Regions)
// ========================

// fromCDEKCities преобразует Cities ответ → []City
func (m *dtoMapper) fromCDEKCities(data []byte) ([]City, error) {
	var rawResp []map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal cities: %w", err)
	}

	cities := make([]City, 0, len(rawResp))
	for _, c := range rawResp {
		city := City{}
		if code, ok := c["code"].(float64); ok {
			city.Code = int(code)
		}
		if cityName, ok := c["city"].(string); ok {
			city.City = cityName
		}
		if region, ok := c["region"].(string); ok {
			city.Region = region
		}
		if country, ok := c["country"].(string); ok {
			city.Country = country
		}
		if countryCode, ok := c["country_code"].(string); ok {
			city.CountryCode = countryCode
		}
		cities = append(cities, city)
	}
	return cities, nil
}

// fromCDEKRegions преобразует Regions ответ → []Region
func (m *dtoMapper) fromCDEKRegions(data []byte) ([]Region, error) {
	var rawResp []map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal regions: %w", err)
	}

	regions := make([]Region, 0, len(rawResp))
	for _, r := range rawResp {
		region := Region{}
		if code, ok := r["region_code"].(float64); ok {
			region.Code = int(code)
		}
		if regionName, ok := r["region"].(string); ok {
			region.Region = regionName
		}
		if country, ok := r["country"].(string); ok {
			region.Country = country
		}
		regions = append(regions, region)
	}
	return regions, nil
}

// ========================
// Intakes
// ========================

// toCDEKIntakeRequest преобразует IntakeRequest → map для Intake API
func (m *dtoMapper) toCDEKIntakeRequest(req *IntakeRequest) (map[string]interface{}, error) {
	intake := map[string]interface{}{
		"intake_date":      req.IntakeDate,
		"intake_time_from": req.IntakeTimeFrom,
		"intake_time_to":   req.IntakeTimeTo,
		"sender":           map[string]interface{}{"name": req.Sender.Name},
	}
	if req.Comment != nil {
		intake["comment"] = *req.Comment
	}
	return intake, nil
}

// fromCDEKIntakeResponse преобразует Intake ответ → IntakeResponse
func (m *dtoMapper) fromCDEKIntakeResponse(data []byte) (*IntakeResponse, error) {
	var rawResp map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal intake response: %w", err)
	}
	intakeResp := &IntakeResponse{}
	if entity, ok := rawResp["entity"].(map[string]interface{}); ok {
		if uuid, ok := entity["uuid"].(string); ok {
			intakeResp.UUID = uuid
		}
	}
	return intakeResp, nil
}

// fromCDEKIntakeInfo преобразует GetIntake ответ → IntakeInfo
func (m *dtoMapper) fromCDEKIntakeInfo(data []byte) (*IntakeInfo, error) {
	var rawResp map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal intake info: %w", err)
	}
	info := &IntakeInfo{}
	if entity, ok := rawResp["entity"].(map[string]interface{}); ok {
		if uuid, ok := entity["uuid"].(string); ok {
			info.UUID = uuid
		}
	}
	return info, nil
}

// ========================
// Webhooks
// ========================

// fromCDEKWebhookResponse преобразует CreateWebhook ответ → WebhookResponse
func (m *dtoMapper) fromCDEKWebhookResponse(data []byte) (*WebhookResponse, error) {
	var rawResp map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal webhook response: %w", err)
	}
	webhookResp := &WebhookResponse{}
	if entity, ok := rawResp["entity"].(map[string]interface{}); ok {
		if uuid, ok := entity["uuid"].(string); ok {
			webhookResp.UUID = uuid
		}
	}
	return webhookResp, nil
}

// fromCDEKWebhook преобразует GetWebhook ответ → Webhook
func (m *dtoMapper) fromCDEKWebhook(data []byte) (*Webhook, error) {
	var rawResp map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal webhook: %w", err)
	}

	webhook := &Webhook{}
	if entity, ok := rawResp["entity"].(map[string]interface{}); ok {
		if uuid, ok := entity["uuid"].(string); ok {
			webhook.UUID = uuid
		}
		if url, ok := entity["url"].(string); ok {
			webhook.URL = url
		}
		if webhookType, ok := entity["type"].(string); ok {
			webhook.Type = webhookType
		}
	}
	return webhook, nil
}

// fromCDEKWebhooks преобразует ListWebhooks ответ → []Webhook
func (m *dtoMapper) fromCDEKWebhooks(data []byte) ([]Webhook, error) {
	var rawResp []map[string]interface{}
	if err := json.Unmarshal(data, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal webhooks: %w", err)
	}
	webhooks := make([]Webhook, 0, len(rawResp))
	for _, w := range rawResp {
		webhook := Webhook{}
		if uuid, ok := w["uuid"].(string); ok {
			webhook.UUID = uuid
		}
		if url, ok := w["url"].(string); ok {
			webhook.URL = url
		}
		if webhookType, ok := w["type"].(string); ok {
			webhook.Type = webhookType
		}
		webhooks = append(webhooks, webhook)
	}
	return webhooks, nil
}
