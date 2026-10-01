package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailTemplateUpdate(t *testing.T) {
	t.Parallel()
	tc := newEmailTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()

	created, err := client.CreateEmailTemplateWithResponse(
		context.Background(),
		tc.product.ProductID,
		ct.CreateEmailTemplateJSONRequestBody{
			Slug:     uniqueSlug(),
			Name:     "Original",
			Subject:  "Subj",
			BodyHtml: "<p>Body</p>",
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, created.StatusCode())
	tplID := created.JSON201.Id

	t.Run("updates name", func(t *testing.T) {
		resp, updateErr := client.UpdateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
			ct.UpdateEmailTemplateJSONRequestBody{Name: new("Renamed")},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, "Renamed", resp.JSON200.Name)
	})

	t.Run("updates description", func(t *testing.T) {
		resp, updateErr := client.UpdateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
			ct.UpdateEmailTemplateJSONRequestBody{Description: new("new desc")},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("deactivates template", func(t *testing.T) {
		resp, updateErr := client.UpdateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
			ct.UpdateEmailTemplateJSONRequestBody{IsActive: new(false)},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.False(t, resp.JSON200.IsActive)
	})

	t.Run("returns 404 for unknown ID", func(t *testing.T) {
		resp, updateErr := client.UpdateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ids.MustNew("etpl"),
			ct.UpdateEmailTemplateJSONRequestBody{Name: new("x")},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

// Not parallel: it holds a row lock and waits for the rename to queue behind it.
func TestEmailTemplateRenameKeepsExamplesSavedConcurrently(t *testing.T) {
	tc := newEmailTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()
	created, err := client.CreateEmailTemplateWithResponse(
		t.Context(),
		tc.product.ProductID,
		ct.CreateEmailTemplateJSONRequestBody{
			Slug:     uniqueSlug(),
			Name:     "Original",
			Subject:  "Subj",
			BodyHtml: "<p>Body</p>",
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, created.StatusCode())
	tplID := created.JSON201.Id

	saveInFlight, err := testDB.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = saveInFlight.Rollback() })
	_, err = saveInFlight.ExecContext(
		t.Context(),
		`UPDATE email_templates SET example_data = $2 WHERE id = $1`,
		tplID, `[{"id": "ex-1", "name": "Happy path", "variables": {"first_name": "Ada"}}]`,
	)
	require.NoError(t, err)
	var savePID int
	require.NoError(t, saveInFlight.QueryRowContext(t.Context(), `SELECT pg_backend_pid()`).Scan(&savePID))

	renamed := make(chan functional.Result[*ct.UpdateEmailTemplateResponse], 1)
	go func() {
		renamed <- functional.New(client.UpdateEmailTemplateWithResponse(
			t.Context(), tc.product.ProductID, tplID, ct.UpdateEmailTemplateJSONRequestBody{Name: new("Renamed")},
		))
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		scanErr := testDB.QueryRowContext(t.Context(),
			`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)))`, savePID,
		).Scan(&blocked)
		return scanErr == nil && blocked
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, saveInFlight.Commit())

	rename, err := (<-renamed).Value()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rename.StatusCode(), string(rename.Body))
	assert.Equal(t, "Renamed", rename.JSON200.Name)

	examples, err := client.GetEmailTemplateExamplesWithResponse(t.Context(), tc.product.ProductID, tplID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, examples.StatusCode())
	require.NotNil(t, examples.JSON200)
	assert.Equal(t, []ct.TemplateExample{
		{Id: "ex-1", Name: "Happy path", Variables: map[string]any{"first_name": "Ada"}},
	}, examples.JSON200.Examples)
}
