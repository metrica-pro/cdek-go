# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.1.0]: https://github.com/metrica-pro/cdek-go/releases/tag/v0.1.0
