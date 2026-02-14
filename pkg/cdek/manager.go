package cdek

import (
	"context"
	"fmt"
	"sync"
)

// Manager мультиаккаунт менеджер для работы с несколькими аккаунтами СДЭК
type Manager struct {
	config  *Config
	clients map[string]*AuthenticatedClient
	mu      sync.RWMutex
}

// NewManager создает новый менеджер
func NewManager(config *Config) (*Manager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	m := &Manager{
		config:  config,
		clients: make(map[string]*AuthenticatedClient),
	}

	// Инициализируем клиентов для всех аккаунтов
	for _, accConfig := range config.Accounts {
		client, err := NewAuthenticatedClient(&accConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for account %q: %w", accConfig.Name, err)
		}
		m.clients[accConfig.Name] = client
	}

	return m, nil
}

// GetClient возвращает клиент для указанного аккаунта
func (m *Manager) GetClient(accountName string) (*AuthenticatedClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[accountName]
	if !ok {
		return nil, fmt.Errorf("account %q not found", accountName)
	}

	return client, nil
}

// GetDefaultClient возвращает клиент по умолчанию
func (m *Manager) GetDefaultClient() (*AuthenticatedClient, error) {
	if m.config.DefaultAccount == "" {
		if len(m.config.Accounts) == 1 {
			return m.GetClient(m.config.Accounts[0].Name)
		}
		return nil, fmt.Errorf("no default account configured")
	}

	return m.GetClient(m.config.DefaultAccount)
}

// ListAccounts возвращает список всех аккаунтов
func (m *Manager) ListAccounts() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	accounts := make([]string, 0, len(m.clients))
	for name := range m.clients {
		accounts = append(accounts, name)
	}

	return accounts
}

// AddAccount добавляет новый аккаунт в runtime
func (m *Manager) AddAccount(config AccountConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[config.Name]; exists {
		return fmt.Errorf("account %q already exists", config.Name)
	}

	client, err := NewAuthenticatedClient(&config)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	m.clients[config.Name] = client
	m.config.Accounts = append(m.config.Accounts, config)

	return nil
}

// RemoveAccount удаляет аккаунт
func (m *Manager) RemoveAccount(accountName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[accountName]; !exists {
		return fmt.Errorf("account %q not found", accountName)
	}

	delete(m.clients, accountName)

	// Удаляем из конфига
	for i, acc := range m.config.Accounts {
		if acc.Name == accountName {
			m.config.Accounts = append(m.config.Accounts[:i], m.config.Accounts[i+1:]...)
			break
		}
	}

	return nil
}

// HealthCheck проверяет доступность всех аккаунтов
func (m *Manager) HealthCheck(ctx context.Context) map[string]error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]error)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, client := range m.clients {
		wg.Add(1)
		go func(n string, c *AuthenticatedClient) {
			defer wg.Done()

			_, err := c.GetToken(ctx)

			mu.Lock()
			results[n] = err
			mu.Unlock()
		}(name, client)
	}

	wg.Wait()

	return results
}
