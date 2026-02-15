# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-02-15

### Added
- **High-level Service API** for simplified CDEK integration (4 key methods)
  - `CalculateCost()` - delivery cost calculation with automatic tariff selection
  - `CreateOrder()` - order creation with validation and type conversion
  - `TrackOrder()` - order tracking with status history
  - `ListDeliveryPoints()` - PVZ (pickup points) listing
- **Service-level DTO types** for simplified API usage
  - Cost: CostRequest, CostResponse, Package, TariffOption
  - Orders: OrderRequest, OrderResponse, Contact, Recipient, Location, OrderPackage, Item
  - Tracking: TrackingInfo, StatusEvent
  - Delivery Points: DeliveryPointsRequest, DeliveryPoint, PointLocation, Phone
- **DTO mapper** with map[string]interface{} support for generated types
  - toCDEKCalculatorRequest / fromCDEKCalculatorResponse
  - toCDEKOrderRequest / fromCDEKOrderResponse
  - fromCDEKOrderToTracking
  - fromCDEKDeliveryPoints
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
- Integration test for high-level CalculateCost API
- Unit tests for orderValidator (4 scenarios) and costCalculator (4 scenarios)

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
