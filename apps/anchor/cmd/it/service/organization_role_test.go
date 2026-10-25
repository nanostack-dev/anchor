package service_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/product/role"
	orgroleservice "anchor/internal/organizationrole/service"
)

// A release before keys wrote role templates with none. Only the startup
// backfill keys them, and no route reaches it.
func TestRoleTemplateKeyBackfill(t *testing.T) {
	t.Run("A template written without a key gets its name in lower case", func(t *testing.T) {
		ctx := t.Context()
		productID := GivenATenantAndProduct(t).Product.ID
		template := givenUnkeyedTemplate(t, productID, "Legacy Admin "+uuid.NewString())

		require.NoError(
			t,
			orgroleservice.BackfillKeys(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		assert.Equal(t, strings.ToLower(template.Name), templateKey(t, template.ID))
	})

	t.Run("A template whose name another template holds as its key gets its own ID", func(t *testing.T) {
		ctx := t.Context()
		productID := GivenATenantAndProduct(t).Product.ID
		name := "Viewer " + uuid.NewString()
		keyed := role.ProductRole{ProductID: productID, Name: "Reader " + uuid.NewString(), Key: strings.ToLower(name)}
		keyed.GenerateID()
		_, err := ProductRoleRepo.Create(ctx, keyed)
		require.NoError(t, err)
		unkeyed := givenUnkeyedTemplate(t, productID, name)

		require.NoError(
			t,
			orgroleservice.BackfillKeys(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		assert.Equal(t, unkeyed.ID, templateKey(t, unkeyed.ID))
	})

	t.Run("Running the backfill again changes nothing", func(t *testing.T) {
		ctx := t.Context()
		productID := GivenATenantAndProduct(t).Product.ID
		template := givenUnkeyedTemplate(t, productID, "Admin "+uuid.NewString())
		require.NoError(
			t,
			orgroleservice.BackfillKeys(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		require.NoError(
			t,
			orgroleservice.BackfillKeys(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		assert.Equal(t, strings.ToLower(template.Name), templateKey(t, template.ID))
	})
}

// The next release copies every role template into each Organization and
// moves memberships onto the copies. This release runs beside it during that
// deploy, so it reads and writes those rows correctly before it makes any.
func TestOrganizationCopiesWrittenByTheNextRelease(t *testing.T) {
	t.Run("Template searches leave the copies out", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		found, err := ProductRoleRepo.SearchByProductID(t.Context(), fixture.productID,
			search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
				Pagination: search.Pagination{Limit: 10},
			})

		require.NoError(t, err)
		require.Len(t, found.Items, 1)
		assert.Equal(t, fixture.templateID, found.Items[0].ID)
	})

	t.Run("A member on a copy shows the template's ID", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		member, err := MembershipService.GetMember(t.Context(), organization.GetMemberInput{
			ProductID: fixture.productID, OrganizationID: fixture.organizationID, ProductUserID: fixture.userID,
		})

		require.NoError(t, err)
		assert.Equal(t, fixture.templateID, member.RoleID)
	})

	t.Run("A member search by the template's ID finds a member on the copy", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		found, err := MembershipService.SearchMembers(t.Context(), organization.SearchMembersInput{
			ProductID: fixture.productID, OrganizationID: fixture.organizationID,
			Request: search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
				Filter:     &organization.SearchMembersFilter{RoleIDs: []string{fixture.templateID}},
				Pagination: search.Pagination{Limit: 10},
			},
		})

		require.NoError(t, err)
		require.Len(t, found.Items, 1)
		assert.Equal(t, fixture.userID, found.Items[0].ProductUserID)
	})

	t.Run("A template whose copy a member holds can't be deleted", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		err := ProductRoleService.DeleteProductRole(t.Context(), role.DeleteProductRoleInput{
			ProductID: fixture.productID, ID: fixture.templateID,
		})

		assertFaultCode(t, err, "ROLE_IN_USE")
	})

	t.Run("A template ID named in the Organization resolves to its copy", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		resolved, err := OrganizationRoleService.Resolve(
			t.Context(), fixture.productID, fixture.organizationID, fixture.templateID,
		)

		require.NoError(t, err)
		assert.Equal(t, fixture.copyID, resolved.RowID)
	})

	t.Run("A permission removed from the template leaves the copy", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		require.NoError(t, ProductRoleService.UnassignPermissionFromProductRole(t.Context(),
			role.UnassignPermissionFromProductRoleInput{
				ProductID: fixture.productID, ProductRoleID: fixture.templateID, PermissionName: fixture.permission,
			}))

		assert.Empty(t, rolePermissions(t, fixture.copyID))
	})

	t.Run("A permission given to the template reaches the copy", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)
		added := givenPermission(t, fixture.productID)

		require.NoError(t, ProductRoleService.AssignPermissionToProductRole(t.Context(),
			role.AssignPermissionToProductRoleInput{
				ProductID: fixture.productID, ProductRoleID: fixture.templateID, PermissionName: added,
			}))

		assert.ElementsMatch(t, []string{fixture.permission, added}, rolePermissions(t, fixture.copyID))
	})

	t.Run("A renamed template renames the copy", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)
		name := "Renamed " + uuid.NewString()

		_, err := ProductRoleService.UpdateProductRole(t.Context(), role.UpdateProductRoleInput{
			ProductID: fixture.productID, ID: fixture.templateID, Name: &name,
		})

		require.NoError(t, err)
		var copyName string
		require.NoError(t, TestDB.QueryRowContext(t.Context(),
			`SELECT name FROM product_roles WHERE id = $1`, fixture.copyID).Scan(&copyName))
		assert.Equal(t, name, copyName)
	})
}

type copyFixture struct {
	productID      string
	organizationID string
	templateID     string
	copyID         string
	userID         string
	permission     string
}

// givenACopyHeldByAMember writes what the next release writes: a template
// with one permission, its copy in an Organization with the same
// permission, and a member who holds the copy.
func givenACopyHeldByAMember(t *testing.T) copyFixture {
	t.Helper()
	ctx := t.Context()
	productID := GivenATenantAndProduct(t).Product.ID
	permission := givenPermission(t, productID)
	template := role.ProductRole{ProductID: productID, Name: "Editor " + uuid.NewString()}
	template.Key = strings.ToLower(template.Name)
	template.GenerateID()
	_, err := ProductRoleRepo.Create(ctx, template)
	require.NoError(t, err)
	givenRolePermission(t, productID, template.ID, permission)
	org := organization.Organization{ProductID: productID, Name: "Org " + uuid.NewString()}
	org.GenerateID()
	_, err = OrganizationRepo.Create(ctx, org)
	require.NoError(t, err)
	copyID := "product_role_" + uuid.NewString()
	_, err = TestDB.ExecContext(ctx, `INSERT INTO product_roles
		(id, product_id, organization_id, template_id, key, name) VALUES ($1, $2, $3, $4, $5, $6)`,
		copyID, productID, org.ID, template.ID, template.Key, template.Name)
	require.NoError(t, err)
	givenRolePermission(t, productID, copyID, permission)
	userID := givenProductUser(t, productID)
	_, err = TestDB.ExecContext(ctx, `INSERT INTO organization_memberships
		(organization_id, product_user_id, product_role_id) VALUES ($1, $2, $3)`, org.ID, userID, copyID)
	require.NoError(t, err)
	return copyFixture{
		productID: productID, organizationID: org.ID, templateID: template.ID,
		copyID: copyID, userID: userID, permission: permission,
	}
}

func givenUnkeyedTemplate(t *testing.T, productID, name string) role.ProductRole {
	t.Helper()
	template := role.ProductRole{ProductID: productID, Name: name}
	template.GenerateID()
	_, err := ProductRoleRepo.Create(t.Context(), template)
	require.NoError(t, err)
	_, err = TestDB.ExecContext(t.Context(), `UPDATE product_roles SET key = NULL WHERE id = $1`, template.ID)
	require.NoError(t, err)
	return template
}

func givenPermission(t *testing.T, productID string) string {
	t.Helper()
	name := "doc:" + uuid.NewString()
	_, err := TestDB.ExecContext(t.Context(),
		`INSERT INTO product_resource_permissions (product_id, name) VALUES ($1, $2)`, productID, name)
	require.NoError(t, err)
	return name
}

func givenRolePermission(t *testing.T, productID, roleID, permission string) {
	t.Helper()
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO product_role_resource_permissions
		(id, product_id, product_role_id, permission_name) VALUES ($1, $2, $3, $4)`,
		"product_role_resource_permission_"+uuid.NewString(), productID, roleID, permission)
	require.NoError(t, err)
}

func givenProductUser(t *testing.T, productID string) string {
	t.Helper()
	userID := "pusr_" + uuid.NewString()
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO product_users (id, product_id, email, status)
		VALUES ($1, $2, $3, 'ACTIVE')`, userID, productID, uuid.NewString()+"@example.com")
	require.NoError(t, err)
	return userID
}

func templateKey(t *testing.T, templateID string) string {
	t.Helper()
	var key string
	require.NoError(t, TestDB.QueryRowContext(t.Context(),
		`SELECT key FROM product_roles WHERE id = $1`, templateID).Scan(&key))
	return key
}

func rolePermissions(t *testing.T, roleID string) []string {
	t.Helper()
	rows, err := TestDB.QueryContext(t.Context(),
		`SELECT permission_name FROM product_role_resource_permissions WHERE product_role_id = $1`, roleID)
	require.NoError(t, err)
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())
	return names
}

func assertFaultCode(t *testing.T, err error, code string) {
	t.Helper()
	failure, isFault := fault.As(err)
	require.True(t, isFault, "expected a fault, got %v", err)
	require.NotEmpty(t, failure.Details)
	assert.Equal(t, code, failure.Details[0].Code)
}
