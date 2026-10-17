package ct_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailTemplatePublish(t *testing.T) {
	t.Parallel()
	tc := newEmailTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()

	t.Run("The API publishes draft and updates template pointer", func(t *testing.T) {
		created, err := client.CreateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:     uniqueSlug(),
				Name:     "Publish Me",
				Subject:  "Hello",
				BodyHtml: "<p>Hello</p>",
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode())
		tplID := created.JSON201.Id
		require.Nil(t, created.JSON201.PublishedVersionId)

		pub, publishErr := client.PublishEmailTemplateWithResponse(context.Background(), tc.product.ProductID, tplID)
		require.NoError(t, publishErr)
		assert.Equal(t, http.StatusOK, pub.StatusCode())
		require.NotNil(t, pub.JSON200)
		assert.Equal(t, ct.EmailTemplateVersionStatusPUBLISHED, pub.JSON200.Status)
		assert.NotNil(t, pub.JSON200.PublishedAt)

		get, getErr := client.GetEmailTemplateWithResponse(context.Background(), tc.product.ProductID, tplID)
		require.NoError(t, getErr)
		assert.NotNil(t, get.JSON200.PublishedVersionId)
		assert.Equal(t, pub.JSON200.Id, *get.JSON200.PublishedVersionId)
	})

	t.Run("Second publish creates new published version and opens fresh draft", func(t *testing.T) {
		created, err := client.CreateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:     uniqueSlug(),
				Name:     "Republish Me",
				Subject:  "v1",
				BodyHtml: "<p>v1</p>",
			},
		)
		require.NoError(t, err)
		tplID := created.JSON201.Id

		first, firstPublishErr := client.PublishEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
		)
		require.NoError(t, firstPublishErr)
		require.Equal(t, http.StatusOK, first.StatusCode())

		_, err = client.UpdateEmailTemplateDraftWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
			ct.UpdateEmailTemplateDraftJSONRequestBody{Subject: new("v2")},
		)
		require.NoError(t, err)

		second, secondPublishErr := client.PublishEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
		)
		require.NoError(t, secondPublishErr)
		assert.Equal(t, http.StatusOK, second.StatusCode())
		assert.NotEqual(t, first.JSON200.Id, second.JSON200.Id)
		assert.Equal(t, "v2", second.JSON200.Subject)
	})

	t.Run("The API returns 404 for unknown template", func(t *testing.T) {
		resp, publishErr := client.PublishEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ids.MustNew("etpl"),
		)
		require.NoError(t, publishErr)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

func TestEmailTemplateConcurrentDraftWritesAndPublishesKeepOneReferencedDraft(t *testing.T) {
	t.Parallel()

	t.Run("Email template concurrent draft writes and publishes keep one referenced draft", func(t *testing.T) {
		t.Parallel()

		tc := newEmailTestCtx(t)
		client := tc.product.OwnerAuthenticatedClient()

		created, err := client.CreateEmailTemplateWithResponse(t.Context(), tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:     uniqueSlug(),
				Name:     "Concurrent Publish",
				Subject:  "v1",
				BodyHtml: "<p>v1</p>",
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode())
		tplID := created.JSON201.Id

		const callsPerKind = 8
		var mu sync.Mutex
		var unexpected []string
		expectOK := func(kind string, status int, body []byte, callErr error) {
			mu.Lock()
			defer mu.Unlock()
			if callErr != nil {
				unexpected = append(unexpected, fmt.Sprintf("%s: %v", kind, callErr))
			} else if status != http.StatusOK {
				unexpected = append(unexpected, fmt.Sprintf("%s: %d %s", kind, status, body))
			}
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		for range callsPerKind {
			wg.Go(func() {
				<-start
				resp, callErr := client.PublishEmailTemplateWithResponse(t.Context(), tc.product.ProductID, tplID)
				if callErr != nil {
					expectOK("publish", 0, nil, callErr)
					return
				}
				expectOK("publish", resp.StatusCode(), resp.Body, nil)
			})
			wg.Go(func() {
				<-start
				resp, callErr := client.UpdateEmailTemplateDraftWithResponse(t.Context(), tc.product.ProductID, tplID,
					ct.UpdateEmailTemplateDraftJSONRequestBody{Subject: new("edited")})
				if callErr != nil {
					expectOK("update draft", 0, nil, callErr)
					return
				}
				expectOK("update draft", resp.StatusCode(), resp.Body, nil)
			})
			wg.Go(func() {
				<-start
				resp, callErr := client.GetEmailTemplateDraftWithResponse(t.Context(), tc.product.ProductID, tplID)
				if callErr != nil {
					expectOK("get draft", 0, nil, callErr)
					return
				}
				expectOK("get draft", resp.StatusCode(), resp.Body, nil)
			})
		}
		close(start)
		wg.Wait()
		assert.Empty(t, unexpected)

		var versions, drafts, published int
		require.NoError(t, testDB.QueryRowContext(t.Context(),
			`SELECT count(*), count(*) FILTER (WHERE status = 'DRAFT'), count(*) FILTER (WHERE status = 'PUBLISHED')
		FROM email_template_versions WHERE template_id = $1`,
			tplID,
		).Scan(&versions, &drafts, &published))
		assert.Equal(t, callsPerKind+1, versions)
		assert.Equal(t, 1, drafts)
		assert.Equal(t, 1, published)

		draft, err := client.GetEmailTemplateDraftWithResponse(t.Context(), tc.product.ProductID, tplID)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, draft.StatusCode(), string(draft.Body))
		template, err := client.GetEmailTemplateWithResponse(t.Context(), tc.product.ProductID, tplID)
		require.NoError(t, err)
		require.NotNil(t, template.JSON200.DraftVersionId)
		assert.Equal(t, draft.JSON200.Id, *template.JSON200.DraftVersionId)
		assert.Equal(t, callsPerKind+1, draft.JSON200.VersionNumber)
	})
}

func TestEmailTemplateDraftRepointsADraftTheTemplateLostTrackOf(t *testing.T) {
	t.Parallel()

	t.Run("Email template draft repoints a draft the template lost track of", func(t *testing.T) {
		t.Parallel()

		tc := newEmailTestCtx(t)
		client := tc.product.OwnerAuthenticatedClient()

		created, err := client.CreateEmailTemplateWithResponse(t.Context(), tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:     uniqueSlug(),
				Name:     "Lost Draft",
				Subject:  "v1",
				BodyHtml: "<p>v1</p>",
			},
		)
		require.NoError(t, err)
		tplID := created.JSON201.Id
		pub, err := client.PublishEmailTemplateWithResponse(t.Context(), tc.product.ProductID, tplID)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, pub.StatusCode())
		before, err := client.GetEmailTemplateWithResponse(t.Context(), tc.product.ProductID, tplID)
		require.NoError(t, err)
		require.NotNil(t, before.JSON200.DraftVersionId)
		lostDraftID := *before.JSON200.DraftVersionId

		_, err = testDB.ExecContext(t.Context(),
			`UPDATE email_templates SET draft_version_id = NULL WHERE id = $1`, tplID)
		require.NoError(t, err)

		update, err := client.UpdateEmailTemplateDraftWithResponse(t.Context(), tc.product.ProductID, tplID,
			ct.UpdateEmailTemplateDraftJSONRequestBody{Subject: new("v2")})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, update.StatusCode(), string(update.Body))
		assert.Equal(t, lostDraftID, update.JSON200.Id)

		after, err := client.GetEmailTemplateWithResponse(t.Context(), tc.product.ProductID, tplID)
		require.NoError(t, err)
		require.NotNil(t, after.JSON200.DraftVersionId)
		assert.Equal(t, lostDraftID, *after.JSON200.DraftVersionId)
	})
}
