package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/nanostack-dev/pgkit/queue"
	"github.com/stretchr/testify/require"
)

// caseUndecryptableClerkKey is stored verbatim because it already has the
// versioned "enc:<version>:<payload>" shape, but no cipher key exists for its
// version. Reconcile therefore fails inside resolveConfig, before any HTTP call
// to Clerk can be made. The test never contacts api.clerk.com.
const caseUndecryptableClerkKey = "enc:secaudit-no-such-key-version:cHJvYmU"

// Not parallel: it uses the process-wide Clerk reconcile queue with a keyed Clerk instance.
func TestClerkReconcileSkipsADisabledInstance(t *testing.T) {
	// Guard: the probe key must be in versioned-encrypted form so that the
	// provider stores it verbatim and cannot decrypt it (no outbound request).
	require.True(t, secrets.IsVersionedEncryptedSecret(caseUndecryptableClerkKey))

	productContext := createTestProductContext(t)
	owner := productContext.OwnerAuthenticatedClient()

	createBody := ct.CreateIntegrationInstanceJSONRequestBody{}
	require.NoError(t, createBody.FromClerkIntegrationInstanceCreateRequest(
		ct.ClerkIntegrationInstanceCreateRequest{ProviderType: "CLERK"},
	))
	createResp, err := owner.CreateIntegrationInstanceWithResponse(
		context.Background(), productContext.ProductID, createBody,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)
	instanceID := createResp.JSON201.Id
	t.Cleanup(func() { deleteInstance(t, productContext, instanceID) })

	// One request disables the integration and saves an API key. The same
	// transaction that persists is_enabled=false / INACTIVE decides whether a
	// reconcile job is enqueued, so every job for this instance is post-disable.
	cfg := ct.IntegrationProviderConfig{}
	require.NoError(t, cfg.FromClerkIntegrationConfig(ct.ClerkIntegrationConfig{
		ApiKey: new(caseUndecryptableClerkKey),
	}))
	disabled := false
	updateResp, err := owner.UpdateIntegrationInstanceWithResponse(
		context.Background(), productContext.ProductID, instanceID,
		ct.UpdateIntegrationInstanceJSONRequestBody{Config: &cfg, IsEnabled: &disabled},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, updateResp.StatusCode())
	require.NotNil(t, updateResp.JSON200)
	require.False(t, updateResp.JSON200.IsEnabled)
	require.Equal(t, ct.IntegrationInstanceStatusINACTIVE, updateResp.JSON200.Status)

	// Observe reconcile jobs for this instance. A disabled instance must not be
	// reconciled: either no job exists, or every processed job completes without
	// reaching the Clerk provider (no error from the provider's config/fetch step).
	const (
		observeWindow = 12 * time.Second
		pollEvery     = 250 * time.Millisecond
	)
	deadline := time.Now().Add(observeWindow)
	for time.Now().Before(deadline) {
		jobs, listErr := reconcileQueue.ListJobs(context.Background(), queue.ListJobsParams{
			QueueName: integrationReconcileQueueName,
			Search:    instanceID,
			Limit:     100,
		})
		require.NoError(t, listErr)

		for _, job := range jobs {
			var payload struct {
				IntegrationInstanceID string `json:"integration_instance_id"`
			}
			if json.Unmarshal(job.Payload, &payload) != nil || payload.IntegrationInstanceID != instanceID {
				continue
			}
			require.Falsef(t, job.LastError.Valid, "reconcile reached the provider for a disabled instance: %s",
				job.LastError.String)
		}
		time.Sleep(pollEvery)
	}
}
