package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/nanostack-dev/pgkit/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

func (w *licenseWorld) writeLockKey() string {
	return "licensing-write:" + w.tenantID + ":" + w.productID()
}

func (w *licenseWorld) holdWriteLock() func() {
	w.t.Helper()
	tx, err := testDB.BeginTx(w.t.Context(), nil)
	require.NoError(w.t, err)
	w.t.Cleanup(func() { _ = tx.Rollback() })
	acquired, err := pglock.TryAdvisoryXactLock(w.t.Context(), tx, w.writeLockKey())
	require.NoError(w.t, err)
	require.True(w.t, acquired)
	return func() { require.NoError(w.t, tx.Rollback()) }
}

func assertLicensingWriteConflict(t *testing.T, call func(context.Context) (*http.Response, error)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	response, err := call(ctx)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusConflict, response.StatusCode)
	var body ct.ApiErrorResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
	assertLicenseAPIError(t, body.Errors, "LICENSING_WRITE_IN_PROGRESS")
	assert.Equal(
		t,
		"Another licensing update is in progress for this product. Please try again shortly.",
		body.Errors[0].Message,
	)
}

// Not parallel: it asserts lock contention within tight timeouts.
func TestLicensingWritesRejectConcurrentOperation(t *testing.T) {
	w := newLicensedWorld(t)
	other := newLicensedWorld(t)
	unlicensedID := w.NewOrganization()
	client := w.client()
	values := validTemplateValues()
	fields := templateSchemaFields()
	release := w.holdWriteLock()

	for _, test := range []struct {
		name string
		call func(context.Context) (*http.Response, error)
	}{
		{"create schema", func(ctx context.Context) (*http.Response, error) {
			return client.CreateLicenseSchema(ctx, w.productID(), ct.CreateLicenseSchemaJSONRequestBody{Fields: fields})
		}},
		{"update schema", func(ctx context.Context) (*http.Response, error) {
			return client.UpdateLicenseSchema(ctx, w.productID(), ct.UpdateLicenseSchemaJSONRequestBody{Fields: &fields})
		}},
		{"delete schema", func(ctx context.Context) (*http.Response, error) {
			return client.DeleteLicenseSchema(ctx, w.productID())
		}},
		{"create template", func(ctx context.Context) (*http.Response, error) {
			return client.CreateLicenseTemplate(ctx, w.productID(), ct.CreateLicenseTemplateJSONRequestBody{Name: uniqueTemplateName(), Values: values})
		}},
		{"update template values", func(ctx context.Context) (*http.Response, error) {
			return client.UpdateLicenseTemplate(ctx, w.productID(), w.TemplateID(), ct.UpdateLicenseTemplateJSONRequestBody{Values: &values})
		}},
		{"rename template", func(ctx context.Context) (*http.Response, error) {
			return client.UpdateLicenseTemplate(ctx, w.productID(), w.TemplateID(), ct.UpdateLicenseTemplateJSONRequestBody{Name: new("Renamed")})
		}},
		{"archive template", func(ctx context.Context) (*http.Response, error) {
			return client.ArchiveLicenseTemplate(ctx, w.productID(), w.TemplateID())
		}},
		{"delete template", func(ctx context.Context) (*http.Response, error) {
			return client.DeleteLicenseTemplate(ctx, w.productID(), w.TemplateID())
		}},
		{"adjust license", func(ctx context.Context) (*http.Response, error) {
			return client.AdjustOrganizationLicense(ctx, w.productID(), w.OrganizationID(), ct.AdjustOrganizationLicenseJSONRequestBody{Values: ct.LicenseTemplateValues{"flows": 800}})
		}},
		{"migrate licenses", func(ctx context.Context) (*http.Response, error) {
			return client.MigrateOrganizationLicenses(ctx, w.productID(), ct.MigrateOrganizationLicensesJSONRequestBody{
				TemplateId: w.TemplateID(), OrganizationIds: new([]string{unlicensedID}),
			})
		}},
	} {
		t.Run(test.name, func(t *testing.T) { assertLicensingWriteConflict(t, test.call) })
	}

	assertValues(t, w.License().Get().Values, values)
	assert.Empty(t, w.License().Get().AdjustedFields)
	assertValues(t, w.Template().Read().Values, values)
	assert.Equal(t, http.StatusNotFound, w.License().For(unlicensedID).GetRaw().StatusCode())
	other.License().Adjust(ct.LicenseTemplateValues{"flows": 700})
	w.Usage().Report(ct.UsageReportRequest{Key: "flows", Value: 12})

	release()
	w.License().For(unlicensedID).Instantiate(w.TemplateID())
	w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})
	assert.Equal(t, []string{"flows"}, w.License().Get().AdjustedFields)
}

func (w *licenseWorld) waitForWriteLock() {
	w.t.Helper()
	key := uint64(pglock.KeyHash(w.writeLockKey()))
	require.Eventually(w.t, func() bool {
		var held bool
		err := testDB.QueryRowContext(w.t.Context(), `SELECT EXISTS (
			SELECT 1 FROM pg_locks WHERE locktype = 'advisory' AND granted
			AND classid::bigint = $1 AND objid::bigint = $2 AND objsubid = 1
		)`, key>>32, key&0xffffffff).Scan(&held)
		return err == nil && held
	}, 5*time.Second, 10*time.Millisecond)
}

// Not parallel: it asserts lock contention within tight timeouts.
func TestTemplateRenameRejectsConcurrentValuesWrite(t *testing.T) {
	w := newLicensedWorld(t)
	rowLock, err := testDB.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rowLock.Rollback() })
	_, err = rowLock.ExecContext(
		t.Context(),
		`SELECT id FROM license_templates WHERE id = $1 FOR UPDATE`,
		w.TemplateID(),
	)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	first := make(chan functional.Result[*ct.UpdateLicenseTemplateResponse], 1)
	go func() {
		first <- functional.New(w.client().UpdateLicenseTemplateWithResponse(ctx, w.productID(), w.TemplateID(), ct.UpdateLicenseTemplateJSONRequestBody{Name: new("Renamed")}))
	}()
	w.waitForWriteLock()
	values := templateValuesWith("flows", 900)
	assertLicensingWriteConflict(t, func(ctx context.Context) (*http.Response, error) {
		return w.client().
			UpdateLicenseTemplate(ctx, w.productID(), w.TemplateID(), ct.UpdateLicenseTemplateJSONRequestBody{Values: &values})
	})
	require.NoError(t, rowLock.Rollback())
	response, err := (<-first).Value()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	w.Template().ReplaceValues(values)
	waitForLicenseValues(t, w.License(), values)
	assert.Equal(t, "Renamed", w.Template().Read().Name)
}

// Not parallel: it asserts lock contention within tight timeouts.
func TestMigrationHoldsWriteLockAcrossOrganizationTransactions(t *testing.T) {
	w := newLicensedWorld(t)
	secondID := w.NewOrganization()
	rowLock, err := testDB.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rowLock.Rollback() })
	_, err = rowLock.ExecContext(
		t.Context(),
		`SELECT id FROM organization_licenses WHERE organization_id = $1 FOR UPDATE`,
		w.OrganizationID(),
	)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	first := make(chan functional.Result[*ct.MigrateOrganizationLicensesResponse], 1)
	go func() {
		first <- functional.New(w.client().MigrateOrganizationLicensesWithResponse(ctx, w.productID(), ct.MigrateOrganizationLicensesJSONRequestBody{
			TemplateId: w.TemplateID(), OrganizationIds: new([]string{w.OrganizationID(), secondID}), OnDifference: new(ct.DISCARD),
		}))
	}()
	w.waitForWriteLock()
	values := templateValuesWith("flows", 900)
	assertLicensingWriteConflict(t, func(ctx context.Context) (*http.Response, error) {
		return w.client().
			UpdateLicenseTemplate(ctx, w.productID(), w.TemplateID(), ct.UpdateLicenseTemplateJSONRequestBody{Values: &values})
	})
	require.NoError(t, rowLock.Rollback())
	response, err := (<-first).Value()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	require.NotNil(t, response.JSON200)
	assert.Zero(t, response.JSON200.Failed)
	w.Template().ReplaceValues(values)
	waitForLicenseValues(t, w.License(), values)
	waitForLicenseValues(t, w.License().For(secondID), values)
}

// Not parallel: it asserts lock contention within tight timeouts.
func TestTemplateSyncRetriesLicensingWriteConflict(t *testing.T) {
	w := newLicensedWorld(t)
	payload, err := json.Marshal(
		map[string]string{"tenant_id": w.tenantID, "product_id": w.productID(), "template_id": w.TemplateID()},
	)
	require.NoError(t, err)
	release := w.holdWriteLock()
	err = templateSync.ProcessQueueJob(t.Context(), queue.Job{Payload: payload})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Another licensing update is in progress")
	assert.False(t, queue.IsNonRetryable(err))
	release()
	require.NoError(t, templateSync.ProcessQueueJob(t.Context(), queue.Job{Payload: payload}))
}

// holdRowForUpdate locks one row until the returned release, stalling any
// write whose foreign key references it right after everything it did before.
func holdRowForUpdate(t *testing.T, table, id string) func() {
	t.Helper()
	tx, err := testDB.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(t.Context(), `SELECT id FROM `+table+` WHERE id = $1 FOR UPDATE`, id)
	require.NoError(t, err)
	return func() { require.NoError(t, tx.Rollback()) }
}

func waitUntilStatementWaitsOnALock(t *testing.T, statement, table string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting bool
		err := testDB.QueryRowContext(t.Context(), `SELECT EXISTS (
			SELECT 1 FROM pg_stat_activity WHERE wait_event_type = 'Lock'
			AND query ILIKE $1 AND query ILIKE $2
		)`, "%"+statement+"%", "%"+table+"%").Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "no %s on %s waits on a lock", statement, table)
}

func replaceTemplateValuesInBackground(
	w *licenseWorld, values ct.LicenseTemplateValues,
) <-chan functional.Result[*ct.UpdateLicenseTemplateResponse] {
	client := w.client()
	updated := make(chan functional.Result[*ct.UpdateLicenseTemplateResponse], 1)
	go func() {
		updated <- functional.New(client.UpdateLicenseTemplateWithResponse(
			context.Background(), w.productID(), w.TemplateID(),
			ct.UpdateLicenseTemplateJSONRequestBody{Values: &values},
		))
	}()
	return updated
}

func requireTemplateUpdated(t *testing.T, updated <-chan functional.Result[*ct.UpdateLicenseTemplateResponse]) {
	t.Helper()
	response, err := (<-updated).Value()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
}

func TestConcurrentLicenseInstantiationsDoNotRefuseEachOther(t *testing.T) {
	t.Parallel()
	const organizations = 8
	w := newLicensedWorld(t)
	client := w.client()
	apiClient := w.product.AllScopeAPIKeyClient()
	unlicensedIDs := make([]string, organizations)
	names := make([]string, organizations)
	statuses := make([]int, 2*organizations)
	for i := range organizations {
		unlicensedIDs[i] = w.NewOrganization()
		names[i] = itdsl.UniqueOrganizationName()
	}

	start := make(chan struct{})
	var writes sync.WaitGroup
	for i := range organizations {
		writes.Go(func() {
			<-start
			response, err := client.InstantiateOrganizationLicenseWithResponse(
				context.Background(), w.productID(), unlicensedIDs[i],
				ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: w.TemplateID()},
			)
			if err == nil {
				statuses[2*i] = response.StatusCode()
			}
		})
		writes.Go(func() {
			<-start
			response, err := apiClient.CreateProductOrganizationWithResponse(
				context.Background(), w.productID(), organizationBody(names[i], new(w.TemplateID())),
			)
			if err == nil {
				statuses[2*i+1] = response.StatusCode()
			}
		})
	}
	close(start)
	writes.Wait()

	for i, status := range statuses {
		assert.Equal(t, http.StatusCreated, status, "licensing write %d", i)
	}
}

// Not parallel: it asserts lock contention within tight timeouts.
func TestLicenseInstantiationIgnoresTheProductLicensingLock(t *testing.T) {
	w := newLicensedWorld(t)
	unlicensedID := w.NewOrganization()
	name := itdsl.UniqueOrganizationName()
	release := w.holdWriteLock()

	w.License().For(unlicensedID).Instantiate(w.TemplateID())
	created := w.Organizations().Create(organizationBody(name, new(w.TemplateID())))

	release()
	require.NotNil(t, created.License)
	assertValues(t, created.License.Values, validTemplateValues())
}

// Not parallel: it asserts lock contention within tight timeouts.
func TestTemplateUpdateDuringOrganizationCreationReachesTheNewLicense(t *testing.T) {
	w := newLicenseWorld(t)
	founder := w.FoundingMember()
	body := organizationBody(itdsl.UniqueOrganizationName(), new(w.TemplateID()))
	body.FoundingMember = &ct.FoundingMemberRequest{ProductUserId: founder.ProductUserID, RoleId: founder.RoleID}
	updatedValues := templateValuesWith("flows", 900)
	apiClient := w.product.AllScopeAPIKeyClient()
	releaseRole := holdRowForUpdate(t, "product_roles", founder.RoleID)

	created := make(chan functional.Result[*ct.CreateProductOrganizationResponse], 1)
	go func() {
		created <- functional.New(apiClient.CreateProductOrganizationWithResponse(context.Background(), w.productID(), body))
	}()
	waitUntilStatementWaitsOnALock(t, "INSERT", "organization_memberships")
	updated := replaceTemplateValuesInBackground(w, updatedValues)
	waitUntilStatementWaitsOnALock(t, "", "license_templates")
	releaseRole()

	response, err := (<-created).Value()
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, response.StatusCode(), string(response.Body))
	require.NotNil(t, response.JSON201.License)
	assertValues(t, response.JSON201.License.Values, validTemplateValues())
	requireTemplateUpdated(t, updated)
	waitForLicenseValues(t, w.License().For(response.JSON201.Id), updatedValues)
}

// Not parallel: it asserts lock contention within tight timeouts.
func TestTemplateUpdateDuringInstantiationReachesTheNewLicense(t *testing.T) {
	w := newLicenseWorld(t)
	updatedValues := templateValuesWith("flows", 900)
	client := w.client()
	releaseOrganization := holdRowForUpdate(t, "organizations", w.OrganizationID())

	instantiated := make(chan functional.Result[*ct.InstantiateOrganizationLicenseResponse], 1)
	go func() {
		instantiated <- functional.New(client.InstantiateOrganizationLicenseWithResponse(
			context.Background(), w.productID(), w.OrganizationID(),
			ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: w.TemplateID()},
		))
	}()
	waitUntilStatementWaitsOnALock(t, "INSERT", "organization_licenses")
	updated := replaceTemplateValuesInBackground(w, updatedValues)
	waitUntilStatementWaitsOnALock(t, "", "license_templates")
	releaseOrganization()

	response, err := (<-instantiated).Value()
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, response.StatusCode(), string(response.Body))
	assertValues(t, response.JSON201.Values, validTemplateValues())
	requireTemplateUpdated(t, updated)
	waitForLicenseValues(t, w.License(), updatedValues)
}
