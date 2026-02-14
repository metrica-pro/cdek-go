# Package cdek

Go клиент для работы с CDEK API v2.

## Import

```go
import "github.com/metrica-pro/cdek-go/pkg/cdek"
```

## Быстрый старт

См. [корневой README](../../README.md)

## Основные типы

### Config

```go
type Config struct {
    Accounts       []AccountConfig
    DefaultAccount string
}

type AccountConfig struct {
    Name         string
    ClientID     string
    ClientSecret string
    BaseURL      string
    Timeout      time.Duration
    MaxRetries   int
}
```

### Manager

```go
type Manager struct {
    // ...
}

func NewManager(config *Config) (*Manager, error)
func (m *Manager) GetClient(accountName string) (*AuthenticatedClient, error)
func (m *Manager) GetDefaultClient() (*AuthenticatedClient, error)
func (m *Manager) HealthCheck(ctx context.Context) map[string]error
```

### AuthenticatedClient

```go
type AuthenticatedClient struct {
    // ...
}

func NewAuthenticatedClient(config *AccountConfig) (*AuthenticatedClient, error)
func (a *AuthenticatedClient) GetToken(ctx context.Context) (string, error)
func (a *AuthenticatedClient) Client() *Client
func (a *AuthenticatedClient) Do(ctx context.Context, method, path string, body interface{}) (*http.Response, error)
```

### Service

```go
type Service struct {
    // ...
}

func NewService(client *AuthenticatedClient) *Service
func (s *Service) GetClient() *AuthenticatedClient
func (s *Service) HealthCheck(ctx context.Context) error
```

## Константы

```go
const (
    URLProduction = "https://api.cdek.ru"
    URLSandbox    = "https://api.edu.cdek.ru"
)
```

## Ошибки

```go
var (
    ErrNotFound          = fmt.Errorf("cdek: not found")
    ErrUnauthorized      = fmt.Errorf("cdek: unauthorized")
    ErrInvalidRequest    = fmt.Errorf("cdek: invalid request")
    ErrRateLimitExceeded = fmt.Errorf("cdek: rate limit exceeded")
    ErrServerError       = fmt.Errorf("cdek: server error")
)
```

## Примеры

### Создание клиента

```go
config := cdek.DefaultConfig("client-id", "client-secret")
manager, err := cdek.NewManager(config)
client, err := manager.GetDefaultClient()
```

### Мультиаккаунт

```go
config := &cdek.Config{
    Accounts: []cdek.AccountConfig{
        {Name: "msk", ClientID: "...", ClientSecret: "...", BaseURL: cdek.URLProduction},
        {Name: "spb", ClientID: "...", ClientSecret: "...", BaseURL: cdek.URLProduction},
    },
    DefaultAccount: "msk",
}

manager, _ := cdek.NewManager(config)
mskClient, _ := manager.GetClient("msk")
spbClient, _ := manager.GetClient("spb")
```

### Health Check

```go
service := cdek.NewService(client)
if err := service.HealthCheck(ctx); err != nil {
    log.Fatal("API недоступен:", err)
}
```

## Документация

- **GoDoc:** https://pkg.go.dev/github.com/metrica-pro/cdek-go/pkg/cdek
- **Архитектура:** [../../ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Интеграция с ERP:** [../../INTEGRATION_WITH_ERP.md](../../INTEGRATION_WITH_ERP.md)

## Тестирование

```bash
go test -v ./...
```

**Coverage:** 93.6%
