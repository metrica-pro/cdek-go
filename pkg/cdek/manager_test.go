package cdek

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &Config{
			Accounts: []AccountConfig{
				{
					Name:         "test",
					ClientID:     "id1",
					ClientSecret: "secret1",
					BaseURL:      URLProduction,
					Timeout:      10 * time.Second,
					MaxRetries:   3,
				},
			},
			DefaultAccount: "test",
		}

		manager, err := NewManager(config)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		if manager == nil {
			t.Fatal("NewManager() returned nil")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		config := &Config{} // No accounts

		_, err := NewManager(config)
		if err == nil {
			t.Error("NewManager() should return error for invalid config")
		}
	})

	t.Run("multiple accounts", func(t *testing.T) {
		config := &Config{
			Accounts: []AccountConfig{
				{Name: "acc1", ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
				{Name: "acc2", ClientID: "id2", ClientSecret: "secret2", BaseURL: URLSandbox},
			},
			DefaultAccount: "acc1",
		}

		manager, err := NewManager(config)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if len(manager.clients) != 2 {
			t.Errorf("Manager has %d clients, want 2", len(manager.clients))
		}
	})
}

func TestManager_GetClient(t *testing.T) {
	config := &Config{
		Accounts: []AccountConfig{
			{Name: "test1", ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
			{Name: "test2", ClientID: "id2", ClientSecret: "secret2", BaseURL: URLSandbox},
		},
		DefaultAccount: "test1",
	}

	manager, _ := NewManager(config)

	t.Run("existing client", func(t *testing.T) {
		client, err := manager.GetClient("test1")
		if err != nil {
			t.Fatalf("GetClient() error = %v", err)
		}
		if client == nil {
			t.Error("GetClient() returned nil")
		}
	})

	t.Run("non-existent client", func(t *testing.T) {
		_, err := manager.GetClient("nonexistent")
		if err == nil {
			t.Error("GetClient() should return error for non-existent account")
		}
	})
}

func TestManager_GetDefaultClient(t *testing.T) {
	t.Run("with default account", func(t *testing.T) {
		config := &Config{
			Accounts: []AccountConfig{
				{Name: "test", ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
			},
			DefaultAccount: "test",
		}

		manager, _ := NewManager(config)
		client, err := manager.GetDefaultClient()
		if err != nil {
			t.Fatalf("GetDefaultClient() error = %v", err)
		}
		if client == nil {
			t.Error("GetDefaultClient() returned nil")
		}
	})

	t.Run("single account without default", func(t *testing.T) {
		config := &Config{
			Accounts: []AccountConfig{
				{Name: "test", ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
			},
		}

		manager, _ := NewManager(config)
		client, err := manager.GetDefaultClient()
		if err != nil {
			t.Fatalf("GetDefaultClient() error = %v", err)
		}
		if client == nil {
			t.Error("GetDefaultClient() returned nil")
		}
	})

	t.Run("multiple accounts without default", func(t *testing.T) {
		config := &Config{
			Accounts: []AccountConfig{
				{Name: "test1", ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
				{Name: "test2", ClientID: "id2", ClientSecret: "secret2", BaseURL: URLSandbox},
			},
		}

		manager, _ := NewManager(config)
		_, err := manager.GetDefaultClient()
		if err == nil {
			t.Error("GetDefaultClient() should return error when multiple accounts and no default")
		}
	})
}

func TestManager_ListAccounts(t *testing.T) {
	config := &Config{
		Accounts: []AccountConfig{
			{Name: "test1", ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
			{Name: "test2", ClientID: "id2", ClientSecret: "secret2", BaseURL: URLSandbox},
		},
	}

	manager, _ := NewManager(config)
	accounts := manager.ListAccounts()

	if len(accounts) != 2 {
		t.Errorf("ListAccounts() returned %d accounts, want 2", len(accounts))
	}
}

func TestManager_AddAccount(t *testing.T) {
	config := &Config{
		Accounts: []AccountConfig{
			{Name: "test1", ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
		},
	}

	manager, _ := NewManager(config)

	t.Run("add new account", func(t *testing.T) {
		newAccount := AccountConfig{
			Name:         "test2",
			ClientID:     "id2",
			ClientSecret: "secret2",
			BaseURL:      URLSandbox,
		}

		err := manager.AddAccount(newAccount)
		if err != nil {
			t.Fatalf("AddAccount() error = %v", err)
		}

		// Verify account was added
		_, err = manager.GetClient("test2")
		if err != nil {
			t.Error("Added account not found")
		}
	})

	t.Run("add duplicate account", func(t *testing.T) {
		duplicate := AccountConfig{
			Name:         "test1",
			ClientID:     "id",
			ClientSecret: "secret",
			BaseURL:      URLProduction,
		}

		err := manager.AddAccount(duplicate)
		if err == nil {
			t.Error("AddAccount() should return error for duplicate")
		}
	})
}

func TestManager_RemoveAccount(t *testing.T) {
	config := &Config{
		Accounts: []AccountConfig{
			{Name: "test1", ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
			{Name: "test2", ClientID: "id2", ClientSecret: "secret2", BaseURL: URLSandbox},
		},
	}

	manager, _ := NewManager(config)

	t.Run("remove existing account", func(t *testing.T) {
		err := manager.RemoveAccount("test2")
		if err != nil {
			t.Fatalf("RemoveAccount() error = %v", err)
		}

		// Verify account was removed
		_, err = manager.GetClient("test2")
		if err == nil {
			t.Error("Removed account still exists")
		}
	})

	t.Run("remove non-existent account", func(t *testing.T) {
		err := manager.RemoveAccount("nonexistent")
		if err == nil {
			t.Error("RemoveAccount() should return error for non-existent account")
		}
	})
}

func TestManager_HealthCheck(t *testing.T) {
	config := &Config{
		Accounts: []AccountConfig{
			{Name: "test", ClientID: "id", ClientSecret: "secret", BaseURL: "http://localhost:9999"},
		},
	}

	manager, _ := NewManager(config)

	// Health check будет возвращать ошибки т.к. API не доступен
	ctx := context.Background()
	results := manager.HealthCheck(ctx)

	if len(results) != 1 {
		t.Errorf("HealthCheck() returned %d results, want 1", len(results))
	}

	// Ожидаем ошибку т.к. API недоступен
	if _, ok := results["test"]; !ok {
		t.Error("HealthCheck() missing result for 'test' account")
	}
}
