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

	// Обязательные поля — CDEK v2 expects integer type: 1=online_store, 2=delivery
	switch req.Type {
	case "delivery", "2":
		order["type"] = 2
	case "online_store", "1":
		order["type"] = 1
	default:
		order["type"] = 1
	}
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

// ========================
// Order Info (GetOrder)
// ========================

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

	// Тип заказа
	if orderType, ok := entity["type"].(string); ok {
		info.Type = orderType
	}

	// Код тарифа
	if tariff, ok := entity["tariff_code"].(float64); ok {
		info.TariffCode = int(tariff)
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

	// Дата создания из requests
	if requests, ok := entity["requests"].([]interface{}); ok && len(requests) > 0 {
		if firstReq, ok := requests[0].(map[string]interface{}); ok {
			if dt, ok := firstReq["date_time"].(string); ok {
				info.CreatedAt = dt
			}
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

// toCDEKUpdateOrderRequest преобразует UpdateOrderRequest → map для Update API
func (m *dtoMapper) toCDEKUpdateOrderRequest(req *UpdateOrderRequest) (map[string]interface{}, error) {
	update := make(map[string]interface{})

	// UUID обязательный для обновления
	update["uuid"] = req.OrderUUID

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

	// Адрес получателя (если указан)
	if req.ToLocation != nil {
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
		update["to_location"] = toLoc
	}

	// Адрес отправителя (если указан)
	if req.FromLocation != nil {
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
