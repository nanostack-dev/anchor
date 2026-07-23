package reconcile_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
)

// Queue is populated by fx.Populate via ExtraPopulateTargets so that tests can
// inspect pgqueue state directly without going through the HTTP API.
var Queue *pgqueue.Client

const integrationReconcileQueueName = "integration-reconcile"

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}

	itshared.RunTestMain(
		m, itshared.TestConfig{
			EnableRedis:             true,
			PopulateRepositories:    true,
			APIKeyService:           &itshared.APIKeyService,
			PermissionRepository:    &itshared.PermissionRepository,
			ProductRepository:       &itshared.ProductRepository,
			ProductUserRepository:   &itshared.ProductUserRepository,
			OrgMembershipRepository: &itshared.OrgMembershipRepository,
			TenantRepository:        &itshared.TenantRepository,
			UserRepository:          &itshared.UserRepository,
			PlatformUserRepository:  &itshared.PlatformTenantUserRepo,
			JWTHelper:               &itshared.JWTHelper,
			ExtraPopulateTargets:    []any{&Queue},
		},
	)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// countPendingSchedulerJobs returns the number of live scheduler jobs in the
// integration-reconcile queue. A job is considered a scheduler job when its
// payload contains "is_scheduler": true.
//
// Both PENDING and PROCESSING count as live. The scheduler job re-enqueues
// itself, so a worker that has claimed it leaves no PENDING row until the
// handler commits the next one; counting only PENDING makes the scheduler
// appear to vanish for the width of that window and turns every
// "count did not change" assertion into a race.
func countPendingSchedulerJobs(t *testing.T) int {
	t.Helper()

	const maxJobs = 1000

	count := 0
	for _, status := range []pgqueue.JobStatus{pgqueue.StatusPending, pgqueue.StatusProcessing} {
		jobs, err := Queue.ListJobs(context.Background(), pgqueue.ListJobsParams{
			QueueName: integrationReconcileQueueName,
			Status:    status,
			Limit:     maxJobs,
		})
		require.NoError(t, err)

		for _, job := range jobs {
			var payload struct {
				IsScheduler bool `json:"is_scheduler"`
			}
			if jsonErr := json.Unmarshal(job.Payload, &payload); jsonErr == nil && payload.IsScheduler {
				count++
			}
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
