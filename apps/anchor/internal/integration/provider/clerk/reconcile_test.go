//nolint:testpackage // tests require access to unexported symbols (resolveConfig, newTestProvider)
package clerk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/secret"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"anchor/internal/repository"
	"anchor/internal/security/encryption"
	serviceconfig "anchor/internal/service/config"

	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareConfigForStorageEncryptsAPIKey(t *testing.T) {
	p := newTestProvider(t)

	normalized, err := p.PrepareConfigForStorage(
		context.Background(),
		[]byte(`{"api_key":"sk_test_123"}`),
	)
	require.NoError(t, err)

	var stored Config
	require.NoError(t, json.Unmarshal(normalized, &stored))
	require.NotEqual(t, "sk_test_123", stored.APIKey)
	require.True(t, secrets.IsVersionedEncryptedSecret(stored.APIKey))

	resolved, err := p.resolveConfig(normalized)
	require.NoError(t, err)
	require.Equal(t, "sk_test_123", resolved.APIKey)
}

func TestPrepareConfigForStorageDoesNotDoubleEncrypt(t *testing.T) {
	p := newTestProvider(t)

	// First pass
	normalized1, err := p.PrepareConfigForStorage(
		context.Background(),
		[]byte(`{"api_key":"sk_test_123"}`),
	)
	require.NoError(t, err)

	// Second pass (e.g. updating some other config value, while keeping the encrypted API key)
	normalized2, err := p.PrepareConfigForStorage(
		context.Background(),
		normalized1,
	)
	require.NoError(t, err)

	// Ensure the second pass doesn't modify the already encrypted config
	require.Equal(t, string(normalized1), string(normalized2))

	// Ensure it still resolves correctly
	resolved, err := p.resolveConfig(normalized2)
	require.NoError(t, err)
	require.Equal(t, "sk_test_123", resolved.APIKey)
}

func TestPrepareConfigForStorageRequiresEncryptionKeyWhenAPIKeyProvided(t *testing.T) {
	p := &Provider{}

	_, err := p.PrepareConfigForStorage(
		context.Background(),
		[]byte(`{"api_key":"sk_test_123"}`),
	)
	require.ErrorIs(t, err, errMissingEncryptionKey)
}

func TestReconcileBuildsCommandsFromPaginatedClerkUsers(t *testing.T) {
	t.Parallel()

	apiKey := "sk_test_abc"
	wiremockURL := startWireMock(t)

	p := newTestProvider(t)
	p.baseURL = wiremockURL + "/v1"

	res, err := p.Reconcile(
		context.Background(),
		[]byte(`{"api_key":"`+apiKey+`"}`),
	)
	require.NoError(t, err)
	require.Len(t, res.Upserts, 3)
	require.Len(t, res.SeenIDs, 3)
	require.False(t, res.SnapshotStart.IsZero())

	for _, cmd := range res.Upserts {
		require.Equal(t, CommandUpsertUser, cmd.Type)
		upsertData, ok := cmd.Data.(UpsertUserData)
		require.True(t, ok)
		require.NotEmpty(t, upsertData.ExternalID)
		require.NotEmpty(t, upsertData.Email)
		require.Equal(t, SyncSourceReconcile, upsertData.Source)
	}
}

func startWireMock(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	wd, err := os.Getwd()
	require.NoError(t, err)
	mockDir := filepath.Join(wd, "testdata", "wiremock")

	req := testcontainers.ContainerRequest{
		Image:        "wiremock/wiremock:3.3.1",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor: wait.ForHTTP("/__admin/health").
			WithPort("8080/tcp").
			WithStartupTimeout(30 * time.Second),
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.Binds = []string{mockDir + ":/home/wiremock:ro"}
		},
	}

	wm, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, testcontainers.TerminateContainer(wm))
	})

	host, err := wm.Host(ctx)
	require.NoError(t, err)
	port, err := wm.MappedPort(ctx, "8080/tcp")
	require.NoError(t, err)

	return "http://" + net.JoinHostPort(host, port.Port())
}

func newTestProvider(t *testing.T) *Provider {
	t.Helper()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	encService, err := encryption.NewService(&serviceconfig.CoreConfig{
		Encryption: serviceconfig.EncryptionConfig{
			GlobalKey:        secret.New(base64.StdEncoding.EncodeToString(key)),
			GlobalKeyVersion: "v1",
		},
	})
	require.NoError(t, err)

	return NewProvider(nil, nil, nil, encService, nil, nil, slog.New(slog.DiscardHandler))
}

type fakeReconcileRepo struct {
	repository.ProductUserRepository
	stampedCount int64
	beforeIDs    []string
}

func (f *fakeReconcileRepo) CountClerkStampedUsers(_ context.Context, _ string) (int64, error) {
	return f.stampedCount, nil
}

func (f *fakeReconcileRepo) ListClerkStampedExternalIDsBefore(
	_ context.Context, _ string, _ time.Time,
) ([]string, error) {
	return f.beforeIDs, nil
}

func TestReconcileFailsOnEarlyEmptyPageAndPageError(t *testing.T) {
	t.Parallel()

	t.Run("early empty page returns error instead of truncating", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/v1/users/count":
				_, _ = w.Write([]byte(`{"total_count":3}`))
			case "/v1/users":
				assert.Equal(t, "+created_at", r.URL.Query().Get("order_by"))
				if r.URL.Query().Get("offset") == "0" {
					_, _ = w.Write(
						[]byte(
							`[{"id":"user_1","email_addresses":[{"id":"e1","email_address":"a@example.com"}],"primary_email_address_id":"e1","updated_at":1716883200000}]`,
						),
					)
					return
				}
				_, _ = w.Write([]byte(`[]`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer srv.Close()

		p := newTestProvider(t)
		p.baseURL = srv.URL + "/v1"

		_, err := p.Reconcile(t.Context(), []byte(`{"api_key":"sk_test_abc"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty page early")
	})

	t.Run("page error aborts reconciliation with zero commands", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/v1/users/count":
				_, _ = w.Write([]byte(`{"total_count":2}`))
			case "/v1/users":
				if r.URL.Query().Get("offset") == "0" {
					_, _ = w.Write(
						[]byte(
							`[{"id":"user_1","email_addresses":[{"id":"e1","email_address":"a@example.com"}],"primary_email_address_id":"e1","updated_at":1716883200000}]`,
						),
					)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors":[{"message":"boom"}]}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer srv.Close()

		p := newTestProvider(t)
		p.baseURL = srv.URL + "/v1"

		res, err := p.Reconcile(t.Context(), []byte(`{"api_key":"sk_test_abc"}`))
		require.Error(t, err)
		assert.Empty(t, res.Upserts)
	})
}

func TestPlanReconcileDeletionsSafetyGuardsAnd404Confirmation(t *testing.T) {
	t.Parallel()

	t.Run("only explicit 404 from Clerk confirms deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/v1/users/user_deleted":
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errors":[{"code":"resource_not_found","message":"not found"}]}`))
			case "/v1/users/user_still_there":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"user_still_there"}`))
			case "/v1/users/user_server_err":
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors":[{"message":"upstream error"}]}`))
			case "/v1/users/user_rate_limited":
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"errors":[{"message":"rate limited"}]}`))
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		defer srv.Close()

		p := newTestProvider(t)
		p.baseURL = srv.URL + "/v1"
		p.productUserRepo = &fakeReconcileRepo{
			stampedCount: 5,
			beforeIDs: []string{
				"user_seen",
				"user_deleted",
				"user_still_there",
				"user_server_err",
				"user_rate_limited",
			},
		}

		plan, err := p.PlanReconcileDeletions(
			t.Context(),
			"prod_1",
			[]byte(`{"api_key":"sk_test_abc"}`),
			ReconcileResult{
				SeenIDs:       map[string]struct{}{"user_seen": {}},
				SnapshotStart: time.Now().UTC(),
			},
		)
		require.NoError(t, err)
		assert.Empty(t, plan.AbortedReason)
		assert.Equal(t, 4, plan.CandidateCount)
		require.Len(t, plan.Deletes, 1)
		delData, ok := plan.Deletes[0].Data.(DeleteUserData)
		require.True(t, ok)
		assert.Equal(t, "user_deleted", delData.ExternalID)
		assert.NotNil(t, delData.EventAt)
	})

	t.Run("empty seen list while stamped users exist aborts all deletions", func(t *testing.T) {
		t.Parallel()

		p := newTestProvider(t)
		p.productUserRepo = &fakeReconcileRepo{
			stampedCount: 3,
			beforeIDs:    []string{"user_1", "user_2", "user_3"},
		}

		plan, err := p.PlanReconcileDeletions(
			t.Context(),
			"prod_1",
			[]byte(`{"api_key":"sk_test_abc"}`),
			ReconcileResult{
				SeenIDs:       map[string]struct{}{},
				SnapshotStart: time.Now().UTC(),
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "empty_clerk_user_list", plan.AbortedReason)
		assert.True(t, plan.AbortRun)
		assert.Empty(t, plan.Deletes)
	})

	t.Run("zero overlap with existing stamped users aborts the entire run", func(t *testing.T) {
		t.Parallel()

		p := newTestProvider(t)
		p.productUserRepo = &fakeReconcileRepo{
			stampedCount: 3, // small product below the 25-user absolute floor
			beforeIDs:    []string{"user_prod_1", "user_prod_2", "user_prod_3"},
		}

		plan, err := p.PlanReconcileDeletions(
			t.Context(),
			"prod_1",
			[]byte(`{"api_key":"sk_test_wrong_app"}`),
			ReconcileResult{
				SeenIDs: map[string]struct{}{
					"user_dev_1": {},
					"user_dev_2": {},
					"user_dev_3": {},
				},
				SnapshotStart: time.Now().UTC(),
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "no_overlap_with_existing_users", plan.AbortedReason)
		assert.True(t, plan.AbortRun)
		assert.Equal(t, 3, plan.CandidateCount)
		assert.Empty(t, plan.Deletes)
	})

	t.Run("candidate count above volume cap aborts deletions only", func(t *testing.T) {
		t.Parallel()

		beforeIDs := make([]string, 0, 27)
		beforeIDs = append(beforeIDs, "user_seen")
		for i := range 26 {
			beforeIDs = append(beforeIDs, fmt.Sprintf("user_gone_%d", i))
		}

		p := newTestProvider(t)
		p.productUserRepo = &fakeReconcileRepo{
			stampedCount: 50, // cap is max(25, 50/10) = 25; 26 candidates > 25
			beforeIDs:    beforeIDs,
		}

		plan, err := p.PlanReconcileDeletions(
			t.Context(),
			"prod_1",
			[]byte(`{"api_key":"sk_test_abc"}`),
			ReconcileResult{
				SeenIDs:       map[string]struct{}{"user_seen": {}},
				SnapshotStart: time.Now().UTC(),
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "deletion_volume_cap_exceeded", plan.AbortedReason)
		assert.False(t, plan.AbortRun)
		assert.Equal(t, 26, plan.CandidateCount)
		assert.Empty(t, plan.Deletes)
	})

	t.Run("per-run confirmations are capped at 50 and remainder deferred", func(t *testing.T) {
		t.Parallel()

		confirmCalls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			confirmCalls++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"code":"resource_not_found","message":"not found"}]}`))
		}))
		defer srv.Close()

		beforeIDs := make([]string, 0, 61)
		beforeIDs = append(beforeIDs, "user_seen")
		for i := range 60 {
			beforeIDs = append(beforeIDs, fmt.Sprintf("user_gone_%d", i))
		}

		p := newTestProvider(t)
		p.baseURL = srv.URL + "/v1"
		p.productUserRepo = &fakeReconcileRepo{
			stampedCount: 1000, // cap is 100; 60 candidates allowed, 50 confirmed per run
			beforeIDs:    beforeIDs,
		}

		plan, err := p.PlanReconcileDeletions(
			t.Context(),
			"prod_1",
			[]byte(`{"api_key":"sk_test_abc"}`),
			ReconcileResult{
				SeenIDs:       map[string]struct{}{"user_seen": {}},
				SnapshotStart: time.Now().UTC(),
			},
		)
		require.NoError(t, err)
		assert.Empty(t, plan.AbortedReason)
		assert.Equal(t, 50, confirmCalls)
		assert.Len(t, plan.Deletes, 50)
		assert.Equal(t, 10, plan.DeferredCount)
	})
}
