package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLicenseTemplateScopes checks each route against a Product API key holding
// every other license template scope. The route security matrix already proves
// the contract declares security; this proves the four scopes are genuinely
// distinct, so a read-only key cannot rewrite what a tier grants.
func TestLicenseTemplateScopes(t *testing.T) {
	t.Run("read scope cannot write", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())
		readOnly, _ := tc.product.CreateAPIKeyClientWithScopes([]string{"license_template:read"})

		create, err := readOnly.CreateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseTemplateJSONRequestBody{
				Name:   uniqueTemplateName(),
				Values: validTemplateValues(),
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, create.StatusCode())

		update, err := readOnly.UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			created.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Name: new("Enterprise")},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, update.StatusCode())

		archive, err := readOnly.ArchiveLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, created.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, archive.StatusCode())

		del, err := readOnly.DeleteLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, created.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, del.StatusCode())
	})

	t.Run("write scopes cannot read", func(t *testing.T) {
		tc := newTemplateCtx(t)
		writeOnly, _ := tc.product.CreateAPIKeyClientWithScopes(
			[]string{"license_template:create", "license_template:update", "license_template:delete"},
		)

		created, err := writeOnly.CreateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseTemplateJSONRequestBody{
				Name:   uniqueTemplateName(),
				Values: validTemplateValues(),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))
		require.NotNil(t, created.JSON201)

		read, err := writeOnly.GetLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, created.JSON201.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, read.StatusCode())

		list, err := writeOnly.ListLicenseTemplatesWithResponse(
			context.Background(), tc.product.ProductID, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, list.StatusCode())
	})

	t.Run("delete scope cannot archive, update scope cannot delete", func(t *testing.T) {
		// Archive is an edit to the row's status, not a removal of the row, and is
		// scoped as one: license_template:update, not license_template:delete. A
		// key trusted only to remove templates outright must not thereby be
		// trusted to withdraw one in a way that keeps the record, and a key
		// trusted to edit templates must not thereby be trusted to destroy a row.
		// See docs/adr/0011-unreferenced-license-template-can-be-deleted.md.
		tc := newTemplateCtx(t)
		archivable := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())
		deletable := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		deleteOnly, _ := tc.product.CreateAPIKeyClientWithScopes([]string{"license_template:delete"})
		archive, err := deleteOnly.ArchiveLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, archivable.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, archive.StatusCode())

		updateOnly, _ := tc.product.CreateAPIKeyClientWithScopes([]string{"license_template:update"})
		del, err := updateOnly.DeleteLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, deletable.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, del.StatusCode())

		// And each does grant its own verb.
		archived, err := updateOnly.ArchiveLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, archivable.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, archived.StatusCode(), string(archived.Body))

		deleted, err := deleteOnly.DeleteLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, deletable.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, deleted.StatusCode(), string(deleted.Body))
	})

	t.Run("a license schema scope does not reach templates", func(t *testing.T) {
		tc := newTemplateCtx(t)
		// The two resources are separate entries in the permission catalog, so a
		// key trusted to declare what a license may contain is not thereby
		// trusted to decide what a tier grants.
		schemaOnly, _ := tc.product.CreateAPIKeyClientWithScopes(
			[]string{"license_schema:read", "license_schema:create", "license_schema:update"},
		)

		list, err := schemaOnly.ListLicenseTemplatesWithResponse(
			context.Background(), tc.product.ProductID, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, list.StatusCode())

		create, err := schemaOnly.CreateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseTemplateJSONRequestBody{
				Name:   uniqueTemplateName(),
				Values: validTemplateValues(),
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, create.StatusCode())
	})
}
