package service_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/model"
	"anchor/internal/domain/resource"
	"anchor/internal/domain/resourcetype"
)

// mirroredSyncResources is the registry size the sync cost is measured at.
const mirroredSyncResources = 1000

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

func (w registryWorld) put(t *testing.T, resourceType, id string, parent resource.Ref) resource.Resource {
	t.Helper()
	stored, err := ResourceService.Put(t.Context(), resource.PutInput{
		ProductID: w.productID, OrganizationID: w.organizationID, Type: resourceType, ID: id, Name: id, Parent: parent,
	})
	require.NoError(t, err)
	return stored
}

func (w registryWorld) sync(t *testing.T) {
	t.Helper()
	require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
}

// parentsOf reads the parent tuples the store holds on a resource's object.
func parentsOf(t *testing.T, productID string, r resource.Resource) []string {
	t.Helper()
	found, err := AuthzRepo.FindStore(t.Context(), productID)
	require.NoError(t, err)
	resp, err := openFGA(t).Read(t.Context(), &openfgav1.ReadRequest{
		StoreId:  found.Value().StoreID,
		TupleKey: &openfgav1.ReadRequestTupleKey{Object: model.ResourceObject(r.Type, r.RowID)},
	})
	require.NoError(t, err)
	var parents []string
	for _, tuple := range resp.GetTuples() {
		require.Equal(t, model.RelationParent, tuple.GetKey().GetRelation())
		parents = append(parents, tuple.GetKey().GetUser())
	}
	return parents
}

func TestResourceMirror(t *testing.T) {
	openFGA(t)

	t.Run("A registered resource holds one parent tuple to its parent", func(t *testing.T) {
		w := givenRegistryWorld(t)
		project := w.put(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		folder := w.put(t, "folder", uuid.NewString(), project.Ref())

		w.sync(t)

		assert.Equal(t, []string{model.Organization(w.organizationID)}, parentsOf(t, w.productID, project))
		assert.Equal(t, []string{model.ResourceObject("project", project.RowID)}, parentsOf(t, w.productID, folder))
	})

	t.Run("A moved resource holds its new parent tuple and not the old one", func(t *testing.T) {
		w := givenRegistryWorld(t)
		first := w.put(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		second := w.put(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		folder := w.put(t, "folder", uuid.NewString(), first.Ref())
		w.sync(t)

		w.put(t, "folder", folder.ExternalID, second.Ref())
		w.sync(t)

		assert.Equal(t, []string{model.ResourceObject("project", second.RowID)}, parentsOf(t, w.productID, folder))
	})

	t.Run("A deleted resource's tuples stay until the reconcile removes them", func(t *testing.T) {
		w := givenRegistryWorld(t)
		project := w.put(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
		folder := w.put(t, "folder", uuid.NewString(), project.Ref())
		w.sync(t)

		require.NoError(t, ResourceService.Delete(
			t.Context(), w.productID, w.organizationID, "project", project.ExternalID, true))
		w.sync(t)
		assert.Len(t, parentsOf(t, w.productID, folder), 1, "no row names the deleted folder any more")

		require.NoError(t, AuthzSyncer.Reconcile(t.Context(), w.productID))

		assert.Empty(t, parentsOf(t, w.productID, project))
		assert.Empty(t, parentsOf(t, w.productID, folder))
	})
}

// TestOrganizationSyncWithManyResources measures one Organization sync with
// mirroredSyncResources registered resources: the first writes every parent
// tuple, the next reads one object per resource and writes nothing.
func TestOrganizationSyncWithManyResources(t *testing.T) {
	openFGA(t)

	t.Run("An organization with 1,000 registered resources syncs every parent tuple", func(t *testing.T) {
		w := givenRegistryWorld(t)
		rows := givenRegisteredResources(t, w, mirroredSyncResources)

		started := time.Now()
		w.sync(t)
		first := time.Since(started)
		started = time.Now()
		w.sync(t)
		steady := time.Since(started)
		t.Logf("organization sync with %d resources: first %s, unchanged %s", mirroredSyncResources, first, steady)

		for _, r := range []resource.Resource{rows[0], rows[len(rows)/2], rows[len(rows)-1]} {
			assert.Len(t, parentsOf(t, w.productID, r), 1)
		}
	})
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
	const projectsEvery = 10
	rows := make([]resource.Resource, 0, n)
	var projects []string
	var values []string
	var args []any
	for i := range n {
		r := resource.Resource{RowID: resource.NewRowID(), Type: "project", ExternalID: uuid.NewString()}
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
