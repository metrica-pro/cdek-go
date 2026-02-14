package cdek

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// TokenResponse ответ при получении токена
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// AuthenticatedClient клиент с автоматической OAuth2 авторизацией
type AuthenticatedClient struct {
	config              *AccountConfig
	client              *Client
	clientWithResponses *ClientWithResponses
	httpClient          *http.Client

	// Компоненты (camelCase по регламенту 4.6)
	cache   *tokenCache
	parser  *responseParser
	builder *requestBuilder
}

// NewAuthenticatedClient создает новый аутентифицированный клиент
func NewAuthenticatedClient(config *AccountConfig) (*AuthenticatedClient, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	client, err := NewClient(config.BaseURL, WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	clientWithResponses, err := NewClientWithResponses(config.BaseURL, WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create client with responses: %w", err)
	}

	return &AuthenticatedClient{
		config:              config,
		client:              client,
		clientWithResponses: clientWithResponses,
		httpClient:          httpClient,
		cache:               newTokenCache(),
		parser:              newResponseParser(),
		builder:             newRequestBuilder(config.BaseURL),
	}, nil
}

// GetToken получает или обновляет токен авторизации
func (a *AuthenticatedClient) GetToken(ctx context.Context) (string, error) {
	// Проверяем кеш через компонент
	if token, ok := a.cache.get(); ok {
		return token, nil
	}

	// Запрашиваем новый токен
	params := GetOAuthTokenParams{
		Request: RequestDto{
			ClientId:     a.config.ClientID,
			ClientSecret: a.config.ClientSecret,
			GrantType:    "client_credentials",
		},
	}

	resp, err := a.client.GetOAuthToken(ctx, &params)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	var tokenResp TokenResponse
	if err := a.parser.parse(resp, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	// Сохраняем в кеш через компонент
	a.cache.set(tokenResp.AccessToken, tokenResp.ExpiresIn)

	return tokenResp.AccessToken, nil
}

// Client возвращает базовый клиент с автоматической авторизацией
func (a *AuthenticatedClient) Client() *Client {
	return a.client
}

// Do выполняет HTTP запрос с автоматической авторизацией
func (a *AuthenticatedClient) Do(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	token, err := a.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	// Используем компонент requestBuilder
	req, err := a.builder.
		withAuthorization(token).
		build(ctx, method, path, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	return resp, nil
}

// DoWithResponse выполняет запрос и парсит ответ
func (a *AuthenticatedClient) DoWithResponse(ctx context.Context, method, path string, body, result interface{}) error {
	resp, err := a.Do(ctx, method, path, body)
	if err != nil {
		return err
	}

	return a.parser.parse(resp, result)
}

// ClientWithResponses возвращает клиент с typed responses
func (a *AuthenticatedClient) ClientWithResponses() *ClientWithResponses {
	return a.clientWithResponses
}
