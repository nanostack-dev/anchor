package service_test

import (
	"context"
	"sync"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/openfga/openfga/pkg/server"
	"github.com/openfga/openfga/pkg/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/model"
	"anchor/internal/authz/repository"
	"anchor/internal/authz/service"
)

const (
	productID = "prd_acme"
	orgID     = "org_acme"
	otherOrg  = "org_other"
	payments  = "ws_payments"
	ops       = "ws_ops"
	foreignWS = "ws_foreign"
	viewer    = "role_viewer"
	editor    = "role_editor"
	alice     = "usr_alice"
	bob       = "usr_bob"
	ciKey     = "key_ci"
)

func TestAuthorizer(t *testing.T) {
	t.Parallel()

	t.Run("an organization role grants its permissions in the organization and every workspace", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)

		org := w.resolve(w.authorizer.Product(productID).User(alice).InOrganization(orgID))
		assert.True(t, org.Reaches())
		assert.Equal(t, []string{"flows:read"}, org.Permissions)
		assert.Equal(t, viewer, org.OrganizationRole.Value().ID)

		ws := w.resolve(w.authorizer.Product(productID).User(alice).InWorkspace(orgID, ops))
		assert.Equal(t, []string{"flows:read"}, ws.Permissions)
		assert.False(t, ws.WorkspaceRole.IsPresent())
	})

	t.Run("a workspace role adds its permissions in that workspace only", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)

		ws := w.resolve(w.authorizer.Product(productID).User(alice).InWorkspace(orgID, payments))
		assert.Equal(t, []string{"flows:delete", "flows:read", "flows:update"}, ws.Permissions)
		assert.Equal(t, editor, ws.WorkspaceRole.Value().ID)

		other := w.resolve(w.authorizer.Product(productID).User(alice).InWorkspace(orgID, ops))
		assert.False(t, other.Holds("flows:update"))
	})

	t.Run("a product user outside the organization reaches nothing", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)

		access := w.resolve(w.authorizer.Product(productID).User(bob).InWorkspace(orgID, payments))
		assert.True(t, access.OrganizationExists)
		assert.False(t, access.IsMember)
		assert.Empty(t, access.Permissions)
	})

	t.Run("a workspace of another organization is refused", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)

		access := w.resolve(w.authorizer.Product(productID).User(alice).InWorkspace(orgID, foreignWS))
		assert.False(t, access.WorkspaceInOrganization)
		assert.Empty(t, access.Permissions)
	})

	t.Run("an unknown organization does not exist", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)

		access := w.resolve(w.authorizer.Product(productID).User(alice).InOrganization("org_missing"))
		assert.False(t, access.OrganizationExists)
	})

	t.Run("an API key holds its own permissions in its organization and its workspaces", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)

		access := w.resolve(w.authorizer.Product(productID).APIKey(ciKey).InWorkspace(orgID, ops))
		assert.True(t, access.Reaches())
		assert.Equal(t, []string{"flows:delete"}, access.Permissions)
	})

	t.Run("a marked membership removal applies to the next decision", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.resolve(w.authorizer.Product(productID).User(alice).InOrganization(orgID))

		w.repo.update(orgID, func(state *model.OrganizationState) { state.Members = nil })
		w.repo.mark(orgID)

		access := w.resolve(w.authorizer.Product(productID).User(alice).InWorkspace(orgID, payments))
		assert.False(t, access.IsMember)
		assert.Empty(t, access.Permissions, "the workspace role went with the organization membership")
	})

	t.Run("a role permission change rebuilds the model for the whole product", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.resolve(w.authorizer.Product(productID).User(alice).InOrganization(orgID))

		w.repo.catalog.Roles[0].Permissions = []string{"flows:read", "flows:update"}
		w.repo.mark(repository.ProductWide)

		access := w.resolve(w.authorizer.Product(productID).User(alice).InOrganization(orgID))
		assert.Equal(t, []string{"flows:read", "flows:update"}, access.Permissions)
	})

	t.Run("a deleted role takes its assignments with it", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.resolve(w.authorizer.Product(productID).User(alice).InWorkspace(orgID, payments))

		w.repo.catalog.Roles = w.repo.catalog.Roles[:1]
		w.repo.mark(repository.ProductWide)

		access := w.resolve(w.authorizer.Product(productID).User(alice).InWorkspace(orgID, payments))
		assert.Equal(t, []string{"flows:read"}, access.Permissions)
		assert.False(t, access.WorkspaceRole.IsPresent())
	})

	t.Run("an unmarked change is not seen until it is marked", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.resolve(w.authorizer.Product(productID).User(alice).InOrganization(orgID))

		w.repo.update(orgID, func(state *model.OrganizationState) { state.Members = nil })
		stale := w.resolve(w.authorizer.Product(productID).User(alice).InOrganization(orgID))
		assert.True(t, stale.IsMember, "writes reach the store only through marks")

		require.NoError(t, w.syncer.SyncProduct(context.Background(), productID))
		w.repo.mark(orgID)
		require.NoError(t, w.syncer.SyncProduct(context.Background(), productID))
		fresh := w.resolve(w.authorizer.Product(productID).User(alice).InOrganization(orgID))
		assert.False(t, fresh.IsMember)
	})
}

type world struct {
	t          *testing.T
	repo       *fakeRepository
	syncer     *service.Syncer
	authorizer *service.Authorizer
}

// newWorld builds Acme: Alice is an organization viewer and an editor of the
// Payments workspace; Bob is a product user outside Acme; the CI key may
// delete flows; ws_foreign belongs to another organization.
func newWorld(t *testing.T) world {
	t.Helper()
	openfga, err := server.NewServerWithOpts(server.WithDatastore(memory.New()))
	require.NoError(t, err)
	t.Cleanup(openfga.Close)

	repo := &fakeRepository{
		catalog: model.Catalog{
			Roles: []model.Role{
				{ID: viewer, Name: "Viewer", Permissions: []string{"flows:read"}},
				{ID: editor, Name: "Editor", Permissions: []string{"flows:read", "flows:update", "flows:delete"}},
			},
			Permissions: []string{"flows:read", "flows:update", "flows:delete"},
		},
		organizations: map[string]model.OrganizationState{
			orgID: {
				ProductID: productID, OrganizationID: orgID, Exists: true,
				Members: []model.RoleAssignment{{ProductUserID: alice, RoleID: viewer}},
				Workspaces: []model.WorkspaceState{
					{ID: payments, Members: []model.RoleAssignment{
						{ProductUserID: alice, RoleID: editor},
						{ProductUserID: bob, RoleID: editor},
					}},
					{ID: ops},
				},
				APIKeys: []model.APIKeyGrant{{ID: ciKey, Permissions: []string{"flows:delete"}}},
			},
			otherOrg: {
				ProductID: productID, OrganizationID: otherOrg, Exists: true,
				Workspaces: []model.WorkspaceState{{ID: foreignWS}},
			},
		},
		marks: map[string]int64{},
	}
	var lock sync.Mutex
	syncer := service.NewSyncerWithLock(openfga, repo,
		func(ctx context.Context, _ string, fn func(context.Context) error) error {
			lock.Lock()
			defer lock.Unlock()
			return fn(ctx)
		})
	return world{t: t, repo: repo, syncer: syncer, authorizer: service.NewAuthorizer(syncer)}
}

func (w world) resolve(question service.Question) service.Access {
	w.t.Helper()
	access, err := question.Resolve(context.Background())
	require.NoError(w.t, err)
	return access
}

// fakeRepository holds one product in memory.
type fakeRepository struct {
	mu            sync.Mutex
	catalog       model.Catalog
	organizations map[string]model.OrganizationState
	storeID       string
	marks         map[string]int64
}

func (r *fakeRepository) update(organizationID string, change func(*model.OrganizationState)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.organizations[organizationID]
	change(&state)
	r.organizations[organizationID] = state
}

func (r *fakeRepository) mark(organizationID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.marks[organizationID]++
}

func (r *fakeRepository) Catalog(context.Context, string) (model.Catalog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.catalog, nil
}

func (r *fakeRepository) OrganizationState(
	_ context.Context,
	product, organizationID string,
) (model.OrganizationState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if state, ok := r.organizations[organizationID]; ok {
		return state, nil
	}
	return model.OrganizationState{ProductID: product, OrganizationID: organizationID}, nil
}

func (r *fakeRepository) OrganizationIDs(context.Context, string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []string
	for id, state := range r.organizations {
		if state.Exists {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (r *fakeRepository) ProductIDsInternal(context.Context) ([]string, error) {
	return []string{productID}, nil
}

func (r *fakeRepository) FindStoreID(context.Context, string) (functional.Option[string], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return functional.OptionOf(r.storeID, r.storeID != ""), nil
}

func (r *fakeRepository) SaveStoreID(_ context.Context, _, storeID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.storeID == "" {
		r.storeID = storeID
	}
	return r.storeID, nil
}

func (r *fakeRepository) Mark(_ context.Context, _, organizationID string) error {
	r.mark(organizationID)
	return nil
}

func (r *fakeRepository) PendingFor(_ context.Context, product, organizationID string) ([]repository.Mark, error) {
	return r.pending(product, func(id string) bool { return id == repository.ProductWide || id == organizationID }), nil
}

func (r *fakeRepository) PendingForProduct(_ context.Context, product string) ([]repository.Mark, error) {
	return r.pending(product, func(string) bool { return true }), nil
}

func (r *fakeRepository) OldestInternal(_ context.Context, _ int64) ([]repository.Mark, error) {
	return r.pending(productID, func(string) bool { return true }), nil
}

func (r *fakeRepository) pending(product string, keep func(string) bool) []repository.Mark {
	r.mu.Lock()
	defer r.mu.Unlock()
	var marks []repository.Mark
	for id, revision := range r.marks {
		if keep(id) {
			marks = append(marks, repository.Mark{ProductID: product, OrganizationID: id, Revision: revision})
		}
	}
	return marks
}

func (r *fakeRepository) Clear(_ context.Context, mark repository.Mark) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.marks[mark.OrganizationID] == mark.Revision {
		delete(r.marks, mark.OrganizationID)
	}
	return nil
}
