# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1] - 2026-02-15

### Added
- **Webhooks - Full CRUD** (completing webhook functionality)
  - `GetWebhook()` - retrieve webhook by UUID
  - `DeleteWebhook()` - delete webhook by UUID
  - Webhook cleanup helper for integration tests
- **6 webhook unit tests** - constants, DTO mapper, validation
- **6 webhook integration tests** - full CRUD lifecycle, different types, edge cases
- **Comprehensive documentation**
  - `ROADMAP.md` - feature roadmap with completion status
  - `docs/API_ENDPOINTS.md` - complete API reference (794 lines)
  - `docs/DEPLOYMENT.md` - deployment guide (815 lines)
  - `docs/INTEGRATION_GUIDE.md` - ERP/CRM integration guide (1036 lines)

### Fixed
- golangci-lint issues (removed redundant type declarations)
- Webhook integration tests now properly clean up test data

### Changed
- ROADMAP.md updated - Webhooks marked as complete
- Test coverage: 18.7% (excluding generated client.go)
- Total integration tests: 22 (was 16)

### Removed
- Duplicate documentation files (FINAL_REPORT.md, PROJECT_STATUS.md, REORGANIZATION_COMPLETE.md)

## [0.2.0] - 2026-02-15

### Added
- **High-level Service API** for simplified CDEK integration (18 methods)

  **Week 1 (MVP) - Core delivery operations:**
  - `CalculateCost()` - delivery cost calculation with automatic tariff selection
  - `CreateOrder()` - order creation with validation and type conversion
  - `TrackOrder()` - order tracking with status history
  - `ListDeliveryPoints()` - PVZ (pickup points) listing
  - `PrintBarcode()` - barcode label printing job creation
  - `PrintWaybill()` - waybill printing job creation

  **Week 2 (Extended) - Full order lifecycle:**
  - `GetOrder()` - retrieve complete order information
  - `UpdateOrder()` - update existing order details
  - `CancelOrder()` - cancel order (if status allows)
  - `DownloadBarcode()` - download barcode labels as PDF
  - `DownloadWaybill()` - download waybill as PDF

  **Week 3 (Full API) - Location & notification support:**
  - `ListCities()` - search cities for delivery
  - `ListRegions()` - list regions/oblasts
  - `CreateIntake()` - create pickup request
  - `GetIntake()` - retrieve intake information
  - `DeleteIntake()` - cancel pickup request
  - `CreateWebhook()` - register webhook for status notifications
  - `ListWebhooks()` - list registered webhooks
- **Service-level DTO types** for simplified API usage
  - Cost: CostRequest, CostResponse, Package, TariffOption
  - Orders: OrderRequest, OrderResponse, OrderInfo, UpdateOrderRequest, Contact, Recipient, Location, OrderPackage, Item
  - Tracking: TrackingInfo, StatusEvent
  - Delivery Points: DeliveryPointsRequest, DeliveryPoint, PointLocation, Phone
  - Printing: PrintRequest, PrintResponse
  - Locations: CitiesRequest, City, RegionsRequest, Region
  - Intakes: IntakeRequest, IntakeResponse, IntakeInfo, IntakeOrder
  - Webhooks: WebhookRequest, WebhookResponse, Webhook
- **DTO mapper** with map[string]interface{} support for generated types
  - Calculator: toCDEKCalculatorRequest, fromCDEKCalculatorResponse
  - Orders: toCDEKOrderRequest, fromCDEKOrderResponse, fromCDEKOrderToInfo, fromCDEKOrderToTracking, toCDEKUpdateOrderRequest
  - Delivery Points: fromCDEKDeliveryPoints
  - Locations: fromCDEKCities, fromCDEKRegions
  - Intakes: toCDEKIntakeRequest, fromCDEKIntakeResponse, fromCDEKIntakeInfo
  - Webhooks: fromCDEKWebhookResponse, fromCDEKWebhooks
- **Company recipient support** with TIN (ИНН) field
  - Recipient type extended to support legal entities
  - TIN field for companies and individual entrepreneurs (10 or 12 characters)
  - Passport fields for physical persons
  - Reference implementation based on vseinstrumentiru/CDEK library
- **Circuit Breaker** protection (sony/gobreaker v2)
  - Automatic protection from cascading failures
  - Configurable failure thresholds (60% failure ratio, 3 min requests)
  - Half-open state recovery with request limiting
  - 60 second timeout for circuit recovery
- **Structured Logging** (zerolog) with optional configuration
  - Request/response logging
  - Error logging with context
  - No-op logger by default (zero overhead)
- **ServiceConfig** for flexible service configuration
  - Circuit breaker settings customization
  - Optional logger injection
  - Default configuration with sensible defaults
- **Enhanced validation** components
  - costCalculator: city codes (positive), packages (non-empty, positive weight)
  - orderValidator: type, tariff code, recipient (name, phones), packages (number, weight, items)
  - Detailed error messages with field paths
- **Comprehensive integration tests** (16 tests, all passing)
  - Week 1: CalculateCost, CreateOrder, TrackOrder, ListDeliveryPoints, PrintBarcode, PrintWaybill
  - Week 2: GetOrder, UpdateOrder, CancelOrder, DownloadBarcode, DownloadWaybill
  - Week 3: ListCities, ListRegions, CreateIntake, ListWebhooks, CreateWebhook
  - Full order CRUD lifecycle tested against CDEK Sandbox
- **Unit tests** for validation components
  - orderValidator: 4 scenarios (nil, valid, missing fields, empty packages)
  - costCalculator: 4 scenarios (nil, valid, empty packages, zero weight)
- **Coverage: 70.2% domain code** (excluding generated client.go)
  - config.go: 100%, manager.go: 97.0%, components.go: 93.5%
  - errors.go: 92.2%, auth.go: 89.7%
  - service.go: 70.4%, dto_mapper.go: 65.1%

### Changed
- **BREAKING**: `NewService()` signature changed
  - Before: `NewService(client *AuthenticatedClient)`
  - After: `NewService(client *AuthenticatedClient, config *ServiceConfig)`
  - Pass `nil` for default configuration
- Service structure enhanced with Circuit Breaker and logger fields
- Updated README with CalculateCost usage example
- Updated architecture diagram with Circuit Breaker and components

### Dependencies
- Added `github.com/sony/gobreaker/v2 v2.4.0`
- Added `github.com/rs/zerolog v1.34.0`

### Documentation
- Production-Ready Features section in README
- Circuit Breaker configuration documentation
- High-level API usage examples

## [0.1.0] - 2026-02-15

### Added
- Initial release of CDEK Go Client
- OAuth2 authentication with automatic token refresh and caching
- Multi-account support for managing multiple CDEK accounts
- Thread-safe token caching implementation
- High-level Service API wrapper for simplified usage
- Auto-generated client from OpenAPI 3.0 specification (40+ endpoints)
- Integration tests for key endpoints:
  - OAuth2 token retrieval
  - Health check
  - Delivery points listing (~7133 PVZ)
  - Cities search
  - Tariff calculation (~33 tariffs)
- Comprehensive test coverage (89.2% excluding generated code)
- Custom OpenAPI template for Go-standard "Code generated" header
- golangci-lint configuration with essential linters
- Multi-account manager with parallel health checks

### Fixed
- OAuth2 endpoint: changed from `/oauth2/token` (410 Gone) to `/v2/oauth/token`
- OAuth parameters: changed from query string to `application/x-www-form-urlencoded` body
- OpenAPI operationIds: fixed duplicates and non-descriptive names
- URLSandbox: updated to production URL (api.edu.cdek.ru is deprecated)
- Generated code header: moved "Code generated" comment to first line per Go standard
- Removed "version v2.5.1" from generated comment (violated Go regex standard)

### Documentation
- Complete README.md with usage examples
- Architecture documentation
- Testing guidelines (unit + integration)

[0.2.0]: https://github.com/metrica-pro/cdek-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/metrica-pro/cdek-go/releases/tag/v0.1.0
