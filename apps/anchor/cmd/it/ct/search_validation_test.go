package ct_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// The request body of a search is not checked against the OpenAPI enums, so the
// service input has to refuse a filter value outside the set the contract
// names. Without it the value reaches the query and silently matches nothing.
func TestSearchesRejectFilterValuesOutsideTheirEnums(t *testing.T) {
	// Resource searches refuse filter values outside their declared enums before returning results.
	t.Parallel()

	type attempt struct {
		name string
		body string
		// field is the property the error must name.
		field string
		post  func(ctx context.Context, body io.Reader) (int, []byte, error)
	}

	var attempts []attempt
	add := func(name, body, field string, post func(ctx context.Context, body io.Reader) (int, []byte, error)) {
		attempts = append(attempts, attempt{name: name, body: body, field: field, post: post})
	}

	email := newEmailTestCtx(t)
	emailClient := email.product.OwnerAuthenticatedClient()
	add("email send status", `{"filter":{"statuses":["BOGUS"]}}`, "Statuses",
		func(ctx context.Context, body io.Reader) (int, []byte, error) {
			resp, err := emailClient.SearchEmailSendsWithBodyWithResponse(
				ctx,
				email.product.ProductID,
				"application/json",
				body,
			)
			return httpCall(resp, err, func(r *ct.SearchEmailSendsResponse) []byte { return r.Body })
		})

	audit, auditInstance, _ := seedAuditLogs(t)
	auditClient := audit.product.OwnerAuthenticatedClient()
	auditPost := func(ctx context.Context, body io.Reader) (int, []byte, error) {
		resp, err := auditClient.SearchIntegrationAuditLogsWithBodyWithResponse(
			ctx, audit.product.ProductID, auditInstance, "application/json", body)
		return httpCall(resp, err, func(r *ct.SearchIntegrationAuditLogsResponse) []byte { return r.Body })
	}
	add("audit log severity", `{"filter":{"severities":["BOGUS"]}}`, "Severities", auditPost)
	add("audit log action", `{"filter":{"actions":["NOT_AN_ACTION"]}}`, "filter.actions", auditPost)

	templates := newTemplateCtx(t)
	templatesClient := templates.product.OwnerAuthenticatedClient()
	add("license template status", `{"filter":{"statuses":["BOGUS"]}}`, "Statuses",
		func(ctx context.Context, body io.Reader) (int, []byte, error) {
			resp, err := templatesClient.SearchLicenseTemplatesWithBodyWithResponse(
				ctx,
				templates.product.ProductID,
				"application/json",
				body,
			)
			return httpCall(resp, err, func(r *ct.SearchLicenseTemplatesResponse) []byte { return r.Body })
		})

	world := newLicensedWorld(t)
	add("license history type", `{"filter":{"types":["BOGUS"]}}`, "Types",
		func(ctx context.Context, body io.Reader) (int, []byte, error) {
			resp, err := world.client().SearchOrganizationLicenseHistoryWithBodyWithResponse(
				ctx, world.productID(), world.OrganizationID(), "application/json", body)
			return httpCall(resp, err, func(r *ct.SearchOrganizationLicenseHistoryResponse) []byte { return r.Body })
		})

	tenant := itdsl.Given(t).Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true}).Build().Tenant("t")
	add("platform user role", `{"filter":{"roles":["BOGUS"]}}`, "filter.roles",
		func(ctx context.Context, body io.Reader) (int, []byte, error) {
			resp, err := tenant.OwnerClient.SearchPlatformUsersWithBodyWithResponse(ctx, "application/json", body)
			return httpCall(resp, err, func(r *ct.SearchPlatformUsersResponse) []byte { return r.Body })
		})

	product := createTestProductContext(t)
	productClient := product.AllScopeAPIKeyClient()
	add("product user status", `{"filter":{"statuses":["BOGUS"]}}`, "Statuses",
		func(ctx context.Context, body io.Reader) (int, []byte, error) {
			resp, err := productClient.SearchProductUsersWithBodyWithResponse(
				ctx,
				product.ProductID,
				"application/json",
				body,
			)
			return httpCall(resp, err, func(r *ct.SearchProductUsersResponse) []byte { return r.Body })
		})

	member := createDSLProductUser(t, product)
	add("user organizations has no enum filter but still refuses a sort", `{"sort_by":"bogus"}`, "sort_by",
		func(ctx context.Context, body io.Reader) (int, []byte, error) {
			resp, err := productClient.SearchUserOrganizationsWithBodyWithResponse(
				ctx, product.ProductID, member.ID, nil, "application/json", body)
			return httpCall(resp, err, func(r *ct.SearchUserOrganizationsResponse) []byte { return r.Body })
		})

	for _, tt := range attempts {
		t.Run(tt.name, func(t *testing.T) {
			// The search refuses this filter value outside the declared enum.
			t.Parallel()
			status, raw, err := tt.post(t.Context(), strings.NewReader(tt.body))
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, status, string(raw))
			assert.Contains(t, string(raw), "VALIDATION_ERROR")
			assert.Contains(t, string(raw), tt.field)
		})
	}

	t.Run("a known value still searches", func(t *testing.T) {
		// A known value still searches.
		t.Parallel()
		status, raw, err := attempts[0].post(t.Context(), strings.NewReader(`{"filter":{"statuses":["SENT"]}}`))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status, string(raw))
	})
}
