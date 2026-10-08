package stripeprototype_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/stripeprototype"
)

const anchorTestKey = "anchor_prd_apikey_test_secret"

func TestAnchorGatewayRejectsRemoteOriginsAndUnsafeIdentifiers(t *testing.T) {
	t.Parallel()
	for _, origin := range []string{
		"https://apidev.tryanchor.dev", "http://192.168.1.5:8080", "file:///tmp/anchor",
		"http://secret@localhost:8080", "http://localhost:8080/v1", "http://localhost:8080?secret=value",
		"http://localhost:8080#fragment", "http://localhost.example.test:8080",
	} {
		t.Run(origin, func(t *testing.T) {
			t.Parallel()
			_, err := stripeprototype.NewAnchorGateway(origin, "product", anchorTestKey)
			require.Error(t, err)
			require.NotContains(t, err.Error(), anchorTestKey)
			require.NotContains(t, err.Error(), "secret@")
		})
	}
	for _, id := range []string{"", " ", "product/other", "product?secret=value", "../product"} {
		_, err := stripeprototype.NewAnchorGateway("http://127.0.0.1:8080", id, anchorTestKey)
		require.Error(t, err)
	}
	for _, origin := range []string{"http://127.0.0.1:8080", "http://localhost:8080", "http://[::1]:8080/"} {
		_, err := stripeprototype.NewAnchorGateway(origin, "product", anchorTestKey)
		require.NoError(t, err)
	}
}

func TestAnchorGatewaySnapshotUsesProductScopeAndPaginatesLicenseIncludes(t *testing.T) {
	t.Parallel()
	var pageRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, anchorTestKey, r.Header.Get("X-Product-Api-Key"))
		assert.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/products/product/licensing/templates":
			assert.Equal(t, http.MethodGet, r.Method)
			_, _ = w.Write(
				[]byte(
					`{"items":[{"id":"free","name":"Free","status":"ACTIVE","values":{"seats":3}},{"id":"old","name":"Old","status":"ARCHIVED","values":{}}]}`,
				),
			)
		case "/v1/products/product/organizations/search":
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "license", r.URL.Query().Get("include"))
			var request struct {
				Pagination struct {
					Limit  int `json:"limit"`
					Offset int `json:"offset"`
				} `json:"pagination"`
			}
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&request)) {
				return
			}
			assert.Equal(t, 100, request.Pagination.Limit)
			items := make([]map[string]any, 0, 100)
			if request.Pagination.Offset == 0 {
				for index := range 100 {
					items = append(items, map[string]any{
						"id": fmt.Sprintf("org%d", index), "name": fmt.Sprintf("Organization %d", index),
						"license": map[string]any{"template_id": "free", "values": map[string]any{"seats": 8}},
					})
				}
			} else {
				assert.Equal(t, 100, request.Pagination.Offset)
				items = append(items, map[string]any{"id": "org100", "name": "No license"})
			}
			pageRequests.Add(1)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"items": items, "total": 101}))
		default:
			t.Errorf("unexpected Anchor route: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	gateway, err := stripeprototype.NewAnchorGateway(
		server.URL,
		"product",
		anchorTestKey,
		stripeprototype.WithAnchorProductName("Sandbox product"),
	)
	require.NoError(t, err)
	snapshot, err := gateway.Snapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, stripeprototype.Product{ID: "product", Name: "Sandbox product"}, snapshot.Product)
	require.Len(t, snapshot.Templates, 2)
	require.False(t, snapshot.Templates[0].Archived)
	require.True(t, snapshot.Templates[1].Archived)
	require.InDelta(t, 3, snapshot.Templates[0].Values["seats"], 0.001)
	require.Len(t, snapshot.Organizations, 101)
	require.InDelta(t, 8, snapshot.Organizations[0].LicenseValues["seats"], 0.001)
	require.Empty(t, snapshot.Organizations[100].TemplateID)
	require.NotNil(t, snapshot.Organizations[100].LicenseValues)
	require.Equal(t, int32(2), pageRequests.Load())
}

func TestAnchorGatewaySameTemplateKeepsExistingLicenseUntouched(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/products/product/organizations/org/license", r.URL.Path)
		_, _ = w.Write([]byte(`{"template_id":"pro","values":{"seats":25},"adjusted_fields":["seats"]}`))
	}))
	t.Cleanup(server.Close)
	gateway, err := stripeprototype.NewAnchorGateway(server.URL, "product", anchorTestKey)
	require.NoError(t, err)
	require.NoError(t, gateway.ApplyTemplate(context.Background(), "org", "pro"))
	require.Equal(t, int32(1), requests.Load())
}

func TestAnchorGatewayMigrationCarriesAdjustmentsAndInspectsReceipt(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name          string
		licenseStatus int
		receipt       string
		wantError     bool
	}{
		{"tier change", 200, `{"template_id":"pro","failed":0,"results":[{"organization_id":"org","outcome":"CHANGED"}]}`, false},
		{"first license", 404, `{"template_id":"pro","failed":0,"results":[{"organization_id":"org","outcome":"CHANGED"}]}`, false},
		{"retry unchanged", 200, `{"template_id":"pro","failed":0,"results":[{"organization_id":"org","outcome":"UNCHANGED"}]}`, false},
		{"failed item", 200, `{"template_id":"pro","failed":1,"results":[{"organization_id":"org","outcome":"FAILED","error":{"message":"anchor_prd_apikey_test_secret"}}]}`, true},
		{"other organization", 200, `{"template_id":"pro","failed":0,"results":[{"organization_id":"someone-else","outcome":"CHANGED"}]}`, true},
		{"missing receipt", 200, `{"template_id":"pro","failed":0,"results":[]}`, true},
		{"wrong template", 200, `{"template_id":"free","failed":0,"results":[{"organization_id":"org","outcome":"CHANGED"}]}`, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var migrationRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, anchorTestKey, r.Header.Get("X-Product-Api-Key"))
				if r.Method == http.MethodGet {
					assert.Equal(t, "/v1/products/product/organizations/org/license", r.URL.Path)
					w.WriteHeader(testCase.licenseStatus)
					_, _ = w.Write([]byte(`{"template_id":"free","values":{"seats":25}}`))
					return
				}
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/v1/products/product/licensing/organization-licenses/migrate", r.URL.Path)
				var request map[string]any
				if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&request)) {
					return
				}
				assert.Equal(t, "pro", request["template_id"])
				assert.Equal(t, []any{"org"}, request["organization_ids"])
				assert.Equal(t, "CARRY_FORWARD", request["on_difference"])
				migrationRequests.Add(1)
				_, _ = w.Write([]byte(testCase.receipt))
			}))
			t.Cleanup(server.Close)
			gateway, err := stripeprototype.NewAnchorGateway(server.URL, "product", anchorTestKey)
			require.NoError(t, err)
			err = gateway.ApplyTemplate(context.Background(), "org", "pro")
			if testCase.wantError {
				require.Error(t, err)
				require.NotContains(t, err.Error(), anchorTestKey)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, int32(1), migrationRequests.Load())
		})
	}
}

func TestAnchorGatewayDoesNotFollowCredentialLeakingRedirect(t *testing.T) {
	t.Parallel()
	var followed atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		followed.Store(true)
	}))
	t.Cleanup(target.Close)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(source.Close)
	gateway, err := stripeprototype.NewAnchorGateway(source.URL, "product", anchorTestKey)
	require.NoError(t, err)
	_, err = gateway.Snapshot(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), anchorTestKey)
	require.False(t, followed.Load())
}

func TestAnchorGatewayScrubsUpstreamErrorsAndRefusesUnsafeMutation(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"anchor_prd_apikey_test_secret"}`))
	}))
	t.Cleanup(server.Close)
	gateway, err := stripeprototype.NewAnchorGateway(server.URL, "product", anchorTestKey)
	require.NoError(t, err)
	_, err = gateway.Snapshot(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), anchorTestKey)
	require.Error(t, gateway.ApplyTemplate(context.Background(), "org/other", "pro"))
	require.Equal(t, int32(1), requests.Load())
}
