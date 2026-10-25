package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/model"
	authzservice "anchor/internal/authz/service"
	"anchor/internal/domain/resource"
	"anchor/internal/domain/resourcetype"
)

const (
	// mirroredSyncResources is the registry size the sync cost is measured at.
	mirroredSyncResources = 1000
	// projectsEvery puts one project at the top for this many resources.
	projectsEvery = 10
)

// registryWorld is a product whose tree has project under organization and
// folder under project, nesting in itself, with one Organization.
type registryWorld struct {
	productID      string
	organizationID string
}

func givenRegistryWorld(t *testing.T) registryWorld {
	t.Helper()
	productID := GivenATenantAndProduct(t).Product.ID
	for _, rt := range []resourcetype.CreateInput{
		{ProductID: productID, Slug: "project", Name: "Project", ParentTypes: []string{"organization"}},
		{ProductID: productID, Slug: "folder", Name: "Folder", ParentTypes: []string{"project"}, NestsInItself: true},
	} {
		_, err := ResourceTypeService.Create(t.Context(), rt)
		require.NoError(t, err)
	}
	return registryWorld{productID: productID, organizationID: givenOrganization(t, productID)}
}

// putCollected registers a resource the way a request does, and answers the
// resource objects its write marked.
func (w registryWorld) putCollected(
	t *testing.T, resourceType, id string, parent resource.Ref,
) (resource.Resource, []string) {
	t.Helper()
	ctx, _ := authzservice.CollectWrites(t.Context())
	stored, err := ResourceService.Put(ctx, resource.PutInput{
		ProductID: w.productID, OrganizationID: w.organizationID, Type: resourceType, ID: id, Name: id, Parent: parent,
	})
	require.NoError(t, err)
	return stored, w.objects(ctx)
}

func (w registryWorld) deleteCollected(t *testing.T, r resource.Resource, cascade bool) []string {
	t.Helper()
	ctx, _ := authzservice.CollectWrites(t.Context())
	require.NoError(t, ResourceService.Delete(ctx, w.productID, w.organizationID, r.Type, r.ExternalID, cascade))
	return w.objects(ctx)
}

func (w registryWorld) objects(ctx context.Context) []string {
	var objects []string
	for _, ref := range authzservice.WrittenResources(ctx) {
		objects = append(objects, ref.Object)
	}
	return objects
}

// syncResources replays the given resources the way the request's replay
// does.
func (w registryWorld) syncResources(t *testing.T, resources ...resource.Resource) {
	t.Helper()
	require.NoError(t, AuthzSyncer.SyncResources(t.Context(), w.productID, objectsOf(resources...)))
}

func objectsOf(resources ...resource.Resource) []string {
	objects := make([]string, len(resources))
	for i, r := range resources {
		objects[i] = model.ResourceObject(r.Type, r.RowID)
	}
	return objects
}

func (w registryWorld) pendingResources(t *testing.T) []string {
	t.Helper()
	var objects []string
	rows, err := TestDB.QueryContext(t.Context(),
		`SELECT object FROM authz_resource_marks WHERE product_id = $1 ORDER BY object`, w.productID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var object string
		require.NoError(t, rows.Scan(&object))
		objects = append(objects, object)
	}
	require.NoError(t, rows.Err())
	return objects
}

// parentsOf reads the parent tuples the store holds on a resource's object.
func parentsOf(t *testing.T, productID string, r resource.Resource) []string {
	t.Helper()
	var parents []string
	for _, tuple := range tuplesOn(t, productID, model.ResourceObject(r.Type, r.RowID)) {
		require.Equal(t, model.RelationParent, tuple.GetKey().GetRelation())
		parents = append(parents, tuple.GetKey().GetUser())
	}
	return parents
}

func tuplesOn(t *testing.T, productID, object string) []*openfgav1.Tuple {
	t.Helper()
	found, err := AuthzRepo.FindStore(t.Context(), productID)
	require.NoError(t, err)
	stored, ok := found.Get()
	if !ok {
		return nil
	}
	resp, err := openFGA(t).Read(t.Context(), &openfgav1.ReadRequest{
		StoreId: stored.StoreID, TupleKey: &openfgav1.ReadRequestTupleKey{Object: object},
	})
	require.NoError(t, err)
	return resp.GetTuples()
}

func writeTuple(t *testing.T, productID string, tuple *openfgav1.TupleKey) {
	t.Helper()
	found, err := AuthzRepo.FindStore(t.Context(), productID)
	require.NoError(t, err)
	_, err = openFGA(t).Write(t.Context(), &openfgav1.WriteRequest{
		StoreId: found.Value().StoreID, AuthorizationModelId: found.Value().ModelID,
		Writes: &openfgav1.WriteRequestWrites{TupleKeys: []*openfgav1.TupleKey{tuple}},
	})
	require.NoError(t, err)
}

func TestResourceMirror(t *testing.T) {
	openFGA(t)

	t.Run("A registration marks its own object and its sync writes its parent tuple", func(t *testing.T) {
		w := givenRegistryWorld(t)
		project, marked := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		folder, folderMarked := w.putCollected(t, "folder", uuid.NewString(), project.Ref())

		w.syncResources(t, project, folder)

		assert.Equal(t, objectsOf(project), marked)
		assert.Equal(t, objectsOf(folder), folderMarked)
		assert.Equal(t, []string{model.Organization(w.organizationID)}, parentsOf(t, w.productID, project))
		assert.Equal(t, objectsOf(project), parentsOf(t, w.productID, folder))
		assert.Empty(t, w.pendingResources(t))
	})

	t.Run("A move marks and rewrites only the moved resource", func(t *testing.T) {
		w := givenRegistryWorld(t)
		first, _ := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		second, _ := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		folder, _ := w.putCollected(t, "folder", uuid.NewString(), first.Ref())
		child, _ := w.putCollected(t, "folder", uuid.NewString(), folder.Ref())
		w.syncResources(t, first, second, folder, child)

		_, marked := w.putCollected(t, "folder", folder.ExternalID, second.Ref())
		w.syncResources(t, folder)

		assert.Equal(t, objectsOf(folder), marked)
		assert.Equal(t, objectsOf(second), parentsOf(t, w.productID, folder))
		assert.Equal(t, objectsOf(folder), parentsOf(t, w.productID, child))
	})

	t.Run("A delete marks the deleted resource and its sync drops its tuples", func(t *testing.T) {
		w := givenRegistryWorld(t)
		project, _ := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		folder, _ := w.putCollected(t, "folder", uuid.NewString(), project.Ref())
		w.syncResources(t, project, folder)

		marked := w.deleteCollected(t, folder, false)
		w.syncResources(t, folder)

		assert.Equal(t, objectsOf(folder), marked)
		assert.Empty(t, parentsOf(t, w.productID, folder))
		assert.Len(t, parentsOf(t, w.productID, project), 1)
	})

	t.Run("A cascade marks every deleted resource once, deepest first", func(t *testing.T) {
		w := givenRegistryWorld(t)
		project, _ := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		design, _ := w.putCollected(t, "folder", uuid.NewString(), project.Ref())
		specs, _ := w.putCollected(t, "folder", uuid.NewString(), design.Ref())
		other, _ := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		w.syncResources(t, project, design, specs, other)

		marked := w.deleteCollected(t, project, true)
		w.syncResources(t, project, design, specs)

		assert.Equal(t, objectsOf(specs, design, project), marked)
		for _, gone := range []resource.Resource{project, design, specs} {
			assert.Empty(t, parentsOf(t, w.productID, gone))
		}
		assert.Len(t, parentsOf(t, w.productID, other), 1)
	})

	t.Run("An unchanged registration emits nothing and leaves no mark", func(t *testing.T) {
		w := givenRegistryWorld(t)
		project, _ := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		w.syncResources(t, project)

		_, marked := w.putCollected(t, "project", project.ExternalID, resource.OrganizationRef(w.organizationID))

		assert.Empty(t, marked)
		assert.Empty(t, w.pendingResources(t))
	})

	t.Run("An organization sync neither reads nor writes resource tuples", func(t *testing.T) {
		w := givenRegistryWorld(t)
		project, _ := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		assert.Empty(t, parentsOf(t, w.productID, project), "the organization sync wrote no resource tuple")
		w.syncResources(t, project)
		stray := &openfgav1.TupleKey{
			Object: model.ResourceObject("project", project.RowID), Relation: model.RelationParent,
			User: model.Organization(givenOrganization(t, w.productID)),
		}
		writeTuple(t, w.productID, stray)

		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		assert.Len(t, parentsOf(t, w.productID, project), 2, "the organization sync read no resource object")
		w.syncResources(t, project)

		assert.Equal(t, []string{model.Organization(w.organizationID)}, parentsOf(t, w.productID, project))
	})

	t.Run("The sweeper replays a resource mark the request did not", func(t *testing.T) {
		w := givenRegistryWorld(t)
		project, _ := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))

		require.Eventually(t, func() bool {
			AuthzSweeper.Sweep(t.Context())
			return len(w.pendingResources(t)) == 0
		}, 30*time.Second, 50*time.Millisecond)

		assert.Equal(t, []string{model.Organization(w.organizationID)}, parentsOf(t, w.productID, project))
	})

	t.Run("A deleted organization's resource tuples stay until the reconcile removes them", func(t *testing.T) {
		w := givenRegistryWorld(t)
		project, _ := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		folder, _ := w.putCollected(t, "folder", uuid.NewString(), project.Ref())
		w.syncResources(t, project, folder)
		_, err := TestDB.ExecContext(t.Context(), `DELETE FROM organizations WHERE id = $1`, w.organizationID)
		require.NoError(t, err)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		assert.Len(t, parentsOf(t, w.productID, folder), 1, "the cascade left the tuples")

		require.NoError(t, AuthzSyncer.Reconcile(t.Context(), w.productID))

		assert.Empty(t, parentsOf(t, w.productID, project))
		assert.Empty(t, parentsOf(t, w.productID, folder))
	})

	t.Run("The reconcile writes a resource tuple no replay wrote", func(t *testing.T) {
		w := givenRegistryWorld(t)
		rows := givenRegisteredResources(t, w, projectsEvery)

		require.NoError(t, AuthzSyncer.Reconcile(t.Context(), w.productID))

		for _, r := range rows {
			assert.Len(t, parentsOf(t, w.productID, r), 1)
		}
	})
}

// TestOrganizationSyncWithManyResources measures, with mirroredSyncResources
// registered resources, an Organization sync, which reads no resource, and
// the replay of one resource write, which reads its own object only.
func TestOrganizationSyncWithManyResources(t *testing.T) {
	openFGA(t)

	for _, size := range []int{projectsEvery, mirroredSyncResources} {
		t.Run(
			fmt.Sprintf("A resource write's replay reads its own object among %d resources", size),
			func(t *testing.T) {
				w := givenRegistryWorld(t)
				rows := givenRegisteredResources(t, w, size)
				require.NoError(t, AuthzSyncer.Reconcile(t.Context(), w.productID))

				started := time.Now()
				require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
				organization := time.Since(started)
				folder, _ := w.putCollected(t, "folder", uuid.NewString(), rows[0].Ref())
				started = time.Now()
				w.syncResources(t, folder)
				replay := time.Since(started)
				t.Logf("with %d registered resources: organization sync %s, one resource write's replay %s",
					size, organization, replay)

				assert.Equal(t, objectsOf(rows[0]), parentsOf(t, w.productID, folder))
				for _, r := range []resource.Resource{rows[0], rows[len(rows)/2], rows[len(rows)-1]} {
					assert.Len(t, parentsOf(t, w.productID, r), 1)
				}
			},
		)
	}
}

// givenRegisteredResources inserts n resources straight into the table: a
// tenth are projects at the top, the rest folders spread under them.
func givenRegisteredResources(t *testing.T, w registryWorld, n int) []resource.Resource {
	t.Helper()
	typeIDs := map[string]string{}
	for _, slug := range []string{"project", "folder"} {
		var id string
		require.NoError(t, TestDB.QueryRowContext(t.Context(),
			`SELECT id FROM product_resource_types WHERE product_id = $1 AND slug = $2`, w.productID, slug).Scan(&id))
		typeIDs[slug] = id
	}
	rows := make([]resource.Resource, 0, n)
	var projects []string
	var values []string
	var args []any
	for i := range n {
		r := resource.Resource{
			RowID: resource.NewRowID(), OrganizationID: w.organizationID, Type: "project", ExternalID: uuid.NewString(),
		}
		var parent any
		if i%projectsEvery == 0 {
			projects = append(projects, r.RowID)
		} else {
			r.Type = "folder"
			parent = projects[len(projects)-1]
		}
		rows = append(rows, r)
		base := len(args)
		values = append(values, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7))
		args = append(args, r.RowID, w.productID, w.organizationID, typeIDs[r.Type], r.ExternalID, r.ExternalID, parent)
	}
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO resources
		(id, product_id, organization_id, resource_type_id, external_id, name, parent_resource_id)
		VALUES `+strings.Join(values, ", "), args...)
	require.NoError(t, err)
	return rows
}
