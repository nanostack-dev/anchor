//nolint:testpackage // exercises the unexported validateInstanceIngestionState directly.
package service

import (
	"context"
	"errors"
	"testing"

	"anchor/internal/domain/integration"
	"anchor/internal/integration/provider"

	"github.com/rs/zerolog"
)

// outboundProvider implements only provider.Provider (e.g. SMTP): no inbound
// webhook ingestion, so a webhook secret must never be required.
type outboundProvider struct{}

func (outboundProvider) Type() string                                 { return "SMTP" }
func (outboundProvider) LatestConfigVersion() int32                   { return 1 }
func (outboundProvider) ValidateConfig(context.Context, []byte) error { return nil }
func (outboundProvider) MigrateConfig(context.Context, int32, []byte) ([]byte, error) {
	return nil, nil
}

// ingestProvider also implements provider.WebhookIngestor (e.g. CLERK): an
// active instance must carry a webhook secret.
type ingestProvider struct{ outboundProvider }

func (ingestProvider) ValidateWebhook(context.Context, []byte, map[string]string, string) error {
	return nil
}
func (ingestProvider) ExtractEventType([]byte) string { return "" }
func (ingestProvider) ParseEvent(context.Context, string, []byte) (any, error) {
	return nil, nil
}
func (ingestProvider) ToStandardsCommands(context.Context, string, any) ([]provider.Command, error) {
	return nil, nil
}
func (ingestProvider) ExecuteCommand(
	context.Context, zerolog.Logger, *integration.Instance, provider.Command,
) error {
	return nil
}

func TestValidateInstanceIngestionState(t *testing.T) {
	secret := "whsec_abc"
	empty := "   "

	tests := []struct {
		name    string
		status  integration.Status
		secret  *string
		prov    provider.Provider
		wantErr error
	}{
		{
			name:    "outbound provider active without secret is allowed",
			status:  integration.StatusActive,
			secret:  nil,
			prov:    outboundProvider{},
			wantErr: nil,
		},
		{
			name:    "ingest provider active without secret is rejected",
			status:  integration.StatusActive,
			secret:  nil,
			prov:    ingestProvider{},
			wantErr: ErrIntegrationWebhookSecretRequired,
		},
		{
			name:    "ingest provider active with blank secret is rejected",
			status:  integration.StatusActive,
			secret:  &empty,
			prov:    ingestProvider{},
			wantErr: ErrIntegrationWebhookSecretRequired,
		},
		{
			name:    "ingest provider active with secret is allowed",
			status:  integration.StatusActive,
			secret:  &secret,
			prov:    ingestProvider{},
			wantErr: nil,
		},
		{
			name:    "ingest provider inactive without secret is allowed",
			status:  integration.StatusInactive,
			secret:  nil,
			prov:    ingestProvider{},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := &integration.Instance{Status: tt.status, WebhookSecret: tt.secret}
			if got := validateInstanceIngestionState(inst, tt.prov); !errors.Is(got, tt.wantErr) {
				t.Fatalf("validateInstanceIngestionState() = %v, want %v", got, tt.wantErr)
			}
		})
	}
}
