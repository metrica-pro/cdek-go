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
// Orders, Tracking, Delivery Points
// ========================

// TODO: Implement remaining mappers after analyzing generated types
// Автогенерированные типы для Orders/Tracking/DeliveryPoints используют interface{}
// что усложняет типизацию. Будет реализовано в следующих итерациях.
//
// Планируемые методы:
// - toCDEKOrderRequest(req *OrderRequest) (map[string]interface{}, error)
// - fromCDEKOrderResponse(data []byte) (*OrderResponse, error)
// - fromCDEKOrderToTracking(data []byte) (*TrackingInfo, error)
// - fromCDEKDeliveryPoints(data []byte) ([]DeliveryPoint, error)
