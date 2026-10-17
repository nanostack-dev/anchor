package ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
	domainintegration "anchor/internal/domain/integration"
)

// seedAuditLogs writes one SMTP integration instance and the audit entries the
// filters below tell apart.
func seedAuditLogs(t *testing.T) (emailTestCtx, string, map[string]string) {
	t.Helper()
	tc := newEmailTestCtx(t)
	instance := domainintegration.Instance{
		PlatformTenantID: tc.tenantID,
		ProductID:        tc.product.ProductID,
		ProviderType:     domainintegration.ProviderTypeSMTP,
		ConfigJSON:       []byte(`{}`),
		ConfigVersion:    1,
		IsEnabled:        true,
		Status:           domainintegration.StatusActive,
	}
	instance.GenerateID()
	created, err := IntegrationRepo.Create(t.Context(), instance)
	require.NoError(t, err)

	base := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	rows := []struct {
		key      string
		action   string
		severity string
		at       time.Time
	}{
		{"sync_info_early", "SYNC", "INFO", base},
		{"sync_error_mid", "SYNC", "ERROR", base.Add(time.Hour)},
		{"webhook_info_late", "WEBHOOK_RECEIVED", "INFO", base.Add(2 * time.Hour)},
	}
	idByKey := map[string]string{}
	for _, row := range rows {
		id := ids.MustNew("ial")
		idByKey[row.key] = id
		execSQL(t, `INSERT INTO integration_audit_logs
			(id, integration_instance_id, action, entity_type, severity, message, created_at)
			VALUES ($1, $2, $3, 'user', $4, 'message', $5)`,
			id, created.ID, row.action, row.severity, row.at)
	}

	return tc, created.ID, idByKey
}

func TestSearchIntegrationAuditLogs(t *testing.T) {
	t.Parallel()

	search := func(t *testing.T, tc emailTestCtx, instanceID string, body ct.SearchIntegrationAuditLogsJSONRequestBody) *ct.SearchIntegrationAuditLogsResponse {
		t.Helper()
		resp, err := tc.product.OwnerAuthenticatedClient().SearchIntegrationAuditLogsWithResponse(
			t.Context(), tc.product.ProductID, instanceID, body,
		)
		require.NoError(t, err)
		return resp
	}
	idsOf := func(resp *ct.SearchIntegrationAuditLogsResponse) []string {
		require.NotNil(t, resp.JSON200, string(resp.Body))
		out := make([]string, 0, len(resp.JSON200.Items))
		for _, item := range resp.JSON200.Items {
			out = append(out, item.Id)
		}
		return out
	}

	t.Run("The API lists the newest entry first without a sort", func(t *testing.T) {
		t.Parallel()
		tc, instanceID, entries := seedAuditLogs(t)

		resp := search(t, tc, instanceID, ct.SearchIntegrationAuditLogsJSONRequestBody{})

		assert.Equal(
			t,
			[]string{entries["webhook_info_late"], entries["sync_error_mid"], entries["sync_info_early"]},
			idsOf(resp),
		)
		assert.EqualValues(t, 3, resp.JSON200.Total)
	})

	t.Run("The API filters by action", func(t *testing.T) {
		t.Parallel()
		tc, instanceID, entries := seedAuditLogs(t)

		resp := search(t, tc, instanceID, ct.SearchIntegrationAuditLogsJSONRequestBody{
			Filter: &ct.IntegrationAuditLogFilter{Actions: []string{"WEBHOOK_RECEIVED"}},
		})

		assert.Equal(t, []string{entries["webhook_info_late"]}, idsOf(resp))
	})

	t.Run("The API filters by severity", func(t *testing.T) {
		t.Parallel()
		tc, instanceID, entries := seedAuditLogs(t)

		resp := search(t, tc, instanceID, ct.SearchIntegrationAuditLogsJSONRequestBody{
			Filter: &ct.IntegrationAuditLogFilter{Severities: []ct.IntegrationAuditLogSeverity{"ERROR"}},
		})

		assert.Equal(t, []string{entries["sync_error_mid"]}, idsOf(resp))
	})

	t.Run("The API filters by date range, the start included and the end excluded", func(t *testing.T) {
		t.Parallel()
		tc, instanceID, entries := seedAuditLogs(t)
		start := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

		resp := search(t, tc, instanceID, ct.SearchIntegrationAuditLogsJSONRequestBody{
			Filter: &ct.IntegrationAuditLogFilter{
				CreatedAfter:  new(start),
				CreatedBefore: new(start.Add(2 * time.Hour)),
			},
		})

		assert.Equal(t, []string{entries["sync_error_mid"], entries["sync_info_early"]}, idsOf(resp))
	})

	t.Run("An instance of another product is not found", func(t *testing.T) {
		t.Parallel()
		_, instanceID, _ := seedAuditLogs(t)
		other := newEmailTestCtx(t)

		resp := search(t, other, instanceID, ct.SearchIntegrationAuditLogsJSONRequestBody{})

		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

func TestSearchUserOrganizations(t *testing.T) {
	t.Parallel()

	type fixture struct {
		product   *itdsl.ProductContext
		userID    string
		orgs      []string
		adminRole string
		viewer    string
	}
	build := func(t *testing.T) fixture {
		t.Helper()
		product := createTestProductContext(t)
		product.CreateDefaultProductResourcePermissions(t)
		member := createDSLProductUser(t, product)
		admin := createDSLProductRoleWithPermissions(
			t,
			product,
			"admin-"+ids.MustNew("ct"),
			nil,
			[]string{"file:read", "file:delete"},
		)
		viewer := createDSLProductRole(t, product, "viewer-"+ids.MustNew("ct"), nil)

		f := fixture{product: product, userID: member.ID, adminRole: admin.ID, viewer: viewer.ID}
		for i, roleID := range []string{admin.ID, viewer.ID, admin.ID} {
			org := product.CreateOrganization(t, "org-"+ids.MustNew("ct"), nil)
			createDSLMembership(t, product, member.ID, org.Id, roleID)
			f.orgs = append(f.orgs, org.Id)
			// One membership per second, so the join order is not a tie.
			execSQL(
				t,
				"UPDATE organization_memberships SET created_at = $1 WHERE organization_id = $2 AND product_user_id = $3",
				tiedAt().Add(time.Duration(i)*time.Second),
				org.Id,
				member.ID,
			)
		}
		return f
	}
	search := func(t *testing.T, f fixture, params *ct.SearchUserOrganizationsParams, body ct.SearchUserOrganizationsJSONRequestBody) *ct.SearchUserOrganizationsResponse {
		t.Helper()
		resp, err := f.product.AllScopeAPIKeyClient().SearchUserOrganizationsWithResponse(
			t.Context(), f.product.ProductID, f.userID, params, body,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)
		return resp
	}
	orgIDs := func(resp *ct.SearchUserOrganizationsResponse) []string {
		out := make([]string, 0, len(resp.JSON200.Items))
		for _, item := range resp.JSON200.Items {
			out = append(out, item.Organization.Id)
		}
		return out
	}

	t.Run("The API lists the earliest joined organization first", func(t *testing.T) {
		t.Parallel()
		f := build(t)

		resp := search(t, f, nil, ct.SearchUserOrganizationsJSONRequestBody{})

		assert.Equal(t, f.orgs, orgIDs(resp))
		assert.EqualValues(t, 3, resp.JSON200.Total)
		for _, item := range resp.JSON200.Items {
			assert.Nil(t, item.Role.Permissions, "permissions are absent unless asked for")
		}
	})

	t.Run("The API filters by role", func(t *testing.T) {
		t.Parallel()
		f := build(t)

		resp := search(t, f, nil, ct.SearchUserOrganizationsJSONRequestBody{
			Filter: &ct.UserOrganizationFilter{RoleIds: []string{f.viewer}},
		})

		assert.Equal(t, []string{f.orgs[1]}, orgIDs(resp))
	})

	t.Run("The API filters by organization", func(t *testing.T) {
		t.Parallel()
		f := build(t)

		resp := search(t, f, nil, ct.SearchUserOrganizationsJSONRequestBody{
			Filter: &ct.UserOrganizationFilter{OrganizationIds: []string{f.orgs[2], f.orgs[0]}},
		})

		assert.Equal(t, []string{f.orgs[0], f.orgs[2]}, orgIDs(resp))
	})

	t.Run("The API reads the role permissions of every organization in the page on request", func(t *testing.T) {
		t.Parallel()
		f := build(t)
		include := []ct.UserOrganizationInclude{ct.UserOrganizationIncludeRolePermissions}

		resp := search(
			t,
			f,
			&ct.SearchUserOrganizationsParams{Include: &include},
			ct.SearchUserOrganizationsJSONRequestBody{
				Pagination: &ct.PaginationRequest{Limit: new(int32(2))},
			},
		)

		require.Len(t, resp.JSON200.Items, 2)
		require.NotNil(t, resp.JSON200.Items[0].Role.Permissions)
		assert.ElementsMatch(t, []string{"file:read", "file:delete"}, *resp.JSON200.Items[0].Role.Permissions)
		require.NotNil(t, resp.JSON200.Items[1].Role.Permissions)
		assert.Empty(t, *resp.JSON200.Items[1].Role.Permissions, "a role without permissions reads as an empty list")
		require.NotNil(t, resp.JSON200.NextCursor)
	})

	t.Run("An unknown user is not found", func(t *testing.T) {
		t.Parallel()
		f := build(t)
		f.userID = ids.MustNew("pusr")

		resp, err := f.product.AllScopeAPIKeyClient().SearchUserOrganizationsWithResponse(
			t.Context(), f.product.ProductID, f.userID, nil, ct.SearchUserOrganizationsJSONRequestBody{},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("A key without product user:read is refused", func(t *testing.T) {
		t.Parallel()
		f := build(t)
		client, _ := f.product.CreateAPIKeyClientWithScopes([]string{"organization:read"})

		resp, err := client.SearchUserOrganizationsWithResponse(
			t.Context(), f.product.ProductID, f.userID, nil, ct.SearchUserOrganizationsJSONRequestBody{},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode(), string(resp.Body))
	})
}

func TestSearchEmailTemplates(t *testing.T) {
	t.Parallel()

	create := func(t *testing.T, tc emailTestCtx, slug, name string) string {
		t.Helper()
		resp, err := tc.product.OwnerAuthenticatedClient().CreateEmailTemplateWithResponse(
			t.Context(), tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{Slug: slug, Name: name, Subject: "Subject", BodyHtml: "<p>Body</p>"},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
		return resp.JSON201.Id
	}
	search := func(t *testing.T, tc emailTestCtx, body ct.SearchEmailTemplatesJSONRequestBody) []string {
		t.Helper()
		resp, err := tc.product.OwnerAuthenticatedClient().SearchEmailTemplatesWithResponse(
			t.Context(), tc.product.ProductID, body,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)
		out := make([]string, 0, len(resp.JSON200.Items))
		for _, item := range resp.JSON200.Items {
			out = append(out, item.Id)
		}
		return out
	}

	t.Run("The API filters by slug, id and full text, and reports the total of the filter", func(t *testing.T) {
		t.Parallel()
		tc := newEmailTestCtx(t)
		welcome := create(t, tc, "welcome-"+ids.MustNew("ct"), "Welcome mail")
		invoice := create(t, tc, "invoice-"+ids.MustNew("ct"), "Invoice mail")
		reset := create(t, tc, "reset-"+ids.MustNew("ct"), "Password reset")

		assert.ElementsMatch(
			t,
			[]string{welcome, invoice, reset},
			search(t, tc, ct.SearchEmailTemplatesJSONRequestBody{}),
		)
		assert.Equal(t, []string{invoice}, search(t, tc, ct.SearchEmailTemplatesJSONRequestBody{
			Filter: &ct.EmailTemplateFilter{Ids: []string{invoice}},
		}))
		assert.ElementsMatch(t, []string{welcome, invoice}, search(t, tc, ct.SearchEmailTemplatesJSONRequestBody{
			FullTextSearch: new("mail"),
		}))
		assert.Equal(t, []string{reset}, search(t, tc, ct.SearchEmailTemplatesJSONRequestBody{
			FullTextSearch: new("reset-"),
		}))
	})

	t.Run("The API filters by active state", func(t *testing.T) {
		t.Parallel()
		tc := newEmailTestCtx(t)
		active := create(t, tc, uniqueSlug(), "Active")
		inactive := create(t, tc, uniqueSlug(), "Inactive")
		execSQL(t, "UPDATE email_templates SET is_active = FALSE WHERE id = $1", inactive)

		assert.Equal(t, []string{active}, search(t, tc, ct.SearchEmailTemplatesJSONRequestBody{
			Filter: &ct.EmailTemplateFilter{IsActive: new(true)},
		}))
		assert.Equal(t, []string{inactive}, search(t, tc, ct.SearchEmailTemplatesJSONRequestBody{
			Filter: &ct.EmailTemplateFilter{IsActive: new(false)},
		}))
	})
}

func TestSearchEmailSends(t *testing.T) {
	t.Parallel()

	tc := newEmailTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()
	templateResp, templateErr := client.CreateEmailTemplateWithResponse(
		t.Context(),
		tc.product.ProductID,
		ct.CreateEmailTemplateJSONRequestBody{
			Slug:     uniqueSlug(),
			Name:     "Template",
			Subject:  "Subject",
			BodyHtml: "<p>Body</p>",
		},
	)
	require.NoError(t, templateErr)
	require.NotNil(t, templateResp.JSON201)
	templateID := templateResp.JSON201.Id

	insert := func(status string, template *string, at time.Time) string {
		id := ids.MustNew("esnd")
		execSQL(t, `INSERT INTO email_send_records
			(id, platform_tenant_id, product_id, template_id, to_address, from_address, subject, body_html, status, created_at)
			VALUES ($1, $2, $3, $4, $5, 'noreply@tryanchor.dev', 'Subject', '<p>Body</p>', $6, $7)`,
			id, tc.tenantID, tc.product.ProductID, template, uniqueEmail(), status, at)
		return id
	}
	at := tiedAt()
	sentFromTemplate := insert("SENT", &templateID, at)
	failedFromTemplate := insert("FAILED", &templateID, at.Add(time.Second))
	sentWithoutTemplate := insert("SENT", nil, at.Add(2*time.Second))

	search := func(t *testing.T, filter *ct.EmailSendFilter) []string {
		t.Helper()
		resp, err := client.SearchEmailSendsWithResponse(
			t.Context(), tc.product.ProductID, ct.SearchEmailSendsJSONRequestBody{Filter: filter},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)
		out := make([]string, 0, len(resp.JSON200.Items))
		for _, item := range resp.JSON200.Items {
			out = append(out, item.Id)
		}
		return out
	}

	t.Run("The API lists the newest send first", func(t *testing.T) {
		assert.Equal(t, []string{sentWithoutTemplate, failedFromTemplate, sentFromTemplate}, search(t, nil))
	})

	t.Run("The API filters by template", func(t *testing.T) {
		assert.Equal(t, []string{failedFromTemplate, sentFromTemplate},
			search(t, &ct.EmailSendFilter{TemplateIds: []string{templateID}}))
	})

	t.Run("The API filters by status", func(t *testing.T) {
		assert.Equal(t, []string{sentWithoutTemplate, sentFromTemplate},
			search(t, &ct.EmailSendFilter{Statuses: []ct.EmailSendStatus{"SENT"}}))
	})

	t.Run("The API filters by template and status together", func(t *testing.T) {
		assert.Equal(t, []string{sentFromTemplate},
			search(t, &ct.EmailSendFilter{TemplateIds: []string{templateID}, Statuses: []ct.EmailSendStatus{"SENT"}}))
	})
}

func TestSearchLicenseTemplates(t *testing.T) {
	t.Parallel()

	t.Run("The API filters by status and keeps every template without one", func(t *testing.T) {
		t.Parallel()
		tc := newTemplateCtx(t)
		free := createTemplate(t, tc, "Free", validTemplateValues())
		legacy := createTemplate(t, tc, "Legacy", validTemplateValues())
		pro := createTemplate(t, tc, "Pro", validTemplateValues())
		require.Equal(t, http.StatusOK, archiveTemplate(t, tc, legacy.Id).StatusCode())
		client := tc.product.OwnerAuthenticatedClient()

		search := func(body ct.SearchLicenseTemplatesJSONRequestBody) []string {
			resp, err := client.SearchLicenseTemplatesWithResponse(t.Context(), tc.product.ProductID, body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
			out := make([]string, 0, len(resp.JSON200.Items))
			for _, item := range resp.JSON200.Items {
				out = append(out, item.Id)
			}
			return out
		}

		assert.Equal(
			t,
			[]string{free.Id, legacy.Id, pro.Id},
			search(ct.SearchLicenseTemplatesJSONRequestBody{}),
			"ordered by name",
		)
		assert.Equal(t, []string{free.Id, pro.Id}, search(ct.SearchLicenseTemplatesJSONRequestBody{
			Filter: &ct.LicenseTemplateFilter{Statuses: []ct.LicenseTemplateStatus{ct.LicenseTemplateStatusACTIVE}},
		}))
		assert.Equal(t, []string{legacy.Id}, search(ct.SearchLicenseTemplatesJSONRequestBody{
			Filter: &ct.LicenseTemplateFilter{Statuses: []ct.LicenseTemplateStatus{ct.LicenseTemplateStatusARCHIVED}},
		}))
		assert.Equal(t, []string{pro.Id}, search(ct.SearchLicenseTemplatesJSONRequestBody{FullTextSearch: new("Pr")}))
	})

	t.Run("Full text search matches the description as well as the name", func(t *testing.T) {
		t.Parallel()
		tc := newTemplateCtx(t)
		client := tc.product.OwnerAuthenticatedClient()
		create := func(name, description string) string {
			resp, err := client.CreateLicenseTemplateWithResponse(t.Context(), tc.product.ProductID,
				ct.CreateLicenseTemplateJSONRequestBody{
					Name: name, Description: new(description), Values: validTemplateValues(),
				})
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
			return resp.JSON201.Id
		}
		byName := create("Zephyr", "plain")
		byDescription := create("Quartz", "Includes Zephyr-grade support")
		create("Granite", "nothing relevant")

		resp, err := client.SearchLicenseTemplatesWithResponse(t.Context(), tc.product.ProductID,
			ct.SearchLicenseTemplatesJSONRequestBody{FullTextSearch: new("Zephyr")})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		got := make([]string, 0, len(resp.JSON200.Items))
		for _, item := range resp.JSON200.Items {
			got = append(got, item.Id)
		}
		assert.ElementsMatch(t, []string{byName, byDescription}, got)

		onlyDescription, err := client.SearchLicenseTemplatesWithResponse(t.Context(), tc.product.ProductID,
			ct.SearchLicenseTemplatesJSONRequestBody{FullTextSearch: new("grade support")})
		require.NoError(t, err)
		require.Len(t, onlyDescription.JSON200.Items, 1)
		assert.Equal(t, byDescription, onlyDescription.JSON200.Items[0].Id, "a description-only match is found")

		mixedCase, err := client.SearchLicenseTemplatesWithResponse(t.Context(), tc.product.ProductID,
			ct.SearchLicenseTemplatesJSONRequestBody{FullTextSearch: new("zEPHYR")})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, mixedCase.StatusCode(), string(mixedCase.Body))
		assert.Len(t, mixedCase.JSON200.Items, 2, "matching ignores case, as the table did on the client")
	})

	t.Run("Full text search matches like metacharacters literally", func(t *testing.T) {
		t.Parallel()
		tc := newTemplateCtx(t)
		client := tc.product.OwnerAuthenticatedClient()
		create := func(name string) string {
			resp, err := client.CreateLicenseTemplateWithResponse(t.Context(), tc.product.ProductID,
				ct.CreateLicenseTemplateJSONRequestBody{Name: name, Values: validTemplateValues()})
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
			return resp.JSON201.Id
		}
		percent := create("Discount 100% off")
		underscore := create("seat_cap")
		create("Seatxcap")
		create("Plain")

		for term, want := range map[string][]string{
			"%":        {percent},
			"100%":     {percent},
			"seat_cap": {underscore},
			`\`:        {},
		} {
			resp, err := client.SearchLicenseTemplatesWithResponse(t.Context(), tc.product.ProductID,
				ct.SearchLicenseTemplatesJSONRequestBody{FullTextSearch: new(term)})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
			got := make([]string, 0, len(resp.JSON200.Items))
			for _, item := range resp.JSON200.Items {
				got = append(got, item.Id)
			}
			assert.ElementsMatch(t, want, got, "term %q", term)
		}
	})

	t.Run("A key without license template:read is refused", func(t *testing.T) {
		t.Parallel()
		tc := newTemplateCtx(t)
		client, _ := tc.product.CreateAPIKeyClientWithScopes([]string{"license_template:create"})

		resp, err := client.SearchLicenseTemplatesWithResponse(
			t.Context(), tc.product.ProductID, ct.SearchLicenseTemplatesJSONRequestBody{},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode(), string(resp.Body))
	})
}

func TestSearchOrganizationLicenseHistory(t *testing.T) {
	t.Parallel()

	search := func(t *testing.T, w *licenseWorld, organizationID string, body ct.SearchOrganizationLicenseHistoryJSONRequestBody) *ct.SearchOrganizationLicenseHistoryResponse {
		t.Helper()
		resp, err := w.client().SearchOrganizationLicenseHistoryWithResponse(
			t.Context(), w.productID(), organizationID, body,
		)
		require.NoError(t, err)
		return resp
	}

	t.Run("The API filters by change type and returns the newest entry first", func(t *testing.T) {
		t.Parallel()
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 1, "sso": false})

		all := search(t, w, w.OrganizationID(), ct.SearchOrganizationLicenseHistoryJSONRequestBody{})
		require.Equal(t, http.StatusOK, all.StatusCode(), string(all.Body))
		require.Len(t, all.JSON200.Items, 3)
		assert.Equal(t, ct.INSTANTIATED, all.JSON200.Items[2].Type, "the oldest entry comes last")

		adjusted := search(t, w, w.OrganizationID(), ct.SearchOrganizationLicenseHistoryJSONRequestBody{
			Filter: &ct.OrganizationLicenseHistoryFilter{Types: []ct.LicenseChangeType{ct.ADJUSTED}},
		})
		require.Equal(t, http.StatusOK, adjusted.StatusCode(), string(adjusted.Body))
		assert.Len(t, adjusted.JSON200.Items, 2)
		assert.EqualValues(t, 2, adjusted.JSON200.Total)
	})

	t.Run("The API filters by date range", func(t *testing.T) {
		t.Parallel()
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 1})
		early := tiedAt()
		late := tiedAt().Add(time.Hour)
		execSQL(
			t,
			"UPDATE organization_license_changes SET changed_at = $1 WHERE organization_id = $2 AND change_type = 'INSTANTIATED'",
			early,
			w.OrganizationID(),
		)
		execSQL(
			t,
			"UPDATE organization_license_changes SET changed_at = $1 WHERE organization_id = $2 AND change_type = 'ADJUSTED'",
			late,
			w.OrganizationID(),
		)

		resp := search(t, w, w.OrganizationID(), ct.SearchOrganizationLicenseHistoryJSONRequestBody{
			Filter: &ct.OrganizationLicenseHistoryFilter{ChangedAfter: new(early.Add(time.Minute))},
		})

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.Len(t, resp.JSON200.Items, 1)
		assert.Equal(t, ct.ADJUSTED, resp.JSON200.Items[0].Type)
	})

	t.Run("An organization never licensed has an empty history, and an unknown one is not found", func(t *testing.T) {
		t.Parallel()
		w := newLicenseWorld(t)

		empty := search(t, w, w.OrganizationID(), ct.SearchOrganizationLicenseHistoryJSONRequestBody{})
		require.Equal(t, http.StatusOK, empty.StatusCode(), string(empty.Body))
		assert.Empty(t, empty.JSON200.Items)

		unknown := search(t, w, ids.MustNew("org"), ct.SearchOrganizationLicenseHistoryJSONRequestBody{})
		assert.Equal(t, http.StatusNotFound, unknown.StatusCode(), string(unknown.Body))
	})
}
