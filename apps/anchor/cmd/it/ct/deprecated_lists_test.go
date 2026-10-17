package ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The GET lists below are deprecated in favour of a cursor-paginated search.
// They keep answering until their callers are gone, and these tests keep them
// honest. Every call is staticcheck-flagged on purpose.

//nolint:staticcheck // the deprecated route stays covered until it is removed
func TestDeprecatedListIntegrationAuditLogsStillListsNewestFirst(t *testing.T) {
	t.Parallel()

	t.Run("Deprecated list integration audit logs still lists newest first", func(t *testing.T) {
		t.Parallel()

		tc, instanceID, entries := seedAuditLogs(t)

		resp, err := tc.product.OwnerAuthenticatedClient().
			ListIntegrationAuditLogsWithResponse(
				t.Context(), tc.product.ProductID, instanceID,
			)

		require.NoError(t, err)
		require.NotNil(t, resp.JSON200, string(resp.Body))
		got := make([]string, 0, len(resp.JSON200.Items))
		for _, item := range resp.JSON200.Items {
			got = append(got, item.Id)
		}
		assert.Equal(
			t,
			[]string{entries["webhook_info_late"], entries["sync_error_mid"], entries["sync_info_early"]},
			got,
		)
		assert.Equal(t, 3, resp.JSON200.Count)
	})
}

//nolint:staticcheck // the deprecated route stays covered until it is removed
func TestDeprecatedListEmailTemplatesKeepsCountEqualToTotal(t *testing.T) {
	t.Parallel()

	t.Run("Deprecated list email templates keeps count equal to total", func(t *testing.T) {
		t.Parallel()

		tc := newEmailTestCtx(t)
		client := tc.product.OwnerAuthenticatedClient()
		for range 3 {
			created, err := client.CreateEmailTemplateWithResponse(
				t.Context(),
				tc.product.ProductID,
				ct.CreateEmailTemplateJSONRequestBody{
					Slug:     uniqueSlug(),
					Name:     "Template",
					Subject:  "S",
					BodyHtml: "<p>B</p>",
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))
		}

		resp, err := client.ListEmailTemplatesWithResponse(
			t.Context(), tc.product.ProductID, &ct.ListEmailTemplatesParams{Limit: new(int64(2))},
		)

		require.NoError(t, err)
		require.NotNil(t, resp.JSON200, string(resp.Body))
		assert.Len(t, resp.JSON200.Items, 2)
		assert.Equal(t, 3, resp.JSON200.Count, "the deprecated list reports the total as its count")
	})
}

//nolint:staticcheck // the deprecated route stays covered until it is removed
func TestDeprecatedListEmailSendsFiltersThroughItsQuery(t *testing.T) {
	t.Parallel()

	t.Run("Deprecated list email sends filters through its query", func(t *testing.T) {
		t.Parallel()

		tc := newEmailTestCtx(t)
		client := tc.product.OwnerAuthenticatedClient()
		created, err := client.CreateEmailTemplateWithResponse(
			t.Context(),
			tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:     uniqueSlug(),
				Name:     "Template",
				Subject:  "S",
				BodyHtml: "<p>B</p>",
			},
		)
		require.NoError(t, err)
		require.NotNil(t, created.JSON201)
		templateID := created.JSON201.Id

		insert := func(status string, at time.Time) string {
			id := ids.MustNew("esnd")
			execSQL(t, `INSERT INTO email_send_records
			(id, platform_tenant_id, product_id, template_id, to_address, from_address, subject, body_html, status, created_at)
			VALUES ($1, $2, $3, $4, $5, 'noreply@tryanchor.dev', 'S', '<p>B</p>', $6, $7)`,
				id, tc.tenantID, tc.product.ProductID, templateID, uniqueEmail(), status, at)
			return id
		}
		sent := insert("SENT", tiedAt())
		insert("FAILED", tiedAt().Add(time.Second))
		status := ct.EmailSendStatus("SENT")

		resp, err := client.ListEmailSendsWithResponse(
			t.Context(), tc.product.ProductID, &ct.ListEmailSendsParams{TemplateId: &templateID, Status: &status},
		)

		require.NoError(t, err)
		require.NotNil(t, resp.JSON200, string(resp.Body))
		require.Len(t, resp.JSON200.Items, 1)
		assert.Equal(t, sent, resp.JSON200.Items[0].Id)
	})
}

//nolint:staticcheck // the deprecated route stays covered until it is removed
func TestDeprecatedListLicenseTemplatesFiltersByStatus(t *testing.T) {
	t.Parallel()

	t.Run("Deprecated list license templates filters by status", func(t *testing.T) {
		t.Parallel()

		tc := newTemplateCtx(t)
		free := createTemplate(t, tc, "Free", validTemplateValues())
		legacy := createTemplate(t, tc, "Legacy", validTemplateValues())
		require.Equal(t, http.StatusOK, archiveTemplate(t, tc, legacy.Id).StatusCode())
		status := ct.LicenseTemplateStatusACTIVE

		resp, err := tc.product.OwnerAuthenticatedClient().
			ListLicenseTemplatesWithResponse(
				t.Context(), tc.product.ProductID, &ct.ListLicenseTemplatesParams{Status: &status},
			)

		require.NoError(t, err)
		require.NotNil(t, resp.JSON200, string(resp.Body))
		require.Len(t, resp.JSON200.Items, 1)
		assert.Equal(t, free.Id, resp.JSON200.Items[0].Id)
	})
}

//nolint:staticcheck // the deprecated route stays covered until it is removed
func TestDeprecatedGetOrganizationLicenseHistoryPagesByOffset(t *testing.T) {
	t.Parallel()

	t.Run("Deprecated get organization license history pages by offset", func(t *testing.T) {
		t.Parallel()

		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 1, "sso": false})

		resp, err := w.client().
			GetOrganizationLicenseHistoryWithResponse(
				t.Context(), w.productID(), w.OrganizationID(),
				&ct.GetOrganizationLicenseHistoryParams{Limit: new(int32(2)), Offset: new(int32(1))},
			)

		require.NoError(t, err)
		require.NotNil(t, resp.JSON200, string(resp.Body))
		assert.Len(t, resp.JSON200.Items, 2)
		assert.EqualValues(t, 3, resp.JSON200.Total)
		assert.Nil(t, resp.JSON200.NextCursor, "the third and last entry fills no further page")
	})
}

//nolint:staticcheck // the deprecated route stays covered until it is removed
func TestDeprecatedListUserOrganizationsReturnsEveryMembership(t *testing.T) {
	t.Parallel()

	t.Run("Deprecated list user organizations returns every membership", func(t *testing.T) {
		t.Parallel()

		product := createTestProductContext(t)
		member := createDSLProductUser(t, product)
		role := createDSLProductRole(t, product, "role-"+ids.MustNew("ct"), nil)
		var organizations []string
		for i := range 3 {
			organization := product.CreateOrganization(t, "org-"+ids.MustNew("ct"), nil)
			createDSLMembership(t, product, member.ID, organization.Id, role.ID)
			execSQL(
				t,
				"UPDATE organization_memberships SET created_at = $1 WHERE organization_id = $2 AND product_user_id = $3",
				tiedAt().Add(time.Duration(i)*time.Second),
				organization.Id,
				member.ID,
			)
			organizations = append(organizations, organization.Id)
		}

		resp, err := product.AllScopeAPIKeyClient().
			ListUserOrganizationsWithResponse(
				t.Context(), product.ProductID, member.ID, nil,
			)

		require.NoError(t, err)
		require.NotNil(t, resp.JSON200, string(resp.Body))
		got := make([]string, 0, len(resp.JSON200.Items))
		for _, item := range resp.JSON200.Items {
			got = append(got, item.Organization.Id)
		}
		assert.Equal(t, organizations, got)
	})
}

// The old lists took any limit. A page larger than one search can hold is
// assembled from consecutive searches, so nothing is cut at the search maximum.
const legacyOversizedRows = 1005

//nolint:staticcheck // the deprecated route stays covered until it is removed
func TestDeprecatedListEmailTemplatesReturnsMoreThanAThousand(t *testing.T) {
	t.Parallel()

	t.Run("Deprecated list email templates returns more than a thousand", func(t *testing.T) {
		t.Parallel()

		tc := newEmailTestCtx(t)
		execSQL(t, `INSERT INTO email_templates (id, platform_tenant_id, product_id, slug, name, created_at)
		SELECT 'etpl_' || $1 || lpad(g::text, 6, '0'), $2, $3, 'slug-' || g, 'Template ' || g, $4::timestamptz + g * interval '1 second'
		FROM generate_series(1, $5::int) AS g`,
			ids.MustNew("ct"), tc.tenantID, tc.product.ProductID, tiedAt(), legacyOversizedRows)
		newestFirst := idsFromSQL(
			t,
			`SELECT id FROM email_templates WHERE product_id = $1 ORDER BY created_at DESC, id DESC`,
			tc.product.ProductID,
		)
		client := tc.product.OwnerAuthenticatedClient()

		whole, err := client.ListEmailTemplatesWithResponse(
			t.Context(), tc.product.ProductID, &ct.ListEmailTemplatesParams{Limit: new(int64(1500))},
		)
		require.NoError(t, err)
		require.NotNil(t, whole.JSON200, string(whole.Body))
		assert.Equal(t, newestFirst, templateIDs(whole.JSON200.Items), "every row once, newest first, none cut at 1000")
		assert.Equal(t, legacyOversizedRows, whole.JSON200.Count)

		tail, err := client.ListEmailTemplatesWithResponse(
			t.Context(),
			tc.product.ProductID,
			&ct.ListEmailTemplatesParams{Limit: new(int64(1200)), Offset: new(int64(10))},
		)
		require.NoError(t, err)
		require.NotNil(t, tail.JSON200, string(tail.Body))
		assert.Equal(t, newestFirst[10:], templateIDs(tail.JSON200.Items), "the offset applies once, the rest follows")
	})
}

//nolint:staticcheck // the deprecated route stays covered until it is removed
func TestDeprecatedListEmailSendsReturnsMoreThanAThousand(t *testing.T) {
	t.Parallel()

	t.Run("Deprecated list email sends returns more than a thousand", func(t *testing.T) {
		t.Parallel()

		tc := newEmailTestCtx(t)
		execSQL(t, `INSERT INTO email_send_records
		(id, platform_tenant_id, product_id, to_address, from_address, subject, body_html, status, created_at)
		SELECT 'esnd_' || $1 || lpad(g::text, 6, '0'), $2, $3, 'to' || g || '@example.com', 'noreply@tryanchor.dev', 'S', '<p>B</p>', 'SENT',
			$4::timestamptz + g * interval '1 second'
		FROM generate_series(1, $5::int) AS g`,
			ids.MustNew("ct"), tc.tenantID, tc.product.ProductID, tiedAt(), legacyOversizedRows)
		newestFirst := idsFromSQL(
			t,
			`SELECT id FROM email_send_records WHERE product_id = $1 ORDER BY created_at DESC, id DESC`,
			tc.product.ProductID,
		)

		resp, err := tc.product.OwnerAuthenticatedClient().ListEmailSendsWithResponse(
			t.Context(), tc.product.ProductID, &ct.ListEmailSendsParams{Limit: new(int64(1100)), Offset: new(int64(3))},
		)

		require.NoError(t, err)
		require.NotNil(t, resp.JSON200, string(resp.Body))
		got := make([]string, 0, len(resp.JSON200.Items))
		for _, item := range resp.JSON200.Items {
			got = append(got, item.Id)
		}
		assert.Equal(t, newestFirst[3:], got)
	})
}

func templateIDs(items []ct.EmailTemplateResponse) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Id)
	}
	return out
}
