package ct_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
	resourceservice "anchor/internal/resource/service"
)

const (
	workspaceType = "workspace"
	// sweepWait covers a few runs of the sweeper, which replays every 5 s.
	sweepWait = 30 * time.Second
	sweepPoll = 100 * time.Millisecond
)

// backfillLog collects a backfill's log lines, so a test can see what it
// logged about its own product.
type backfillLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *backfillLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// logged reports whether a line of the level carries every field and a
// message containing the text.
func (l *backfillLog) logged(t *testing.T, level, text string, fields map[string]string) bool {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	scanner := bufio.NewScanner(bytes.NewReader(l.buf.Bytes()))
	for scanner.Scan() {
		var line map[string]any
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &line))
		if line["level"] != level || !strings.Contains(line["message"].(string), text) {
			continue
		}
		matches := true
		for key, value := range fields {
			matches = matches && line[key] == value
		}
		if matches {
			return true
		}
	}
	return false
}

// newWorkspaceBackfill builds the startup backfill with its own logger. A test
// runs it on its own product only, never on every product.
func newWorkspaceBackfill() (*resourceservice.WorkspaceBackfill, *backfillLog) {
	log := &backfillLog{}
	logger := zerolog.New(log)
	mirror := resourceservice.NewWorkspaces(resourceSvc, resourceTypeSvc, logger)
	return resourceservice.NewWorkspaceBackfill(
		workspaceRepo, mirror, resourceRepo, resourceTypeSvc, roleAssignmentSvc, initialWorkspaceTx, logger,
	), log
}

// insertWorkspace writes a workspace as a release before the resource mirror
// did: with no resource.
func insertWorkspace(t *testing.T, organizationID, name string) string {
	t.Helper()
	id := ids.MustNew("ws")
	_, err := testDB.ExecContext(t.Context(),
		`INSERT INTO workspaces (id, organization_id, name) VALUES ($1, $2, $3)`, id, organizationID, name)
	require.NoError(t, err)
	return id
}

func insertWorkspaceMembership(t *testing.T, workspaceID, productUserID, roleID string) {
	t.Helper()
	_, err := testDB.ExecContext(t.Context(),
		`INSERT INTO workspace_memberships (workspace_id, product_user_id, product_role_id) VALUES ($1, $2, $3)`,
		workspaceID, productUserID, roleID)
	require.NoError(t, err)
}

func workspaceMembershipExists(t *testing.T, workspaceID, productUserID string) bool {
	t.Helper()
	var count int
	require.NoError(t, testDB.QueryRowContext(t.Context(),
		`SELECT count(*) FROM workspace_memberships WHERE workspace_id = $1 AND product_user_id = $2`,
		workspaceID, productUserID).Scan(&count))
	return count > 0
}

// requireWorkspaceResource asserts that the workspace is registered under its
// Organization with the workspace's name.
func requireWorkspaceResource(
	t *testing.T, product *itdsl.ProductContext, organizationID, workspaceID, name string,
) ct.ResourceResponse {
	t.Helper()
	got := product.Resources(organizationID).Get(workspaceType, workspaceID)
	assert.Equal(t, workspaceID, got.Id)
	assert.Equal(t, name, got.Name)
	assert.Equal(t, ct.ResourceParentRef{Type: "organization", Id: organizationID}, got.Parent)
	return got
}

// waitForSweep waits until the sweeper replayed the product's marks: a
// backfill runs outside a request, so no request replays its writes.
func waitForSweep(t *testing.T, productID string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var pending int
		require.NoError(t, testDB.QueryRowContext(t.Context(), `SELECT
			(SELECT count(*) FROM authz_resource_marks WHERE product_id = $1) +
			(SELECT count(*) FROM authz_sync_marks WHERE product_id = $1)`, productID).Scan(&pending))
		return pending == 0
	}, sweepWait, sweepPoll)
}

func requireNoWorkspaceResource(t *testing.T, product *itdsl.ProductContext, organizationID, workspaceID string) {
	t.Helper()
	resp := product.Resources(organizationID).GetRaw(workspaceType, workspaceID)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
}

func TestWorkspaceBackfill(t *testing.T) {
	t.Parallel()

	t.Run("Workspaces become resources with their IDs and names under their organizations, once", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		sink := product.CaptureEvents()
		first := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		second := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		payments := insertWorkspace(t, first.Id, "Payments")
		billing := insertWorkspace(t, second.Id, "Billing")
		backfill, _ := newWorkspaceBackfill()

		require.NoError(t, backfill.RunProduct(t.Context(), product.ProductID))

		declared := product.ResourceTypes().BySlug(workspaceType)
		assert.Equal(t, []string{"organization"}, declared.ParentTypes)
		assert.False(t, declared.NestsInItself)
		sink.WaitFor("product.resource_type.created",
			map[string]string{"resource_type": workspaceType, "resource_type_id": declared.Id})
		before := []ct.ResourceResponse{
			requireWorkspaceResource(t, product, first.Id, payments, "Payments"),
			requireWorkspaceResource(t, product, second.Id, billing, "Billing"),
		}

		require.NoError(t, backfill.RunProduct(t.Context(), product.ProductID))

		assert.Equal(t, declared, product.ResourceTypes().BySlug(workspaceType))
		after := []ct.ResourceResponse{
			requireWorkspaceResource(t, product, first.Id, payments, "Payments"),
			requireWorkspaceResource(t, product, second.Id, billing, "Billing"),
		}
		assert.Equal(t, before, after)
		waitForSweep(t, product.ProductID)
	})

	t.Run("A declared workspace type that sits under the organization is reused", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		product.ResourceTypes().CreateUnder("project", "organization")
		declared := product.ResourceTypes().CreateUnder(workspaceType, "organization", "project")
		org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		workspaceID := insertWorkspace(t, org.Id, "Payments")
		backfill, _ := newWorkspaceBackfill()

		require.NoError(t, backfill.RunProduct(t.Context(), product.ProductID))

		assert.Equal(t, declared.Id, product.ResourceTypes().BySlug(workspaceType).Id)
		requireWorkspaceResource(t, product, org.Id, workspaceID, "Payments")
		waitForSweep(t, product.ProductID)
	})

	t.Run("A workspace type that can't sit under the organization is left alone, and logged", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		product.ResourceTypes().CreateUnder("project", "organization")
		product.ResourceTypes().CreateUnder(workspaceType, "project")
		org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		workspaceID := insertWorkspace(t, org.Id, "Payments")
		backfill, log := newWorkspaceBackfill()

		require.NoError(t, backfill.RunProduct(t.Context(), product.ProductID))

		assert.Equal(t, []string{"project"}, product.ResourceTypes().BySlug(workspaceType).ParentTypes)
		requireNoWorkspaceResource(t, product, org.Id, workspaceID)
		assert.True(t, log.logged(t, "error", "can't sit under the organization",
			map[string]string{"product_id": product.ProductID}))
	})

	t.Run("A membership with a workspace role moves to an assignment; one with an organization role stays",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			product.ResourceTypes().CreateUnder(workspaceType, "organization")
			orgMember := createRoleTemplate(t, product, ct.CreateProductRoleJSONRequestBody{
				Name: "Member " + ids.MustNew("ct"),
			})
			editor := createRoleTemplate(t, product, roleOn(workspaceType, "Workspace editor"))
			org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
			withWorkspaceRole := createDSLProductUser(t, product).ID
			withOrganizationRole := createDSLProductUser(t, product).ID
			addMember(t, product, org.Id, withWorkspaceRole, orgMember.Id)
			addMember(t, product, org.Id, withOrganizationRole, orgMember.Id)
			workspaceID := insertWorkspace(t, org.Id, "Payments")
			insertWorkspaceMembership(t, workspaceID, withWorkspaceRole, editor.Id)
			insertWorkspaceMembership(t, workspaceID, withOrganizationRole, orgMember.Id)
			sink := product.CaptureEvents()
			backfill, log := newWorkspaceBackfill()

			require.NoError(t, backfill.RunProduct(t.Context(), product.ProductID))

			members := product.ResourceMembers(org.Id, workspaceType, workspaceID)
			held := members.Get(withWorkspaceRole)
			require.Len(t, held.Roles, 1)
			assert.Equal(t, editor.Name, held.Roles[0].Name)
			assert.False(t, workspaceMembershipExists(t, workspaceID, withWorkspaceRole))
			sink.WaitFor("resource.role_assignment.created", map[string]string{
				"resource_type": workspaceType, "resource_id": workspaceID, "product_user_id": withWorkspaceRole,
			})
			resp := members.GetRaw(withOrganizationRole)
			assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
			assert.True(t, workspaceMembershipExists(t, workspaceID, withOrganizationRole))
			assert.True(t, log.logged(t, "warn", "left in workspace_memberships", map[string]string{
				"product_id": product.ProductID, "workspace_id": workspaceID, "product_user_id": withOrganizationRole,
				"role_id": orgMember.Id,
			}))

			require.NoError(t, backfill.RunProduct(t.Context(), product.ProductID))

			assert.Equal(t, held, members.Get(withWorkspaceRole))
			assert.True(t, workspaceMembershipExists(t, workspaceID, withOrganizationRole))
			waitForSweep(t, product.ProductID)
		})
}

func TestWorkspaceRoutesWriteTheirResource(t *testing.T) {
	t.Parallel()

	t.Run("A product's first workspace declares its workspace type under the organization", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		sink := product.CaptureEvents()
		org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)

		created := product.Workspaces(org.Id).Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Payments"})

		declared := product.ResourceTypes().BySlug(workspaceType)
		assert.Equal(t, []string{"organization"}, declared.ParentTypes)
		sink.WaitFor("product.resource_type.created", map[string]string{"resource_type": workspaceType})
		requireWorkspaceResource(t, product, org.Id, created.Id, "Payments")
	})

	t.Run("An organization created with a workspace registers the workspace", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		org := product.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{
			Name: ids.MustNew("org"), Workspace: &ct.ProductWorkspaceRequest{Name: "Engineering"},
		})

		initial := product.Workspaces(org.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
		require.Len(t, initial.Items, 1)
		requireWorkspaceResource(t, product, org.Id, initial.Items[0].Id, "Engineering")
	})

	t.Run("A rename renames the resource, and a delete removes it with its role assignments", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		product.ResourceTypes().CreateUnder(workspaceType, "organization")
		orgMember := createRoleTemplate(t, product, ct.CreateProductRoleJSONRequestBody{
			Name: "Member " + ids.MustNew("ct"),
		})
		editor := createRoleTemplate(t, product, roleOn(workspaceType, "Workspace editor"))
		org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		member := createDSLProductUser(t, product).ID
		addMember(t, product, org.Id, member, orgMember.Id)
		workspaces := product.Workspaces(org.Id)
		created := workspaces.Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Payments"})
		product.ResourceMembers(org.Id, workspaceType, created.Id).Put(member, editor.Id)
		sink := product.CaptureEvents()

		workspaces.Update(created.Id, ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: "Billing"})
		requireWorkspaceResource(t, product, org.Id, created.Id, "Billing")

		description := "Billing team"
		workspaces.Update(created.Id, ct.UpdateOrganizationWorkspaceJSONRequestBody{
			Name: "Billing", Description: &description,
		})
		requireWorkspaceResource(t, product, org.Id, created.Id, "Billing")

		workspaces.Delete(created.Id)
		requireNoWorkspaceResource(t, product, org.Id, created.Id)
		sink.WaitFor("resource.role_assignment.deleted", map[string]string{
			"resource_type": workspaceType, "resource_id": created.Id, "product_user_id": member,
		})
	})

	t.Run("A rename puts back a resource the product moved or removed", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		product.ResourceTypes().CreateUnder("project", "organization")
		product.ResourceTypes().CreateUnder(workspaceType, "organization", "project")
		org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		resources := product.Resources(org.Id)
		project := resourceID()
		resources.Put("project", project, "Roadmap", resources.Top())
		workspaces := product.Workspaces(org.Id)
		moved := workspaces.Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Payments"})
		removed := workspaces.Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Billing"})
		resources.Put(workspaceType, moved.Id, "Moved", itdsl.Ref("project", project))
		resources.Delete(workspaceType, removed.Id, false)

		workspaces.Update(moved.Id, ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: "Payments"})
		workspaces.Update(removed.Id, ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: "Billing"})

		requireWorkspaceResource(t, product, org.Id, moved.Id, "Payments")
		requireWorkspaceResource(t, product, org.Id, removed.Id, "Billing")
	})

	t.Run("A workspace registered through the resource routes creates no workspace", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		product.ResourceTypes().CreateUnder(workspaceType, "organization")
		org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		resources := product.Resources(org.Id)
		registered := resourceID()

		resources.Put(workspaceType, registered, "Registered", resources.Top())

		listed := product.Workspaces(org.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
		assert.Empty(t, listed.Items)
	})
}

// workspaceAccessWorld is a product whose workspaces are resources: an
// Organization with two workspaces, another Organization with one, a
// permission scoped to the workspace type and one to the organization.
type workspaceAccessWorld struct {
	product         *itdsl.ProductContext
	org, other      ct.ProductOrganizationResponse
	shared, private string
	foreign         string
	// admin holds an organization role bundling both permissions; editor
	// holds a member role and the editor role on shared.
	adminExternalID, editorExternalID string
	editor                            ct.ProductRoleResponse
	viewWorkspace, readReports        string
}

func givenWorkspaceAccessWorld(t *testing.T) workspaceAccessWorld {
	t.Helper()
	product := createTestProductContext(t)
	w := workspaceAccessWorld{
		product:       product,
		org:           product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil),
		other:         product.CreateOrganization(t, "Other-"+ids.MustNew("ct"), nil),
		viewWorkspace: "workspace:view",
		readReports:   "report:read",
	}
	w.shared = product.Workspaces(w.org.Id).Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Shared"}).Id
	w.private = product.Workspaces(w.org.Id).Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Private"}).Id
	w.foreign = product.Workspaces(w.other.Id).Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Shared"}).Id
	product.CreateScopedPermissions(t, workspaceType, w.viewWorkspace)
	product.CreateScopedPermissions(t, "organization", w.readReports)
	admin := createRoleTemplate(t, product, ct.CreateProductRoleJSONRequestBody{
		Name: "Admin " + ids.MustNew("ct"), Permissions: []string{w.viewWorkspace, w.readReports},
	})
	member := createRoleTemplate(t, product, ct.CreateProductRoleJSONRequestBody{Name: "Member " + ids.MustNew("ct")})
	w.editor = createRoleTemplate(t, product, rolePermitting(workspaceType, "Workspace editor", w.viewWorkspace))
	w.adminExternalID = w.member(t, admin.Id)
	editorID, editorExternalID := w.memberWithID(t, member.Id)
	w.editorExternalID = editorExternalID
	product.ResourceMembers(w.org.Id, workspaceType, w.shared).Put(editorID, w.editor.Id)
	return w
}

func (w workspaceAccessWorld) member(t *testing.T, roleID string) string {
	t.Helper()
	_, externalID := w.memberWithID(t, roleID)
	return externalID
}

func (w workspaceAccessWorld) memberWithID(t *testing.T, roleID string) (string, string) {
	t.Helper()
	user := createDSLProductUser(t, w.product)
	externalID := "user_" + ids.MustNew("ext")
	setDSLProductUserExternalID(t, w.product, user.ID, externalID)
	addMember(t, w.product, w.org.Id, user.ID, roleID)
	return user.ID, externalID
}

func (w workspaceAccessWorld) ask(externalID, workspaceID string) itdsl.AccessQuestion {
	return w.product.Access().User(externalID).InWorkspace(w.org.Id, workspaceID)
}

func TestAccessEvaluationOnWorkspaceResources(t *testing.T) {
	t.Parallel()
	w := givenWorkspaceAccessWorld(t)

	t.Run("An organization role allows a workspace permission through the workspace's parent", func(t *testing.T) {
		requireAllowed(t, w.ask(w.adminExternalID, w.shared).Can(w.viewWorkspace).Decide())
	})

	t.Run("An organization permission is still decided on the workspace's organization", func(t *testing.T) {
		requireAllowed(t, w.ask(w.adminExternalID, w.private).Can(w.readReports).Decide())
		requireDenied(t, w.ask(w.editorExternalID, w.shared).Can(w.readReports).Decide(), ct.MISSINGPERMISSION)
	})

	t.Run("A role assigned on the workspace allows what it bundles there, and names itself", func(t *testing.T) {
		response := w.ask(w.editorExternalID, w.shared).Can(w.viewWorkspace).Explain().Decide()

		requireAllowed(t, response)
		assert.Equal(t, []ct.AccessGrant{roleGrant(w.editor, workspaceType, w.shared)},
			*response.Evaluations[0].Context.GrantedBy)
	})

	t.Run("A role on one workspace gives nothing on another", func(t *testing.T) {
		requireDenied(t, w.ask(w.editorExternalID, w.private).Can(w.viewWorkspace).Decide(), ct.MISSINGPERMISSION)
	})

	t.Run("A workspace of another organization, or an unknown one, is not in the organization", func(t *testing.T) {
		for _, workspaceID := range []string{w.foreign, ids.MustNew("ws")} {
			requireDenied(t, w.ask(w.adminExternalID, workspaceID).Can(w.viewWorkspace).Decide(),
				ct.WORKSPACENOTINORGANIZATION)
		}
	})
}

func TestDeprecatedWorkspaceRoutes(t *testing.T) {
	t.Parallel()

	t.Run("Every workspace route answers as before, with a deprecation and a successor link", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		workspaces := product.Workspaces(org.Id)
		base := "/v1/products/" + product.ProductID + "/organizations/" + org.Id
		search := "<" + base + `/resources/search>; rel="successor-version"`
		one := func(id string) string {
			return "<" + base + "/resources/workspace/" + id + `>; rel="successor-version"`
		}
		description := "Payments team"

		created := workspaces.CreateRaw(ct.CreateOrganizationWorkspaceJSONRequestBody{
			Name: "Payments", Description: &description,
		})
		require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))
		id := created.JSON201.Id
		assert.True(t, strings.HasPrefix(id, "ws_"))
		assert.Equal(t, org.Id, created.JSON201.OrganizationId)
		assert.Equal(t, "Payments", created.JSON201.Name)
		assert.Equal(t, &description, created.JSON201.Description)
		listed := workspaces.SearchRaw(ct.SearchOrganizationWorkspacesJSONRequestBody{})
		require.Equal(t, http.StatusOK, listed.StatusCode(), string(listed.Body))
		assert.EqualValues(t, 1, listed.JSON200.Total)
		got, err := product.AllScopeAPIKeyClient().GetOrganizationWorkspaceWithResponse(
			t.Context(), product.ProductID, org.Id, id)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, got.StatusCode(), string(got.Body))
		assert.Equal(t, *created.JSON201, *got.JSON200)
		updated := workspaces.UpdateRaw(id, ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: "Billing"})
		require.Equal(t, http.StatusOK, updated.StatusCode(), string(updated.Body))
		assert.Equal(t, "Billing", updated.JSON200.Name)
		deleted := workspaces.DeleteRaw(id)
		require.Equal(t, http.StatusNoContent, deleted.StatusCode(), string(deleted.Body))
		missing := workspaces.DeleteRaw(id)
		require.Equal(t, http.StatusNotFound, missing.StatusCode(), string(missing.Body))

		for name, answer := range map[string]struct {
			response *http.Response
			link     string
		}{
			"create":           {created.HTTPResponse, search},
			"search":           {listed.HTTPResponse, search},
			"get":              {got.HTTPResponse, one(id)},
			"update":           {updated.HTTPResponse, one(id)},
			"delete":           {deleted.HTTPResponse, one(id)},
			"delete not found": {missing.HTTPResponse, one(id)},
		} {
			assert.Equal(t, "@1792886400", answer.response.Header.Get("Deprecation"), name)
			assert.Equal(t, answer.link, answer.response.Header.Get("Link"), name)
		}
	})

	t.Run("A refused call without a credential carries the deprecation too", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		path := "/v1/products/" + product.ProductID + "/organizations/" + org.Id + "/workspaces/search"
		request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, itshared.ServerURL+path,
			strings.NewReader("{}"))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(request)

		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, "@1792886400", resp.Header.Get("Deprecation"))
		assert.Equal(t, "</v1/products/"+product.ProductID+"/organizations/"+org.Id+
			`/resources/search>; rel="successor-version"`, resp.Header.Get("Link"))
	})

	t.Run("Another route carries no deprecation", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)

		resp := product.Resources(org.Id).SearchRaw(ct.SearchResourcesJSONRequestBody{})

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.Empty(t, resp.HTTPResponse.Header.Get("Deprecation"))
	})
}

// answered turns a call that failed or answered another status into an error.
func answered(resp interface{ StatusCode() int }, err error, want int) error {
	if err != nil {
		return err
	}
	if resp.StatusCode() != want {
		return fmt.Errorf("answered %d, want %d", resp.StatusCode(), want)
	}
	return nil
}

// The backfill and the workspace routes write the same references. Whatever
// order their transactions take, every workspace ends registered with its
// own name, and a deleted one ends unregistered.
func TestWorkspaceBackfillRacesWorkspaceRoutes(t *testing.T) {
	t.Parallel()

	t.Run("Workspace writes during a backfill leave every workspace registered as it is", func(t *testing.T) {
		t.Parallel()
		const (
			writers  = 10
			backfill = 5
		)
		product := createTestProductContext(t)
		product.ResourceTypes().CreateUnder(workspaceType, "organization")
		org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		unregistered := make([]string, writers)
		for i := range unregistered {
			unregistered[i] = insertWorkspace(t, org.Id, "Before "+ids.MustNew("ct"))
		}
		client := product.AllScopeAPIKeyClient()
		runner, _ := newWorkspaceBackfill()
		var (
			mu      sync.Mutex
			deleted []string
			group   errgroup.Group
		)
		for range backfill {
			group.Go(func() error { return runner.RunProduct(t.Context(), product.ProductID) })
		}
		for i := range writers {
			group.Go(func() error {
				created, err := client.CreateOrganizationWorkspaceWithResponse(t.Context(), product.ProductID, org.Id,
					ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Created " + ids.MustNew("ct")})
				if err = answered(created, err, http.StatusCreated); err != nil {
					return err
				}
				renamed, err := client.UpdateOrganizationWorkspaceWithResponse(
					t.Context(),
					product.ProductID,
					org.Id,
					unregistered[i],
					ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: "Renamed " + ids.MustNew("ct")},
				)
				if err = answered(renamed, err, http.StatusOK); err != nil || i%2 == 0 {
					return err
				}
				for _, id := range []string{created.JSON201.Id, unregistered[i]} {
					gone, deleteErr := client.DeleteOrganizationWorkspaceWithResponse(
						t.Context(), product.ProductID, org.Id, id)
					if deleteErr = answered(gone, deleteErr, http.StatusNoContent); deleteErr != nil {
						return deleteErr
					}
					mu.Lock()
					deleted = append(deleted, id)
					mu.Unlock()
				}
				return nil
			})
		}
		require.NoError(t, group.Wait())
		require.NoError(t, runner.RunProduct(t.Context(), product.ProductID))

		remaining := product.Workspaces(org.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{
			Pagination: &ct.PaginationRequest{Limit: new(int32(50))},
		})
		assert.Len(t, remaining.Items, writers)
		for _, ws := range remaining.Items {
			requireWorkspaceResource(t, product, org.Id, ws.Id, ws.Name)
		}
		for _, id := range deleted {
			requireNoWorkspaceResource(t, product, org.Id, id)
		}
		registered := product.Resources(org.Id).Search(ct.SearchResourcesJSONRequestBody{
			Filter: &ct.ResourceFilter{ResourceTypes: []string{workspaceType}},
		})
		assert.EqualValues(t, writers, registered.Total)
		waitForSweep(t, product.ProductID)
	})
}
