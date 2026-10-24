package service_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/openfga/openfga/pkg/server"
	"github.com/openfga/openfga/pkg/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/engine"
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

		org := w.resolve(w.alice().InOrganization(orgID))
		assert.True(t, org.Reaches())
		assert.Equal(t, []string{"flows:read"}, org.Permissions)
		assert.Equal(t, viewer, org.OrganizationRoleID.Value())

		ws := w.resolve(w.alice().InWorkspace(orgID, ops))
		assert.Equal(t, []string{"flows:read"}, ws.Permissions)
		assert.False(t, ws.WorkspaceRoleID.IsPresent())
	})

	t.Run("a workspace role adds its permissions in that workspace only", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)

		ws := w.resolve(w.alice().InWorkspace(orgID, payments))
		assert.Equal(t, []string{"flows:delete", "flows:read", "flows:update"}, ws.Permissions)
		assert.Equal(t, editor, ws.WorkspaceRoleID.Value())

		other := w.resolve(w.alice().InWorkspace(orgID, ops))
		assert.False(t, other.Holds("flows:update"))
	})

	t.Run("a product user outside the organization reaches nothing", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)

		access := w.resolve(w.syncerAuthorizer().Product(productID).User(bob).InWorkspace(orgID, payments))
		assert.True(t, access.OrganizationExists)
		assert.False(t, access.IsMember)
		assert.Empty(t, access.Permissions)
	})

	t.Run("a workspace of another organization is refused", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)

		access := w.resolve(w.alice().InWorkspace(orgID, foreignWS))
		assert.False(t, access.WorkspaceInOrganization)
		assert.Empty(t, access.Permissions)
	})

	t.Run("an unknown organization does not exist", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)

		assert.False(t, w.resolve(w.alice().InOrganization("org_missing")).OrganizationExists)
	})

	t.Run("an API key holds its own permissions in its organization and its workspaces", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)

		access := w.resolve(w.syncerAuthorizer().Product(productID).APIKey(ciKey).InWorkspace(orgID, ops))
		assert.True(t, access.Reaches())
		assert.Equal(t, []string{"flows:delete"}, access.Permissions)
	})

	t.Run("more permissions than one batch check allows are all checked", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		many := make([]string, 0, 120)
		for i := range 120 {
			many = append(many, fmt.Sprintf("resource%03d:read", i))
		}
		w.repo.editCatalog(func(catalog *model.Catalog) {
			catalog.Permissions = append(catalog.Permissions, many...)
			catalog.Roles[0].Permissions = append(catalog.Roles[0].Permissions, many...)
		})

		access := w.resolve(w.alice().InOrganization(orgID))
		assert.Len(t, access.Permissions, 121)
	})

	t.Run("a marked membership removal applies to the next decision", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.resolve(w.alice().InOrganization(orgID))

		w.repo.updateAcme(func(state *model.OrganizationState) { state.Members = nil })
		w.repo.mark(orgID)

		access := w.resolve(w.alice().InWorkspace(orgID, payments))
		assert.False(t, access.IsMember)
		assert.Empty(t, access.Permissions, "the workspace role went with the organization membership")
	})

	t.Run("a role permission change rebuilds the model without rewriting tuples", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.resolve(w.alice().InOrganization(orgID))

		w.repo.editCatalog(func(catalog *model.Catalog) {
			catalog.Roles[0].Permissions = []string{"flows:read", "flows:update"}
		})
		w.repo.mark(repository.ModelScope)

		access := w.resolve(w.alice().InOrganization(orgID))
		assert.Equal(t, []string{"flows:read", "flows:update"}, access.Permissions)
		assert.Equal(t, 0, w.repo.stateReads(orgID)-1, "only the first decision read the organization")
	})

	t.Run("a deleted role grants nothing at once and its tuples go on the product sweep", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.resolve(w.alice().InWorkspace(orgID, payments))

		w.repo.editCatalog(func(catalog *model.Catalog) { catalog.Roles = catalog.Roles[:1] })
		w.repo.updateAcme(func(state *model.OrganizationState) { state.Workspaces[0].Members = nil })
		w.repo.mark(repository.ModelScope)
		w.repo.mark(repository.ProductScope)

		access := w.resolve(w.alice().InWorkspace(orgID, payments))
		assert.Equal(t, []string{"flows:read"}, access.Permissions)
		assert.False(t, access.WorkspaceRoleID.IsPresent())
		assert.NotEmpty(t, w.tuplesOf(model.Workspace(payments), model.RoleRelation(editor)), "left for the sweep")

		require.NoError(t, w.syncer.SyncProduct(context.Background(), productID))
		assert.Empty(t, w.tuplesOf(model.Workspace(payments), model.RoleRelation(editor)))
	})

	t.Run(
		"a replica that read the catalog before a revocation never makes its old model the latest",
		func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			w.resolve(w.alice().InOrganization(orgID))
			w.repo.editCatalog(
				func(catalog *model.Catalog) { catalog.Roles[0].Permissions = []string{"flows:read", "flows:delete"} },
			)
			w.repo.mark(repository.ModelScope)
			require.True(t, w.resolve(w.alice().InOrganization(orgID)).Holds("flows:delete"))

			// Replica B starts cold and stalls right after reading the catalog.
			stalled := &stallingRepository{
				fakeRepository: w.repo,
				release:        make(chan struct{}),
				read:           make(chan struct{}),
			}
			replicaB := service.NewAuthorizer(newSyncer(w.engine, stalled))
			decided := make(chan service.Access)
			go func() {
				access, err := replicaB.Product(productID).
					User(alice).
					InOrganization(orgID).
					Resolve(context.Background())
				assert.NoError(t, err)
				decided <- access
			}()
			<-stalled.read

			// Meanwhile an admin revokes flows:delete and replica A syncs it.
			w.repo.editCatalog(func(catalog *model.Catalog) { catalog.Roles[0].Permissions = []string{"flows:read"} })
			w.repo.mark(repository.ModelScope)
			require.False(t, w.resolve(w.alice().InOrganization(orgID)).Holds("flows:delete"))

			close(stalled.release)
			<-decided
			for range 3 {
				access, err := replicaB.Product(productID).
					User(alice).
					InOrganization(orgID).
					Resolve(context.Background())
				require.NoError(t, err)
				assert.False(t, access.Holds("flows:delete"), "the revocation holds on replica B")
			}
			assert.False(t, w.resolve(w.alice().InOrganization(orgID)).Holds("flows:delete"), "and on replica A")
		},
	)

	t.Run("a store created by a sync that never recorded it is adopted, not duplicated", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		orphan, err := w.engine.CreateStore(context.Background(), &openfgav1.CreateStoreRequest{Name: productID})
		require.NoError(t, err)

		w.resolve(w.alice().InOrganization(orgID))

		assert.Equal(t, orphan.GetId(), w.repo.recordedStore())
		stores, err := w.engine.ListStores(context.Background(), &openfgav1.ListStoresRequest{})
		require.NoError(t, err)
		assert.Len(t, stores.GetStores(), 1)
	})

	t.Run("a reconcile repairs a change no mark announced", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.resolve(w.alice().InOrganization(orgID))

		w.repo.updateAcme(func(state *model.OrganizationState) { state.Members = nil })
		assert.True(t, w.resolve(w.alice().InOrganization(orgID)).IsMember, "writes reach the store through marks")

		require.NoError(t, w.syncer.Reconcile(context.Background(), productID))
		assert.False(t, w.resolve(w.alice().InOrganization(orgID)).IsMember)
	})

	t.Run("a decision waits for another sync of the product instead of failing", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.resolve(w.alice().InOrganization(orgID))
		w.repo.updateAcme(func(state *model.OrganizationState) { state.Members = nil })
		w.repo.mark(orgID)

		w.lock.Lock()
		go func() {
			time.Sleep(50 * time.Millisecond)
			w.lock.Unlock()
		}()
		assert.False(t, w.resolve(w.alice().InOrganization(orgID)).IsMember)
	})
}

type world struct {
	t      *testing.T
	engine engine.Engine
	repo   *fakeRepository
	lock   *sync.Mutex
	syncer *service.Syncer
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
				{ID: viewer, Permissions: []string{"flows:read"}},
				{ID: editor, Permissions: []string{"flows:read", "flows:update", "flows:delete"}},
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
		marks:      map[string]int64{},
		readCounts: map[string]int{},
	}
	lock := &sync.Mutex{}
	syncer := service.NewSyncerWithLock(openfga, repo,
		func(ctx context.Context, _ string, fn func(context.Context) error) (bool, error) {
			if !lock.TryLock() {
				return false, nil
			}
			defer lock.Unlock()
			return true, fn(ctx)
		})
	return world{t: t, engine: openfga, repo: repo, lock: lock, syncer: syncer}
}

// newSyncer is another replica: its own caches, the same store and Postgres.
func newSyncer(openfga engine.Engine, repo repository.Repository) *service.Syncer {
	var lock sync.Mutex
	return service.NewSyncerWithLock(openfga, repo,
		func(ctx context.Context, _ string, fn func(context.Context) error) (bool, error) {
			lock.Lock()
			defer lock.Unlock()
			return true, fn(ctx)
		})
}

func (w world) syncerAuthorizer() *service.Authorizer { return service.NewAuthorizer(w.syncer) }

func (w world) alice() service.SubjectQuestion {
	return w.syncerAuthorizer().Product(productID).User(alice)
}

func (w world) resolve(question service.Question) service.Access {
	w.t.Helper()
	access, err := question.Resolve(context.Background())
	require.NoError(w.t, err)
	return access
}

func (w world) tuplesOf(object, relation string) []*openfgav1.Tuple {
	w.t.Helper()
	read, err := w.engine.Read(context.Background(), &openfgav1.ReadRequest{
		StoreId:  w.repo.recordedStore(),
		TupleKey: &openfgav1.ReadRequestTupleKey{Object: object, Relation: relation},
	})
	require.NoError(w.t, err)
	return read.GetTuples()
}

// stallingRepository pauses its first catalog read until released.
type stallingRepository struct {
	*fakeRepository
	once    sync.Once
	read    chan struct{}
	release chan struct{}
}

func (r *stallingRepository) Catalog(ctx context.Context, product string) (model.Catalog, error) {
	catalog, err := r.fakeRepository.Catalog(ctx, product)
	r.once.Do(func() {
		close(r.read)
		<-r.release
	})
	return catalog, err
}

// fakeRepository holds one product in memory. It returns copies, as Postgres
// would, so a test's later edits never leak into what a syncer already read.
type fakeRepository struct {
	mu            sync.Mutex
	catalog       model.Catalog
	organizations map[string]model.OrganizationState
	storeID       string
	marks         map[string]int64
	readCounts    map[string]int
}

func (r *fakeRepository) updateAcme(change func(*model.OrganizationState)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := cloneState(r.organizations[orgID])
	change(&state)
	r.organizations[orgID] = state
}

func (r *fakeRepository) editCatalog(change func(*model.Catalog)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	catalog := cloneCatalog(r.catalog)
	change(&catalog)
	r.catalog = catalog
}

func (r *fakeRepository) mark(scope string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.marks[scope]++
}

func (r *fakeRepository) recordedStore() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.storeID
}

func (r *fakeRepository) Catalog(context.Context, string) (model.Catalog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneCatalog(r.catalog), nil
}

func (r *fakeRepository) OrganizationState(
	_ context.Context,
	product, organizationID string,
) (model.OrganizationState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.readCounts[organizationID]++
	if state, ok := r.organizations[organizationID]; ok {
		return cloneState(state), nil
	}
	return model.OrganizationState{ProductID: product, OrganizationID: organizationID}, nil
}

func (r *fakeRepository) stateReads(organizationID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.readCounts[organizationID]
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

func (r *fakeRepository) ProductIDsWithoutStoreInternal(context.Context) ([]string, error) {
	if r.recordedStore() == "" {
		return []string{productID}, nil
	}
	return nil, nil
}

func (r *fakeRepository) DueForReconcileInternal(context.Context, time.Time) (functional.Option[string], error) {
	return functional.None[string](), nil
}

func (r *fakeRepository) Reconciled(context.Context, string) error { return nil }

func (r *fakeRepository) FindStoreID(context.Context, string) (functional.Option[string], error) {
	storeID := r.recordedStore()
	return functional.OptionOf(storeID, storeID != ""), nil
}

func (r *fakeRepository) SaveStoreID(_ context.Context, _, storeID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.storeID == "" {
		r.storeID = storeID
	}
	return r.storeID, nil
}

func (r *fakeRepository) Mark(_ context.Context, _, scope string) error {
	r.mark(scope)
	return nil
}

func (r *fakeRepository) PendingFor(_ context.Context, product, organizationID string) ([]repository.Mark, error) {
	return r.pending(product, func(scope string) bool {
		return scope == repository.ModelScope || scope == organizationID
	}), nil
}

func (r *fakeRepository) PendingForProduct(_ context.Context, product string) ([]repository.Mark, error) {
	return r.pending(product, func(string) bool { return true }), nil
}

func (r *fakeRepository) OldestInternal(context.Context, int64) ([]repository.Mark, error) {
	return r.pending(productID, func(string) bool { return true }), nil
}

func (r *fakeRepository) pending(product string, keep func(string) bool) []repository.Mark {
	r.mu.Lock()
	defer r.mu.Unlock()
	var marks []repository.Mark
	for scope, revision := range r.marks {
		if keep(scope) {
			marks = append(marks, repository.Mark{ProductID: product, OrganizationID: scope, Revision: revision})
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

func cloneCatalog(catalog model.Catalog) model.Catalog {
	roles := make([]model.Role, len(catalog.Roles))
	for i, role := range catalog.Roles {
		roles[i] = model.Role{ID: role.ID, Permissions: slices.Clone(role.Permissions)}
	}
	return model.Catalog{Roles: roles, Permissions: slices.Clone(catalog.Permissions)}
}

func cloneState(state model.OrganizationState) model.OrganizationState {
	state.Members = slices.Clone(state.Members)
	state.APIKeys = slices.Clone(state.APIKeys)
	workspaces := make([]model.WorkspaceState, len(state.Workspaces))
	for i, workspace := range state.Workspaces {
		workspaces[i] = model.WorkspaceState{ID: workspace.ID, Members: slices.Clone(workspace.Members)}
	}
	state.Workspaces = workspaces
	return state
}
