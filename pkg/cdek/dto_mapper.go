package cdek

import (
	"encoding/json"
	"fmt"
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
		if min, ok := tariffMap["period_min"].(float64); ok {
			tariff.PeriodMin = int(min)
		}
		if max, ok := tariffMap["period_max"].(float64); ok {
			tariff.PeriodMax = int(max)
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

// toCDEKOrderRequest преобразует OrderRequest → map для OrderCreateRequestDto
// Используем map[string]interface{} т.к. автогенерированные типы используют interface{}
func (m *dtoMapper) toCDEKOrderRequest(req *OrderRequest) (map[string]interface{}, error) {
	order := make(map[string]interface{})

	// Обязательные поля
	order["type"] = req.Type
	order["tariff_code"] = req.TariffCode

	// Опциональный комментарий
	if req.Comment != nil {
		order["comment"] = *req.Comment
	}

	// Отправитель
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
	// Паспортные данные
	if req.Recipient.PassportSeries != nil {
		recipient["passport_series"] = *req.Recipient.PassportSeries
	}
	if req.Recipient.PassportNumber != nil {
		recipient["passport_number"] = *req.Recipient.PassportNumber
	}
	order["recipient"] = recipient

	// Адрес отправителя
	if req.FromLocation.Code != nil || req.FromLocation.Address != nil {
		fromLoc := make(map[string]interface{})
		if req.FromLocation.Code != nil {
			fromLoc["code"] = *req.FromLocation.Code
		}
		if req.FromLocation.Address != nil {
			fromLoc["address"] = *req.FromLocation.Address
		}
		if req.FromLocation.City != nil {
			fromLoc["city"] = *req.FromLocation.City
		}
		order["from_location"] = fromLoc
	}

	// Адрес получателя
	if req.ToLocation.Code != nil || req.ToLocation.Address != nil {
		toLoc := make(map[string]interface{})
		if req.ToLocation.Code != nil {
			toLoc["code"] = *req.ToLocation.Code
		}
		if req.ToLocation.Address != nil {
			toLoc["address"] = *req.ToLocation.Address
		}
		if req.ToLocation.City != nil {
			toLoc["city"] = *req.ToLocation.City
		}
		order["to_location"] = toLoc
	}

	// Упаковки (обязательно)
	packages := make([]map[string]interface{}, len(req.Packages))
	for i, pkg := range req.Packages {
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

		// Товары в упаковке
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
		packages[i] = p
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

	// Проверяем наличие entity
	entity, ok := rawResp["entity"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("entity not found in response")
	}

	orderResp := &OrderResponse{}

	// UUID (обязательный)
	if uuid, ok := entity["uuid"].(string); ok {
		orderResp.UUID = uuid
	} else {
		return nil, fmt.Errorf("uuid not found in entity")
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

	// Дата создания из requests
	if requests, ok := entity["requests"].([]interface{}); ok && len(requests) > 0 {
		if firstReq, ok := requests[0].(map[string]interface{}); ok {
			if dt, ok := firstReq["date_time"].(string); ok {
				orderResp.CreatedAt = dt
			}
		}
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
