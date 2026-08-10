package v1alpha1

import "testing"

func TestConfigSpecValidate_UniqueWebhookPathSuffixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    ConfigSpec
		wantErr bool
	}{
		{
			name: "single webhook without pathSuffix",
			spec: ConfigSpec{
				NotificationSources: []NotificationSource{{
					Type:    webhookNotificationSourceType,
					Webhook: &WebhookConfig{},
				}},
			},
		},
		{
			name: "multiple webhooks with distinct pathSuffixes",
			spec: ConfigSpec{
				NotificationSources: []NotificationSource{
					{
						Type:    webhookNotificationSourceType,
						Webhook: &WebhookConfig{PathSuffix: "vendor-a"},
					},
					{
						Type:    webhookNotificationSourceType,
						Webhook: &WebhookConfig{PathSuffix: "vendor-b"},
					},
				},
			},
		},
		{
			name: "one empty and one distinct pathSuffix",
			spec: ConfigSpec{
				NotificationSources: []NotificationSource{
					{
						Type:    webhookNotificationSourceType,
						Webhook: &WebhookConfig{},
					},
					{
						Type:    webhookNotificationSourceType,
						Webhook: &WebhookConfig{PathSuffix: "vendor-a"},
					},
				},
			},
		},
		{
			name: "multiple webhooks without pathSuffix",
			spec: ConfigSpec{
				NotificationSources: []NotificationSource{
					{
						Type:    webhookNotificationSourceType,
						Webhook: &WebhookConfig{},
					},
					{
						Type:    webhookNotificationSourceType,
						Webhook: &WebhookConfig{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple webhooks with nil webhook block",
			spec: ConfigSpec{
				NotificationSources: []NotificationSource{
					{Type: webhookNotificationSourceType},
					{Type: webhookNotificationSourceType},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate pathSuffix",
			spec: ConfigSpec{
				NotificationSources: []NotificationSource{
					{
						Type:    webhookNotificationSourceType,
						Webhook: &WebhookConfig{PathSuffix: "vendor-a"},
					},
					{
						Type:    webhookNotificationSourceType,
						Webhook: &WebhookConfig{PathSuffix: "vendor-a"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate pathSuffix ignores non-webhook sources",
			spec: ConfigSpec{
				NotificationSources: []NotificationSource{
					{
						Type:    webhookNotificationSourceType,
						Webhook: &WebhookConfig{PathSuffix: "vendor-a"},
					},
					{
						Type: "TCPSocket",
						TCPSocket: &TCPSocketConfig{
							Host: "127.0.0.1",
							Port: 8000,
						},
					},
					{
						Type:    webhookNotificationSourceType,
						Webhook: &WebhookConfig{PathSuffix: "vendor-a"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.spec.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}
