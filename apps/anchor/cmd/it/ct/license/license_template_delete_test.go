package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deleteTemplate(t *testing.T, tc testCtx, templateID string) *ct.DeleteLicenseTemplateResponse {
	t.Helper()
	resp, err := tc.product.OwnerAuthenticatedClient().DeleteLicenseTemplateWithResponse(
		context.Background(), tc.product.ProductID, templateID,
	)
	require.NoError(t, err)
	return resp
}

// TestLicenseTemplateDelete covers the template ADR-0010 left unable to be
// removed: one no Organization license has ever named. See
// docs/adr/0011-unreferenced-license-template-can-be-deleted.md.
func TestLicenseTemplateDelete(t *testing.T) {
	t.Run("removes a template no organization license names", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		resp := deleteTemplate(t, tc, created.Id)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

		// Actually gone, not archived: the row is removed, unlike withdrawal.
		read, err := tc.product.OwnerAuthenticatedClient().GetLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, created.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, read.StatusCode(), string(read.Body))
	})

	t.Run("frees the name for a replacement", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, "Pro", validTemplateValues())

		require.Equal(t, http.StatusNoContent, deleteTemplate(t, tc, created.Id).StatusCode())

		replacement := createTemplate(t, tc, "Pro", templateValuesWith("flows", 900))
		assert.NotEqual(t, created.Id, replacement.Id)
	})

	t.Run("removes an archived template that was never referenced", func(t *testing.T) {
		// Status is not part of the guard: archiving a template by mistake is the
		// case ADR-0010 named as unrecoverable, and the reference check alone is
		// what keeps the provenance guarantee, not the status.
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())
		require.Equal(t, http.StatusOK, archiveTemplate(t, tc, created.Id).StatusCode())

		resp := deleteTemplate(t, tc, created.Id)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
	})

	t.Run("refuses to delete a template an organization license names", func(t *testing.T) {
		w := newLicensedWorld(t)

		resp, err := w.client().DeleteLicenseTemplateWithResponse(
			context.Background(), w.productID(), w.TemplateID(),
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertAPIError(t, resp.JSON400.Errors, "LICENSE_TEMPLATE_IN_USE")

		// The refused write left the row untouched, and the organization's
		// license — which names this template as the statement of what it was
		// sold — still resolves it.
		still := w.Template().Read()
		assert.Equal(t, w.TemplateID(), still.Id)
		assert.Equal(t, w.TemplateID(), w.License().Get().TemplateId)
	})

	t.Run("404 when the product has no template with that identifier", func(t *testing.T) {
		tc := newTemplateCtx(t)

		assert.Equal(t, http.StatusNotFound, deleteTemplate(t, tc, missingTemplateID()).StatusCode())
	})

	t.Run("404 deleting an already-deleted template", func(t *testing.T) {
		// Unlike archive, delete is not idempotent: a second delete finds nothing
		// there, the ordinary answer for a second delete of anything.
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())
		require.Equal(t, http.StatusNoContent, deleteTemplate(t, tc, created.Id).StatusCode())

		again := deleteTemplate(t, tc, created.Id)
		assert.Equal(t, http.StatusNotFound, again.StatusCode(), string(again.Body))
	})
}
