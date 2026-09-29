package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/pgkit/queue"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// reconcileQueue is populated by fx.Populate via ExtraPopulateTargets so that tests can
// inspect queue state directly without going through the HTTP API.
var reconcileQueue *queue.Client

const integrationReconcileQueueName = "integration-reconcile"

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// countPendingSchedulerJobs returns the number of pending scheduler jobs in the
// integration-reconcile queue. A job is considered a scheduler job when its
// payload contains "is_scheduler": true.
func countPendingSchedulerJobs(t *testing.T) int {
	t.Helper()

	const maxJobs = 1000
	jobs, err := reconcileQueue.ListJobs(context.Background(), queue.ListJobsParams{
		QueueName: integrationReconcileQueueName,
		Status:    queue.StatusPending,
		Limit:     maxJobs,
	})
	require.NoError(t, err)

	count := 0
	for _, job := range jobs {
		var payload struct {
			IsScheduler bool `json:"is_scheduler"`
		}
		if jsonErr := json.Unmarshal(job.Payload, &payload); jsonErr == nil && payload.IsScheduler {
			count++
		}
	}

	return count
}

// createInstanceWithAPIKey creates a Clerk integration instance and immediately
// updates it with the provided API key so that maybeStartReconcileScheduler is triggered.
// apiKey is kept as a parameter so that future tests can supply different keys.
func createInstanceWithAPIKey(
	t *testing.T,
	productContext *itdsl.ProductContext,
	apiKey string, //nolint:unparam // parameterised for future flexibility; tests always use fakeAPIKey today
) *ct.IntegrationInstanceResponse {
	t.Helper()

	createBody := ct.CreateIntegrationInstanceJSONRequestBody{}
	require.NoError(
		t,
		createBody.FromClerkIntegrationInstanceCreateRequest(
			ct.ClerkIntegrationInstanceCreateRequest{ProviderType: "CLERK"},
		),
	)

	createResp, err := productContext.OwnerAuthenticatedClient().CreateIntegrationInstanceWithResponse(
		context.Background(),
		productContext.ProductID,
		createBody,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)

	// Attach the API key via update so that maybeStartReconcileScheduler runs post-tx.
	cfg := ct.IntegrationProviderConfig{}
	require.NoError(t, cfg.FromClerkIntegrationConfig(ct.ClerkIntegrationConfig{
		ApiKey: new(apiKey),
	}))

	updateResp, err := productContext.OwnerAuthenticatedClient().UpdateIntegrationInstanceWithResponse(
		context.Background(),
		productContext.ProductID,
		createResp.JSON201.Id,
		ct.UpdateIntegrationInstanceJSONRequestBody{Config: &cfg},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, updateResp.StatusCode())
	require.NotNil(t, updateResp.JSON200)

	return updateResp.JSON200
}

// removeAPIKey updates the instance to clear its API key by passing an empty config.
func removeAPIKey(
	t *testing.T,
	productContext *itdsl.ProductContext,
	instanceID string,
) *ct.IntegrationInstanceResponse {
	t.Helper()

	cfg := ct.IntegrationProviderConfig{}
	require.NoError(t, cfg.FromClerkIntegrationConfig(ct.ClerkIntegrationConfig{
		ApiKey: nil,
	}))

	resp, err := productContext.OwnerAuthenticatedClient().UpdateIntegrationInstanceWithResponse(
		context.Background(),
		productContext.ProductID,
		instanceID,
		ct.UpdateIntegrationInstanceJSONRequestBody{Config: &cfg},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.NotNil(t, resp.JSON200)

	return resp.JSON200
}

// deleteInstance deletes the given integration instance.
func deleteInstance(
	t *testing.T,
	productContext *itdsl.ProductContext,
	instanceID string,
) {
	t.Helper()

	resp, err := productContext.OwnerAuthenticatedClient().DeleteIntegrationInstanceWithResponse(
		context.Background(),
		productContext.ProductID,
		instanceID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode())
}
