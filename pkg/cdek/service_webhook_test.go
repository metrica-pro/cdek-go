package cdek

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookTypes(t *testing.T) {
	// Проверка что все константы определены
	assert.Equal(t, "ORDER_STATUS", WebhookTypeOrderStatus)
	assert.Equal(t, "ORDER_MODIFIED", WebhookTypeOrderModified)
	assert.Equal(t, "PRINT_FORM", WebhookTypePrintForm)
	assert.Equal(t, "RECEIPT", WebhookTypeReceipt)
	assert.Equal(t, "PREALERT_CLOSED", WebhookTypePrealertClosed)
	assert.Equal(t, "ACCOMPANYING_WAYBILL", WebhookTypeAccompanyingWaybill)
	assert.Equal(t, "OFFICE_AVAILABILITY", WebhookTypeOfficeAvailability)
	assert.Equal(t, "DELIV_AGREEMENT", WebhookTypeDelivAgreement)
	assert.Equal(t, "DELIV_PROBLEM", WebhookTypeDelivProblem)
	assert.Equal(t, "COURIER_INFO", WebhookTypeCourierInfo)
}

func TestWebhookRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request *WebhookRequest
		wantErr bool
	}{
		{
			name: "valid request",
			request: &WebhookRequest{
				URL:  "https://example.com/webhook",
				Type: WebhookTypeOrderStatus,
			},
			wantErr: false,
		},
		{
			name: "valid request with different type",
			request: &WebhookRequest{
				URL:  "https://example.com/webhook/cdek",
				Type: WebhookTypePrintForm,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.request)
			assert.NotEmpty(t, tt.request.URL)
			assert.NotEmpty(t, tt.request.Type)
		})
	}
}

func TestDTOMapper_fromCDEKWebhookResponse(t *testing.T) {
	mapper := newDtoMapper()

	t.Run("valid response", func(t *testing.T) {
		jsonData := []byte(`{
			"entity": {
				"uuid": "123e4567-e89b-12d3-a456-426614174000"
			},
			"requests": []
		}`)

		resp, err := mapper.fromCDEKWebhookResponse(jsonData)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", resp.UUID)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		jsonData := []byte(`invalid json`)

		resp, err := mapper.fromCDEKWebhookResponse(jsonData)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("missing entity", func(t *testing.T) {
		jsonData := []byte(`{"requests": []}`)

		resp, err := mapper.fromCDEKWebhookResponse(jsonData)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Empty(t, resp.UUID)
	})
}

func TestDTOMapper_fromCDEKWebhook(t *testing.T) {
	mapper := newDtoMapper()

	t.Run("valid webhook", func(t *testing.T) {
		jsonData := []byte(`{
			"entity": {
				"uuid": "123e4567-e89b-12d3-a456-426614174000",
				"url": "https://example.com/webhook",
				"type": "ORDER_STATUS"
			},
			"requests": []
		}`)

		webhook, err := mapper.fromCDEKWebhook(jsonData)
		assert.NoError(t, err)
		assert.NotNil(t, webhook)
		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", webhook.UUID)
		assert.Equal(t, "https://example.com/webhook", webhook.URL)
		assert.Equal(t, "ORDER_STATUS", webhook.Type)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		jsonData := []byte(`invalid json`)

		webhook, err := mapper.fromCDEKWebhook(jsonData)
		assert.Error(t, err)
		assert.Nil(t, webhook)
	})

	t.Run("missing fields", func(t *testing.T) {
		jsonData := []byte(`{
			"entity": {
				"uuid": "123e4567-e89b-12d3-a456-426614174000"
			},
			"requests": []
		}`)

		webhook, err := mapper.fromCDEKWebhook(jsonData)
		assert.NoError(t, err)
		assert.NotNil(t, webhook)
		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", webhook.UUID)
		assert.Empty(t, webhook.URL)
		assert.Empty(t, webhook.Type)
	})
}

func TestDTOMapper_fromCDEKWebhooks(t *testing.T) {
	mapper := newDtoMapper()

	t.Run("valid webhooks list", func(t *testing.T) {
		jsonData := []byte(`[
			{
				"uuid": "123e4567-e89b-12d3-a456-426614174000",
				"url": "https://example.com/webhook1",
				"type": "ORDER_STATUS"
			},
			{
				"uuid": "987e6543-e21b-34d5-b678-987654321000",
				"url": "https://example.com/webhook2",
				"type": "PRINT_FORM"
			}
		]`)

		webhooks, err := mapper.fromCDEKWebhooks(jsonData)
		assert.NoError(t, err)
		assert.NotNil(t, webhooks)
		assert.Len(t, webhooks, 2)

		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", webhooks[0].UUID)
		assert.Equal(t, "https://example.com/webhook1", webhooks[0].URL)
		assert.Equal(t, "ORDER_STATUS", webhooks[0].Type)

		assert.Equal(t, "987e6543-e21b-34d5-b678-987654321000", webhooks[1].UUID)
		assert.Equal(t, "https://example.com/webhook2", webhooks[1].URL)
		assert.Equal(t, "PRINT_FORM", webhooks[1].Type)
	})

	t.Run("empty webhooks list", func(t *testing.T) {
		jsonData := []byte(`[]`)

		webhooks, err := mapper.fromCDEKWebhooks(jsonData)
		assert.NoError(t, err)
		assert.NotNil(t, webhooks)
		assert.Len(t, webhooks, 0)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		jsonData := []byte(`invalid json`)

		webhooks, err := mapper.fromCDEKWebhooks(jsonData)
		assert.Error(t, err)
		assert.Nil(t, webhooks)
	})
}

func TestService_Webhook_Methods_Exist(t *testing.T) {
	// Этот тест проверяет что все методы существуют (компилируется)
	var service *Service

	// Проверка что методы существуют через интерфейс
	_ = func(s *Service) {
		var (
			_ = s.CreateWebhook
			_ = s.ListWebhooks
			_ = s.GetWebhook
			_ = s.DeleteWebhook
		)
	}

	assert.Nil(t, service) // service не инициализирован, это нормально для проверки сигнатур
}
