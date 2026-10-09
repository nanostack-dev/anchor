package ct_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func uniqueLocalEmail(prefix string) string {
	return prefix + "-" + itshared.Faker.UUID().V4()[:8] + "@example.com"
}

func organizationForEveryUser() ct.WorkflowWriteRequest {
	return workflowBody("product_user.created", step("org", "organization.create", map[string]string{
		"name": "Team of {{event.data.product_user_id}}",
	}))
}

func userForEveryOrganization(email string) ct.WorkflowWriteRequest {
	return workflowBody("organization.created", step("user", "product_user.create", map[string]string{
		"email": email,
	}))
}

func TestCreateWorkflow_RefusesAUserAndAnOrganizationThatCreateEachOther(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	w.createWorkflow(organizationForEveryUser())

	resp := w.createWorkflowRaw(userForEveryOrganization(uniqueLocalEmail("owner")))

	assertLoopRefused(t, resp, 2)
}

func TestWorkflow_CreateProductUserGivesTheNewUserToLaterSteps(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	email := uniqueLocalEmail("ada.lovelace")
	created := w.createWorkflow(workflowBody("organization.created",
		step("user", "product_user.create", map[string]string{"email": email}),
		step("read", "product_user.get", map[string]string{"product_user_id": "{{steps.user.product_user_id}}"}),
	))

	w.newOrganization()

	run := w.waitForRuns(created.Id, 1)[0]
	require.Equal(t, ct.WorkflowRunStatusSucceeded, run.Status)
	require.Len(t, run.Steps, 2)
	require.NotNil(t, run.Steps[1].Output)
	read := *run.Steps[1].Output
	localPart, _, _ := strings.Cut(email, "@")
	assert.Equal(t, email, read["email"])
	assert.Equal(t, localPart, read["name"])
}

// TestWorkflow_AUserAndOrganizationLoopThatGotSavedRunsOnce enables the second
// workflow behind the save check, as two saves racing without the save lock
// would. The run guard still stops the loop at the first repeated workflow.
func TestWorkflow_AUserAndOrganizationLoopThatGotSavedRunsOnce(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	organizations := w.createWorkflow(organizationForEveryUser())
	users := userForEveryOrganization(uniqueLocalEmail("loop"))
	users.Enabled = false
	usersWorkflow := w.createWorkflow(users)
	_, err := testDB.ExecContext(t.Context(),
		`UPDATE product_workflows SET enabled = true WHERE id = $1`, usersWorkflow.Id)
	require.NoError(t, err)

	w.createProductUser(uniqueLocalEmail("first"))

	organizationRuns := w.waitForRuns(organizations.Id, 2)
	byStatus := map[ct.WorkflowRunStatus][]ct.WorkflowRunResponse{}
	for _, run := range organizationRuns {
		byStatus[run.Status] = append(byStatus[run.Status], run)
	}
	require.Len(t, byStatus[ct.WorkflowRunStatusSucceeded], 1, organizationRuns)
	require.Len(t, byStatus[ct.WorkflowRunStatusSkipped], 1, organizationRuns)
	prevented := byStatus[ct.WorkflowRunStatusSkipped][0]
	require.NotNil(t, prevented.Error)
	assert.Contains(t, *prevented.Error, "Loop prevented")
	userRuns := w.waitForRuns(usersWorkflow.Id, 1)
	assert.Equal(t, ct.WorkflowRunStatusSucceeded, userRuns[0].Status)
	assert.Len(t, w.runs(organizations.Id), 2)
	assert.Len(t, w.runs(usersWorkflow.Id), 1)
}

// TestCreateWorkflow_WaitsForAnInFlightSaveAndSeesTheLoopItCloses holds the
// product's save lock in a transaction that enables the first workflow, as a
// save in progress does. The second save waits for it, then refuses the loop
// it would close; without the lock it would read the first workflow as still
// disabled and save the loop.
func TestCreateWorkflow_WaitsForAnInFlightSaveAndSeesTheLoopItCloses(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	first := organizationForEveryUser()
	first.Enabled = false
	organizations := w.createWorkflow(first)
	var tenantID string
	require.NoError(t, testDB.QueryRowContext(t.Context(),
		`SELECT platform_tenant_id FROM product_workflows WHERE id = $1`, organizations.Id,
	).Scan(&tenantID))

	saveInFlight, err := testDB.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = saveInFlight.Rollback() })
	_, err = saveInFlight.ExecContext(t.Context(), `SELECT pg_advisory_xact_lock($1)`,
		pglock.KeyHash("workflow-save:"+tenantID+":"+w.product.ProductID))
	require.NoError(t, err)
	_, err = saveInFlight.ExecContext(t.Context(),
		`UPDATE product_workflows SET enabled = true WHERE id = $1`, organizations.Id)
	require.NoError(t, err)
	var savePID int
	require.NoError(t, saveInFlight.QueryRowContext(t.Context(), `SELECT pg_backend_pid()`).Scan(&savePID))

	second := make(chan functional.Result[*ct.CreateWorkflowResponse], 1)
	go func() {
		second <- functional.New(w.client.CreateWorkflowWithResponse(
			context.Background(), w.product.ProductID, userForEveryOrganization(uniqueLocalEmail("race")),
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

	resp, err := (<-second).Value()
	require.NoError(t, err)
	assertLoopRefused(t, resp, 2)
	list, err := w.client.ListWorkflowsWithResponse(
		context.Background(),
		w.product.ProductID,
		&ct.ListWorkflowsParams{},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, list.StatusCode())
	assert.Len(t, list.JSON200.Items, 1)
}
