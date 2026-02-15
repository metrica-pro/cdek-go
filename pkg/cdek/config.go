package cdek

import (
	"fmt"
	"time"
)

// AccountConfig конфигурация одного аккаунта СДЭК
type AccountConfig struct {
	// Name уникальное имя аккаунта (например, "account1", "warehouse-msk")
	Name string `json:"name" yaml:"name"`

	// ClientID идентификатор клиента для OAuth2
	ClientID string `json:"client_id" yaml:"client_id"`

	// ClientSecret секретный ключ клиента для OAuth2
	ClientSecret string `json:"client_secret" yaml:"client_secret"`

	// BaseURL базовый URL API (production или sandbox)
	BaseURL string `json:"base_url" yaml:"base_url"`

	// Timeout таймаут для HTTP запросов
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`

	// MaxRetries максимальное количество повторов при ошибках
	MaxRetries int `json:"max_retries,omitempty" yaml:"max_retries,omitempty"`
}

// Config общая конфигурация для мультиаккаунт менеджера
type Config struct {
	// Accounts список аккаунтов
	Accounts []AccountConfig `json:"accounts" yaml:"accounts"`

	// DefaultAccount имя аккаунта по умолчанию
	DefaultAccount string `json:"default_account,omitempty" yaml:"default_account,omitempty"`
}

// Validate проверяет валидность конфигурации
func (c *Config) Validate() error {
	if len(c.Accounts) == 0 {
		return fmt.Errorf("no accounts configured")
	}

	names := make(map[string]bool)
	for i, acc := range c.Accounts {
		if acc.Name == "" {
			return fmt.Errorf("account[%d]: name is required", i)
		}
		if names[acc.Name] {
			return fmt.Errorf("account[%d]: duplicate name %q", i, acc.Name)
		}
		names[acc.Name] = true

		if acc.ClientID == "" {
			return fmt.Errorf("account[%d] (%s): client_id is required", i, acc.Name)
		}
		if acc.ClientSecret == "" {
			return fmt.Errorf("account[%d] (%s): client_secret is required", i, acc.Name)
		}
		if acc.BaseURL == "" {
			return fmt.Errorf("account[%d] (%s): base_url is required", i, acc.Name)
		}
	}

	// Проверяем что default account существует
	if c.DefaultAccount != "" && !names[c.DefaultAccount] {
		return fmt.Errorf("default_account %q not found in accounts", c.DefaultAccount)
	}

	return nil
}

// GetAccount возвращает конфигурацию аккаунта по имени
func (c *Config) GetAccount(name string) (*AccountConfig, error) {
	for _, acc := range c.Accounts {
		if acc.Name == name {
			return &acc, nil
		}
	}
	return nil, fmt.Errorf("account %q not found", name)
}

const (
	// URLProduction production URL
	URLProduction = "https://api.cdek.ru"

	// URLSandbox sandbox/test URL
	// ВАЖНО: Используйте production URL (api.cdek.ru), т.к. api.edu.cdek.ru устарел
	URLSandbox = "https://api.cdek.ru"
)

// DefaultConfig создает конфигурацию по умолчанию
func DefaultConfig(clientID, clientSecret string) *Config {
	return &Config{
		Accounts: []AccountConfig{
			{
				Name:         "default",
				ClientID:     clientID,
				ClientSecret: clientSecret,
				BaseURL:      URLProduction,
				Timeout:      30 * time.Second,
				MaxRetries:   3,
			},
		},
		DefaultAccount: "default",
	}
}
