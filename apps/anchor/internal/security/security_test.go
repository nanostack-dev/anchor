package security_test

import (
	"strings"
	"testing"

	"anchor/internal/security"
)

const (
	validAPIKey             = "anchor_prd_apikey_9X6jJUsSGSfbqL0bqawCPQK19rl2OIJSTvKOk8zeuTMqz3CZ_86cd7e60"
	hashedAPIKey            = "495b1076e751b344586f162deeea2f7e419ab692378514195aec517640ed96bd"
	validOrganizationAPIKey = "anchor_org_apikey_9X6jJUsSGSfbqL0bqawCPQK19rl2OIJSTvKOk8zeuTMqz3CZ_86cd7e60"
	customRootPrefix        = "acme"
)

func TestCompareAPIKey(t *testing.T) {
	type args struct {
		apiKey       string
		hashedAPIKey string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Valid API key comparison",
			args: args{
				apiKey:       validAPIKey,
				hashedAPIKey: hashedAPIKey,
			},
			want: true,
		},
		{
			name: "Invalid API key comparison",
			args: args{
				apiKey:       "invalid_key",
				hashedAPIKey: hashedAPIKey,
			},
			want: false,
		},
		{
			name: "Empty API key",
			args: args{
				apiKey:       "",
				hashedAPIKey: hashedAPIKey,
			},
			want: false,
		},
		{
			name: "Empty hashed API key",
			args: args{
				apiKey:       validAPIKey,
				hashedAPIKey: "",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if got := security.CompareAPIKey(tt.args.apiKey, tt.args.hashedAPIKey); got != tt.want {
					t.Errorf("security.CompareAPIKey() = %v, want %v", got, tt.want)
				}
			},
		)
	}
}

func TestGenerateProductAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "Generate valid API key",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got, err := security.GenerateProductAPIKey()
				if (err != nil) != tt.wantErr {
					t.Errorf("security.GenerateProductAPIKey() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if err == nil {
					if !security.IsValidProductAPIKey(got) {
						t.Errorf("security.GenerateProductAPIKey() generated invalid API key: %v", got)
					}
					if !strings.HasPrefix(got, security.DefaultProductAPIKeyPrefix) {
						t.Errorf("security.GenerateProductAPIKey() missing correct prefix: %v", got)
					}
					if len(got) != security.ProductAPIKeyLength() {
						t.Errorf(
							"security.GenerateProductAPIKey() incorrect length: got %d, want %d", len(got),
							security.ProductAPIKeyLength(),
						)
					}
				}
			},
		)
	}
}

func TestGenerateOrganizationAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "Generate valid organization API key",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got, err := security.GenerateOrganizationAPIKey(security.DefaultOrganizationAPIKeyRootPrefix)
				if (err != nil) != tt.wantErr {
					t.Errorf("security.GenerateOrganizationAPIKey() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if err == nil {
					if !security.IsValidOrganizationAPIKey(security.DefaultOrganizationAPIKeyRootPrefix, got) {
						t.Errorf("security.GenerateOrganizationAPIKey() generated invalid API key: %v", got)
					}
					if !strings.HasPrefix(got, security.DefaultOrganizationAPIKeyPrefix) {
						t.Errorf("security.GenerateOrganizationAPIKey() missing correct prefix: %v", got)
					}
					if len(got) != security.OrganizationAPIKeyLength(security.DefaultOrganizationAPIKeyRootPrefix) {
						t.Errorf(
							"security.GenerateOrganizationAPIKey() incorrect length: got %d, want %d",
							len(got),
							security.OrganizationAPIKeyLength(security.DefaultOrganizationAPIKeyRootPrefix),
						)
					}
				}
			},
		)
	}
}

func TestGenerateAPIKeysWithCustomOrganizationRootPrefix(t *testing.T) {
	productKey, err := security.GenerateProductAPIKey()
	if err != nil {
		t.Fatalf("security.GenerateProductAPIKey() failed: %v", err)
	}
	if !strings.HasPrefix(productKey, security.DefaultProductAPIKeyPrefix) {
		t.Fatalf("product API key has unexpected prefix: %s", productKey)
	}
	if strings.HasPrefix(productKey, customRootPrefix+"_prd_apikey_") {
		t.Fatalf("product API key should not use custom organization prefix: %s", productKey)
	}
	if !security.IsValidProductAPIKey(productKey) {
		t.Fatalf("product API key is invalid: %s", productKey)
	}

	organizationKey, err := security.GenerateOrganizationAPIKey(customRootPrefix)
	if err != nil {
		t.Fatalf("security.GenerateOrganizationAPIKey() failed: %v", err)
	}
	if !strings.HasPrefix(organizationKey, security.OrganizationAPIKeyPrefix(customRootPrefix)) {
		t.Fatalf("organization API key has unexpected prefix: %s", organizationKey)
	}
	if !security.IsValidOrganizationAPIKey(customRootPrefix, organizationKey) {
		t.Fatalf("organization API key with custom prefix is invalid: %s", organizationKey)
	}
}

func TestIsValidOrganizationAPIKeyRootPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   bool
	}{
		{name: "default", prefix: security.DefaultOrganizationAPIKeyRootPrefix, want: true},
		{name: "lowercase with number", prefix: "acme2", want: true},
		{name: "internal underscore", prefix: "acme_prod", want: true},
		{name: "too short", prefix: "a", want: false},
		{name: "starts with number", prefix: "1acme", want: false},
		{name: "uppercase", prefix: "Acme", want: false},
		{name: "dash", prefix: "acme-prod", want: false},
		{name: "trailing underscore", prefix: "acme_", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := security.IsValidOrganizationAPIKeyRootPrefix(tt.prefix); got != tt.want {
				t.Fatalf("security.IsValidOrganizationAPIKeyRootPrefix(%q) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestHashSecret(t *testing.T) {
	type args struct {
		apiKey string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Hash valid API key",
			args: args{
				apiKey: validAPIKey,
			},
			want: hashedAPIKey,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if got := security.HashSecret(tt.args.apiKey); got != tt.want {
					t.Errorf("security.HashSecret() = %v, want %v", got, tt.want)
				}
			},
		)
	}
}

func TestIsValidProductAPIKey(t *testing.T) {
	type args struct {
		apiKey string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Valid API key",
			args: args{
				apiKey: validAPIKey,
			},
			want: true,
		},
		{
			name: "Empty API key",
			args: args{
				apiKey: "",
			},
			want: false,
		},
		{
			name: "Invalid prefix",
			args: args{
				apiKey: "invalid_prefix_NlNpZWllYW1qRzZnNVA4VWF0bFVWbUNwbU5lTmhPY1ZHZ2FlUExEcGhzM1NEZjRy_65bbe609",
			},
			want: false,
		},
		{
			name: "Too short API key",
			args: args{
				apiKey: "anchor_prd_apikey_short",
			},
			want: false,
		},
		{
			name: "Invalid checksum",
			args: args{
				apiKey: "anchor_prd_apikey_NlNpZWllYW1qRzZnNVA4VWF0bFVWbUNwbU5lTmhPY1ZHZ2FlUExEcGhzM1NEZjRy_invalidcrc",
			},
			want: false,
		},
		{
			name: "Missing checksum separator",
			args: args{
				apiKey: "anchor_prd_apikey_NlNpZWllYW1qRzZnNVA4VWF0bFVWbUNwbU5lTmhPY1ZHZ2FlUExEcGhzM1NEZjRy65bbe609",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if got := security.IsValidProductAPIKey(tt.args.apiKey); got != tt.want {
					t.Errorf("security.IsValidProductAPIKey() = %v, want %v", got, tt.want)
				}
			},
		)
	}
}

func TestIsValidOrganizationAPIKey(t *testing.T) {
	tests := []struct {
		name string
		args struct{ apiKey string }
		want bool
	}{
		{
			name: "Valid API key",
			args: struct{ apiKey string }{apiKey: validOrganizationAPIKey},
			want: true,
		},
		{
			name: "Empty API key",
			args: struct{ apiKey string }{apiKey: ""},
			want: false,
		},
		{
			name: "Invalid prefix",
			args: struct{ apiKey string }{apiKey: validAPIKey},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if got := security.IsValidOrganizationAPIKey(
					security.DefaultOrganizationAPIKeyRootPrefix,
					tt.args.apiKey,
				); got != tt.want {
					t.Errorf("security.IsValidOrganizationAPIKey() = %v, want %v", got, tt.want)
				}
			},
		)
	}
}

func TestObfuscateProductAPIKey(t *testing.T) {
	type args struct {
		apiKey string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Obfuscate valid API key",
			args: args{
				apiKey: validAPIKey,
			},
			want: "anchor_prd_apikey_***_86cd7e60",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if got := security.ObfuscateProductAPIKey(tt.args.apiKey); got != tt.want {
					t.Errorf("security.ObfuscateProductAPIKey() = %v, want %v", got, tt.want)
				}
			},
		)
	}
}

func TestObfuscateOrganizationAPIKey(t *testing.T) {
	tests := []struct {
		name string
		args struct{ apiKey string }
		want string
	}{
		{
			name: "Obfuscate valid API key",
			args: struct{ apiKey string }{apiKey: validOrganizationAPIKey},
			want: "anchor_org_apikey_***_86cd7e60",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if got := security.ObfuscateOrganizationAPIKey(
					security.DefaultOrganizationAPIKeyRootPrefix,
					tt.args.apiKey,
				); got != tt.want {
					t.Errorf("security.ObfuscateOrganizationAPIKey() = %v, want %v", got, tt.want)
				}
			},
		)
	}
}

func TestGenerateMultipleUniqueAPIKeys(t *testing.T) {
	const numKeys = 100
	keys := make(map[string]bool)

	for i := range numKeys {
		key, err := security.GenerateProductAPIKey()
		if err != nil {
			t.Fatalf("security.GenerateProductAPIKey() failed on iteration %d: %v", i, err)
		}

		if keys[key] {
			t.Fatalf("security.GenerateProductAPIKey() generated duplicate key: %s", key)
		}
		keys[key] = true

		// Verify each generated key is valid
		if !security.IsValidProductAPIKey(key) {
			t.Fatalf("security.GenerateProductAPIKey() generated invalid key on iteration %d: %s", i, key)
		}
	}
}

func TestValidateSecretFormat(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		want   bool
	}{
		{
			name:   "Valid format with correct checksum",
			apiKey: validAPIKey,
			want:   true,
		},
		{
			name:   "Generated key should be valid",
			apiKey: "",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				testKey := tt.apiKey
				if testKey == "" {
					// Generate a fresh key for testing
					var err error
					testKey, err = security.GenerateProductAPIKey()
					if err != nil {
						t.Fatalf("Failed to generate test key: %v", err)
					}
				}

				if got := security.IsValidProductAPIKey(testKey); got != tt.want {
					t.Errorf(
						"security.IsValidProductAPIKey() = %v, want %v for key: %s", got, tt.want, testKey,
					)
				}
			},
		)
	}
}

func TestAPIKeyRoundTrip(t *testing.T) {
	apiKey, err := security.GenerateProductAPIKey()
	if err != nil {
		t.Fatalf("security.GenerateProductAPIKey() failed: %v", err)
	}

	hashedKey := security.HashSecret(apiKey)
	if hashedKey == "" {
		t.Fatal("security.HashSecret() returned empty string")
	}

	if !security.CompareAPIKey(apiKey, hashedKey) {
		t.Error("security.CompareAPIKey() failed for generated and hashed key")
	}

	wrongKey := "wrong_key"
	if security.CompareAPIKey(wrongKey, hashedKey) {
		t.Error("security.CompareAPIKey() incorrectly matched wrong key")
	}

	obfuscated := security.ObfuscateProductAPIKey(apiKey)
	if !strings.HasPrefix(obfuscated, security.DefaultProductAPIKeyPrefix) {
		t.Errorf("security.ObfuscateProductAPIKey() didn't preserve prefix: %s", obfuscated)
	}
	if !strings.Contains(obfuscated, "***") {
		t.Errorf("security.ObfuscateProductAPIKey() didn't obfuscate middle: %s", obfuscated)
	}
}
