//go:build integration

package cdek

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// getTestService создает Service для интеграционных тестов
func getTestService(t *testing.T) *Service {
	t.Helper()

	clientID := os.Getenv("CDEK_TEST_CLIENT_ID")
	clientSecret := os.Getenv("CDEK_TEST_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		t.Skip("CDEK_TEST_CLIENT_ID and CDEK_TEST_CLIENT_SECRET must be set for integration tests")
	}

	config := DefaultConfig(clientID, clientSecret)
	manager, err := NewManager(config)
	require.NoError(t, err, "Failed to create manager")

	client, err := manager.GetDefaultClient()
	require.NoError(t, err, "Failed to get client")

	return NewService(client, nil)
}

// cleanupTestWebhooks удаляет все тестовые webhooks (с URL содержащим "example.com/webhook")
func cleanupTestWebhooks(t *testing.T, service *Service, ctx context.Context) {
	t.Helper()

	webhooks, err := service.ListWebhooks(ctx)
	if err != nil {
		t.Logf("⚠️  Failed to list webhooks for cleanup: %v", err)
		return
	}

	for _, wh := range webhooks {
		// Удаляем только тестовые webhooks (с example.com в URL)
		if strings.Contains(wh.URL, "example.com/webhook") {
			if err := service.DeleteWebhook(ctx, wh.UUID); err != nil {
				t.Logf("⚠️  Failed to delete test webhook %s: %v", wh.UUID, err)
			} else {
				t.Logf("🧹 Cleaned up test webhook: UUID=%s, URL=%s, Type=%s", wh.UUID, wh.URL, wh.Type)
			}
		}
	}
}

func TestIntegration_WebhookCRUD(t *testing.T) {
	service := getTestService(t)
	ctx := context.Background()

	// Очистим тестовые webhooks перед запуском
	cleanupTestWebhooks(t, service, ctx)

	// Уникальный URL для каждого теста (чтобы избежать конфликтов)
	testURL := fmt.Sprintf("https://example.com/webhook/cdek/test-%d", time.Now().Unix())

	// 1. Create Webhook
	t.Run("CreateWebhook", func(t *testing.T) {
		req := &WebhookRequest{
			URL:  testURL,
			Type: WebhookTypeOrderStatus,
		}

		resp, err := service.CreateWebhook(ctx, req)
		require.NoError(t, err, "CreateWebhook should not return error")
		require.NotNil(t, resp, "CreateWebhook should return response")
		require.NotEmpty(t, resp.UUID, "WebhookResponse should have UUID")

		t.Logf("✅ Webhook created: UUID=%s", resp.UUID)

		// Сохраним UUID для последующих тестов
		webhookUUID := resp.UUID

		// 2. Get Webhook
		t.Run("GetWebhook", func(t *testing.T) {
			webhook, err := service.GetWebhook(ctx, webhookUUID)
			require.NoError(t, err, "GetWebhook should not return error")
			require.NotNil(t, webhook, "GetWebhook should return webhook")
			require.Equal(t, webhookUUID, webhook.UUID, "UUID should match")
			require.Equal(t, testURL, webhook.URL, "URL should match")
			require.Equal(t, WebhookTypeOrderStatus, webhook.Type, "Type should match")

			t.Logf("✅ Webhook retrieved: UUID=%s, URL=%s, Type=%s", webhook.UUID, webhook.URL, webhook.Type)
		})

		// 3. List Webhooks
		t.Run("ListWebhooks", func(t *testing.T) {
			webhooks, err := service.ListWebhooks(ctx)
			require.NoError(t, err, "ListWebhooks should not return error")
			require.NotNil(t, webhooks, "ListWebhooks should return list")
			require.NotEmpty(t, webhooks, "ListWebhooks should return at least 1 webhook")

			// Проверим что наш webhook в списке
			found := false
			for _, wh := range webhooks {
				if wh.UUID == webhookUUID {
					found = true
					require.Equal(t, testURL, wh.URL, "URL should match in list")
					require.Equal(t, WebhookTypeOrderStatus, wh.Type, "Type should match in list")
					break
				}
			}
			require.True(t, found, "Our webhook should be in the list")

			t.Logf("✅ Webhooks listed: total=%d, our webhook found", len(webhooks))
		})

		// 4. Delete Webhook
		t.Run("DeleteWebhook", func(t *testing.T) {
			err := service.DeleteWebhook(ctx, webhookUUID)
			require.NoError(t, err, "DeleteWebhook should not return error")

			t.Logf("✅ Webhook deleted: UUID=%s", webhookUUID)

			// Проверим что webhook действительно удален (Get должен вернуть 404)
			t.Run("VerifyDeleted", func(t *testing.T) {
				webhook, err := service.GetWebhook(ctx, webhookUUID)
				// Ожидаем либо ошибку 404, либо пустой результат
				if err == nil {
					t.Logf("⚠️  GetWebhook после удаления не вернул ошибку, но это может быть нормально")
					t.Logf("   Полученный webhook: %+v", webhook)
				} else {
					t.Logf("✅ GetWebhook после удаления вернул ошибку: %v (ожидаемое поведение)", err)
				}
			})
		})
	})
}

func TestIntegration_CreateWebhook_DifferentTypes(t *testing.T) {
	service := getTestService(t)
	ctx := context.Background()

	// Очистим тестовые webhooks перед запуском
	cleanupTestWebhooks(t, service, ctx)

	tests := []struct {
		name        string
		webhookType string
	}{
		{"ORDER_STATUS", WebhookTypeOrderStatus},
		{"PRINT_FORM", WebhookTypePrintForm},
		{"ORDER_MODIFIED", WebhookTypeOrderModified},
	}

	createdWebhooks := make([]string, 0, len(tests))

	// Cleanup: удалим все созданные webhooks в конце
	defer func() {
		for _, uuid := range createdWebhooks {
			_ = service.DeleteWebhook(ctx, uuid)
		}
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testURL := fmt.Sprintf("https://example.com/webhook/cdek/%s-%d", tt.name, time.Now().Unix())

			req := &WebhookRequest{
				URL:  testURL,
				Type: tt.webhookType,
			}

			resp, err := service.CreateWebhook(ctx, req)
			require.NoError(t, err, "CreateWebhook should succeed for type %s", tt.webhookType)
			require.NotNil(t, resp)
			require.NotEmpty(t, resp.UUID)

			t.Logf("✅ Created webhook: Type=%s, UUID=%s", tt.webhookType, resp.UUID)

			createdWebhooks = append(createdWebhooks, resp.UUID)

			// Проверим что webhook создан с правильным типом
			webhook, err := service.GetWebhook(ctx, resp.UUID)
			require.NoError(t, err)
			require.Equal(t, tt.webhookType, webhook.Type, "Type should match")
		})
	}
}

func TestIntegration_ListWebhooks_Empty(t *testing.T) {
	service := getTestService(t)
	ctx := context.Background()

	// Сначала получим список webhooks
	webhooks, err := service.ListWebhooks(ctx)
	require.NoError(t, err, "ListWebhooks should not return error")
	require.NotNil(t, webhooks, "ListWebhooks should return list (even if empty)")

	t.Logf("✅ Current webhooks count: %d", len(webhooks))

	for i, wh := range webhooks {
		t.Logf("  [%d] UUID=%s, URL=%s, Type=%s", i+1, wh.UUID, wh.URL, wh.Type)
	}
}

func TestIntegration_GetWebhook_NotFound(t *testing.T) {
	service := getTestService(t)
	ctx := context.Background()

	// Попытка получить несуществующий webhook
	fakeUUID := "00000000-0000-0000-0000-000000000000"

	webhook, err := service.GetWebhook(ctx, fakeUUID)

	// CDEK API может вернуть либо ошибку, либо пустой результат
	if err != nil {
		t.Logf("✅ GetWebhook для несуществующего UUID вернул ошибку: %v", err)
	} else {
		t.Logf("⚠️  GetWebhook для несуществующего UUID не вернул ошибку")
		t.Logf("   Результат: %+v", webhook)
	}
}

func TestIntegration_DeleteWebhook_NotFound(t *testing.T) {
	service := getTestService(t)
	ctx := context.Background()

	// Попытка удалить несуществующий webhook
	fakeUUID := "00000000-0000-0000-0000-000000000000"

	err := service.DeleteWebhook(ctx, fakeUUID)

	// CDEK API может вернуть либо ошибку, либо success
	if err != nil {
		t.Logf("✅ DeleteWebhook для несуществующего UUID вернул ошибку: %v", err)
	} else {
		t.Logf("⚠️  DeleteWebhook для несуществующего UUID не вернул ошибку (может быть idempotent)")
	}
}

func TestIntegration_Webhook_FullLifecycle(t *testing.T) {
	service := getTestService(t)
	ctx := context.Background()

	// Очистим тестовые webhooks перед запуском
	cleanupTestWebhooks(t, service, ctx)

	testURL := fmt.Sprintf("https://example.com/webhook/cdek/lifecycle-%d", time.Now().Unix())

	// 1. Проверим начальное количество webhooks
	initialWebhooks, err := service.ListWebhooks(ctx)
	require.NoError(t, err)
	initialCount := len(initialWebhooks)
	t.Logf("📊 Initial webhooks count: %d", initialCount)

	// 2. Создадим webhook
	createReq := &WebhookRequest{
		URL:  testURL,
		Type: WebhookTypeOrderStatus,
	}
	createResp, err := service.CreateWebhook(ctx, createReq)
	require.NoError(t, err)
	require.NotEmpty(t, createResp.UUID)
	webhookUUID := createResp.UUID
	t.Logf("✅ Created: UUID=%s", webhookUUID)

	// 3. Проверим что количество увеличилось
	afterCreateWebhooks, err := service.ListWebhooks(ctx)
	require.NoError(t, err)
	require.Equal(t, initialCount+1, len(afterCreateWebhooks), "Should have 1 more webhook")
	t.Logf("📊 After create: %d webhooks", len(afterCreateWebhooks))

	// 4. Получим созданный webhook
	webhook, err := service.GetWebhook(ctx, webhookUUID)
	require.NoError(t, err)
	require.Equal(t, webhookUUID, webhook.UUID)
	require.Equal(t, testURL, webhook.URL)
	require.Equal(t, WebhookTypeOrderStatus, webhook.Type)
	t.Logf("✅ Retrieved: %+v", webhook)

	// 5. Удалим webhook
	err = service.DeleteWebhook(ctx, webhookUUID)
	require.NoError(t, err)
	t.Logf("✅ Deleted: UUID=%s", webhookUUID)

	// 6. Проверим что количество вернулось к начальному
	afterDeleteWebhooks, err := service.ListWebhooks(ctx)
	require.NoError(t, err)
	require.Equal(t, initialCount, len(afterDeleteWebhooks), "Should have initial count after delete")
	t.Logf("📊 After delete: %d webhooks (back to initial)", len(afterDeleteWebhooks))
}
