package cdek

import (
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Accounts: []AccountConfig{
					{
						Name:         "test",
						ClientID:     "id1",
						ClientSecret: "secret1",
						BaseURL:      URLProduction,
					},
				},
				DefaultAccount: "test",
			},
			wantErr: false,
		},
		{
			name:    "no accounts",
			config:  &Config{Accounts: []AccountConfig{}},
			wantErr: true,
		},
		{
			name: "empty account name",
			config: &Config{
				Accounts: []AccountConfig{
					{ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate account name",
			config: &Config{
				Accounts: []AccountConfig{
					{Name: "test", ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
					{Name: "test", ClientID: "id2", ClientSecret: "secret2", BaseURL: URLProduction},
				},
			},
			wantErr: true,
		},
		{
			name: "empty client_id",
			config: &Config{
				Accounts: []AccountConfig{
					{Name: "test", ClientSecret: "secret1", BaseURL: URLProduction},
				},
			},
			wantErr: true,
		},
		{
			name: "empty client_secret",
			config: &Config{
				Accounts: []AccountConfig{
					{Name: "test", ClientID: "id1", BaseURL: URLProduction},
				},
			},
			wantErr: true,
		},
		{
			name: "empty base_url",
			config: &Config{
				Accounts: []AccountConfig{
					{Name: "test", ClientID: "id1", ClientSecret: "secret1"},
				},
			},
			wantErr: true,
		},
		{
			name: "non-existent default account",
			config: &Config{
				Accounts: []AccountConfig{
					{Name: "test", ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
				},
				DefaultAccount: "nonexistent",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_GetAccount(t *testing.T) {
	config := &Config{
		Accounts: []AccountConfig{
			{Name: "test1", ClientID: "id1", ClientSecret: "secret1", BaseURL: URLProduction},
			{Name: "test2", ClientID: "id2", ClientSecret: "secret2", BaseURL: URLSandbox},
		},
	}

	tests := []struct {
		name        string
		accountName string
		wantErr     bool
	}{
		{"existing account", "test1", false},
		{"another existing account", "test2", false},
		{"non-existent account", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, err := config.GetAccount(tt.accountName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.GetAccount() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && acc == nil {
				t.Error("Config.GetAccount() returned nil account")
			}
			if !tt.wantErr && acc.Name != tt.accountName {
				t.Errorf("Config.GetAccount() returned wrong account: got %v, want %v", acc.Name, tt.accountName)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	clientID := "test-client-id"
	clientSecret := "test-client-secret"

	config := DefaultConfig(clientID, clientSecret)

	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if len(config.Accounts) != 1 {
		t.Errorf("DefaultConfig() accounts count = %v, want 1", len(config.Accounts))
	}

	acc := config.Accounts[0]
	if acc.Name != "default" {
		t.Errorf("DefaultConfig() account name = %v, want 'default'", acc.Name)
	}
	if acc.ClientID != clientID {
		t.Errorf("DefaultConfig() client_id = %v, want %v", acc.ClientID, clientID)
	}
	if acc.ClientSecret != clientSecret {
		t.Errorf("DefaultConfig() client_secret = %v, want %v", acc.ClientSecret, clientSecret)
	}
	if acc.BaseURL != URLProduction {
		t.Errorf("DefaultConfig() base_url = %v, want %v", acc.BaseURL, URLProduction)
	}
	if acc.Timeout != 30*time.Second {
		t.Errorf("DefaultConfig() timeout = %v, want 30s", acc.Timeout)
	}
	if acc.MaxRetries != 3 {
		t.Errorf("DefaultConfig() max_retries = %v, want 3", acc.MaxRetries)
	}
	if config.DefaultAccount != "default" {
		t.Errorf("DefaultConfig() default_account = %v, want 'default'", config.DefaultAccount)
	}
}

func TestConstants(t *testing.T) {
	if URLProduction != "https://api.cdek.ru" {
		t.Errorf("URLProduction = %v, want https://api.cdek.ru", URLProduction)
	}
	// URLSandbox теперь указывает на production, так как api.edu.cdek.ru устарел
	if URLSandbox != "https://api.cdek.ru" {
		t.Errorf("URLSandbox = %v, want https://api.cdek.ru", URLSandbox)
	}
}
