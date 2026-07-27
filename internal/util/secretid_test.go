package util

import "testing"

func TestSecretIdentifierAliases(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		want       []string
	}{
		{
			name:       "plain friendly name is left untouched",
			identifier: "platform/ai-gateway/service-secrets",
			want:       []string{"platform/ai-gateway/service-secrets"},
		},
		{
			name:       "standard partition ARN expands to friendly name",
			identifier: "arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/ai-gateway/service-secrets-78YXTj",
			want: []string{
				"arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/ai-gateway/service-secrets-78YXTj",
				"platform/ai-gateway/service-secrets",
			},
		},
		{
			name:       "gov cloud partition ARN expands to friendly name",
			identifier: "arn:aws-us-gov:secretsmanager:us-gov-west-1:051826739313:secret:my-secret-AbC123",
			want: []string{
				"arn:aws-us-gov:secretsmanager:us-gov-west-1:051826739313:secret:my-secret-AbC123",
				"my-secret",
			},
		},
		{
			name:       "china partition ARN expands to friendly name",
			identifier: "arn:aws-cn:secretsmanager:cn-north-1:051826739313:secret:my-secret-AbC123",
			want: []string{
				"arn:aws-cn:secretsmanager:cn-north-1:051826739313:secret:my-secret-AbC123",
				"my-secret",
			},
		},
		{
			name:       "friendly name that itself ends in a 6-char hyphenated segment only strips the AWS-appended suffix",
			identifier: "arn:aws:secretsmanager:us-east-1:051826739313:secret:my-secret-abc123-XyZ789",
			want: []string{
				"arn:aws:secretsmanager:us-east-1:051826739313:secret:my-secret-abc123-XyZ789",
				"my-secret-abc123",
			},
		},
		{
			// Real Secrets Manager ARNs always carry the AWS-appended suffix, so an
			// ARN-shaped identifier that is too short to plausibly contain one (no
			// hyphen at all before "secret:") is left untouched rather than mangled.
			name:       "arn with a friendly name too short to contain a suffix is left untouched",
			identifier: "arn:aws:secretsmanager:us-east-1:051826739313:secret:x",
			want:       []string{"arn:aws:secretsmanager:us-east-1:051826739313:secret:x"},
		},
		{
			name:       "non-secretsmanager arn is left untouched",
			identifier: "arn:aws:sqs:us-east-1:051826739313:my-queue",
			want:       []string{"arn:aws:sqs:us-east-1:051826739313:my-queue"},
		},
		{
			name:       "empty identifier",
			identifier: "",
			want:       []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SecretIdentifierAliases(tt.identifier)
			if len(got) != len(tt.want) {
				t.Fatalf("SecretIdentifierAliases(%q) = %v, want %v", tt.identifier, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SecretIdentifierAliases(%q)[%d] = %q, want %q", tt.identifier, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSecretIdentifierMatches(t *testing.T) {
	tests := []struct {
		name       string
		candidate  string
		identifier string
		want       bool
	}{
		{
			name:       "exact match, no ARN involved",
			candidate:  "platform/ai-gateway/service-secrets",
			identifier: "platform/ai-gateway/service-secrets",
			want:       true,
		},
		{
			name:       "friendly name in spec matches AWS ARN from event",
			candidate:  "platform/ai-gateway/service-secrets",
			identifier: "arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/ai-gateway/service-secrets-78YXTj",
			want:       true,
		},
		{
			name:       "full ARN in spec still matches identical ARN from event",
			candidate:  "arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/ai-gateway/service-secrets-78YXTj",
			identifier: "arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/ai-gateway/service-secrets-78YXTj",
			want:       true,
		},
		{
			name:       "unrelated secret does not match",
			candidate:  "platform/other-service/service-secrets",
			identifier: "arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/ai-gateway/service-secrets-78YXTj",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SecretIdentifierMatches(tt.candidate, tt.identifier); got != tt.want {
				t.Errorf("SecretIdentifierMatches(%q, %q) = %v, want %v", tt.candidate, tt.identifier, got, tt.want)
			}
		})
	}
}
