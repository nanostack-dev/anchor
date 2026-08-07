package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func archiveTemplate(t *testing.T, tc testCtx, templateID string) *ct.ArchiveLicenseTemplateResponse {
	t.Helper()
	resp, err := tc.product.OwnerAuthenticatedClient().ArchiveLicenseTemplateWithResponse(
		context.Background(), tc.product.ProductID, templateID,
	)
	require.NoError(t, err)
	return resp
}

func listTemplates(
	t *testing.T, tc testCtx, params *ct.ListLicenseTemplatesParams,
) ct.LicenseTemplateListResponse {
	t.Helper()
	resp, err := tc.product.OwnerAuthenticatedClient().ListLicenseTemplatesWithResponse(
		context.Background(), tc.product.ProductID, params,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON200)
	return *resp.JSON200
}

func TestLicenseTemplateArchive(t *testing.T) {
	t.Run("withdraws the tier and keeps the record", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())
		require.Equal(t, ct.LicenseTemplateStatusACTIVE, created.Status)

		require.Equal(t, http.StatusNoContent, archiveTemplate(t, tc, created.Id).StatusCode())

		// Still readable by identifier. An organization's license names this
		// template as the statement of what it was sold, so the record has to
		// outlive the offer.
		read, err := tc.product.OwnerAuthenticatedClient().GetLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, created.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, read.StatusCode(), string(read.Body))
		require.NotNil(t, read.JSON200)
		assert.Equal(t, ct.LicenseTemplateStatusARCHIVED, read.JSON200.Status)
		assertValues(t, read.JSON200.Values, validTemplateValues())
	})

	t.Run("leaves the listing", func(t *testing.T) {
		tc := newTemplateCtx(t)
		kept := createTemplate(t, tc, "Free", templateValuesWith("flows", 10))
		withdrawn := createTemplate(t, tc, "Pro", validTemplateValues())

		require.Equal(t, http.StatusNoContent, archiveTemplate(t, tc, withdrawn.Id).StatusCode())

		assert.Equal(t, []string{kept.Name}, templateNames(listTemplates(t, tc, nil).Items))
	})

	t.Run("lists on request", func(t *testing.T) {
		tc := newTemplateCtx(t)
		createTemplate(t, tc, "Free", templateValuesWith("flows", 10))
		withdrawn := createTemplate(t, tc, "Pro", validTemplateValues())

		require.Equal(t, http.StatusNoContent, archiveTemplate(t, tc, withdrawn.Id).StatusCode())

		listed := listTemplates(t, tc, &ct.ListLicenseTemplatesParams{IncludeArchived: new(true)})
		assert.Equal(t, []string{"Free", "Pro"}, templateNames(listed.Items))
	})

	t.Run("frees the name for a replacement", func(t *testing.T) {
		tc := newTemplateCtx(t)
		withdrawn := createTemplate(t, tc, "Pro", validTemplateValues())

		require.Equal(t, http.StatusNoContent, archiveTemplate(t, tc, withdrawn.Id).StatusCode())

		// A withdrawn tier must not block its own replacement, which is why the
		// name is unique among active templates only.
		replacement := createTemplate(t, tc, "Pro", templateValuesWith("flows", 900))
		assert.NotEqual(t, withdrawn.Id, replacement.Id)
	})

	t.Run("is idempotent", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		require.Equal(t, http.StatusNoContent, archiveTemplate(t, tc, created.Id).StatusCode())
		assert.Equal(t, http.StatusNoContent, archiveTemplate(t, tc, created.Id).StatusCode())
	})

	t.Run("refuses to edit a withdrawn tier", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())
		require.Equal(t, http.StatusNoContent, archiveTemplate(t, tc, created.Id).StatusCode())

		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			created.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Name: new("Enterprise")},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertAPIError(t, resp.JSON400.Errors, "LICENSE_TEMPLATE_ARCHIVED")
	})

	t.Run("404 when the product has no template with that identifier", func(t *testing.T) {
		tc := newTemplateCtx(t)

		assert.Equal(t, http.StatusNotFound, archiveTemplate(t, tc, missingTemplateID()).StatusCode())
	})
}
