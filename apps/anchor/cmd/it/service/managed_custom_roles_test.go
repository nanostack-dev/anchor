package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/organizationinvitation"
	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/domain/product/role"
	"anchor/internal/domain/resourcetype"
	rtrepository "anchor/internal/resourcetype/repository"
)

// Every release writes a role type, so the database refuses a role without one.
func TestRoleWithoutAType(t *testing.T) {
	t.Run("A role write without a type fails", func(t *testing.T) {
		productID := GivenATenantAndProduct(t).Product.ID
		managed := role.NewManaged(productID, "Legacy "+uuid.NewString(), "")
		managed.ResourceTypeID = organizationTypeID(t, productID)
		_, err := ProductRoleRepo.Create(t.Context(), managed)
		require.NoError(t, err)

		_, err = TestDB.ExecContext(t.Context(), `UPDATE product_roles SET type = NULL WHERE id = $1`, managed.ID)

		require.ErrorContains(t, err, "not-null")
	})
}

// The release before the custom roles switch writes resource types without it.
func TestResourceTypeWrittenWithoutTheSwitch(t *testing.T) {
	t.Run("A resource type with no switch stored allows no custom roles", func(t *testing.T) {
		productID := GivenATenantAndProduct(t).Product.ID
		typeID := organizationTypeID(t, productID)
		_, err := TestDB.ExecContext(t.Context(),
			`UPDATE product_resource_types SET allows_custom_roles = NULL WHERE id = $1`, typeID)
		require.NoError(t, err)

		types, err := ResourceTypeRepo.FindTree(t.Context(), productID)

		require.NoError(t, err)
		root, found := resourcetype.NewTree(types).Organization().Get()
		require.True(t, found)
		assert.False(t, root.AllowsCustomRoles)
	})
}

// The next releases write custom roles, owned by one Organization. This
// release runs beside them during their deploy, so it reads them correctly
// before it makes any.
func TestCustomRolesWrittenByTheNextRelease(t *testing.T) {
	t.Run("A product role search leaves custom roles out", func(t *testing.T) {
		fixture := givenACustomRole(t)

		found, err := ProductRoleRepo.SearchByProductID(t.Context(), fixture.productID,
			search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
				Pagination: search.Pagination{Limit: 10},
			})

		require.NoError(t, err)
		require.Len(t, found.Items, 1)
		assert.Equal(t, fixture.managedID, found.Items[0].ID)
		assert.Equal(t, role.TypeManaged, found.Items[0].Type)
	})

	t.Run("A product role read answers not found for a custom role", func(t *testing.T) {
		fixture := givenACustomRole(t)

		_, err := ProductRoleService.GetProductRole(t.Context(), role.GetProductRoleInput{
			ProductID: fixture.productID, ID: fixture.customID,
		})

		assertFaultCode(t, err, "ROLE_NOT_FOUND")
	})

	t.Run("A product role delete answers not found for a custom role", func(t *testing.T) {
		fixture := givenACustomRole(t)

		err := ProductRoleService.DeleteProductRole(t.Context(), role.DeleteProductRoleInput{
			ProductID: fixture.productID, ID: fixture.customID,
		})

		assertFaultCode(t, err, "ROLE_NOT_FOUND")
	})

	t.Run("A member of the Organization may hold its custom role", func(t *testing.T) {
		fixture := givenACustomRole(t)

		member, err := MembershipService.AddMember(t.Context(), organization.AddMemberInput{
			ProductID: fixture.productID, OrganizationID: fixture.ownerOrgID,
			ProductUserID: givenProductUser(t, fixture.productID), RoleID: &fixture.customID,
		})

		require.NoError(t, err)
		require.NotNil(t, member.Role)
		assert.Equal(t, fixture.customID, member.Role.ID)
	})

	t.Run("A member added in another Organization may not hold the custom role", func(t *testing.T) {
		fixture := givenACustomRole(t)

		_, err := MembershipService.AddMember(t.Context(), organization.AddMemberInput{
			ProductID: fixture.productID, OrganizationID: fixture.otherOrgID,
			ProductUserID: givenProductUser(t, fixture.productID), RoleID: &fixture.customID,
		})

		assertFaultCode(t, err, "ROLE_NOT_FOUND_IN_REQUEST")
	})

	t.Run("A member of another Organization may not be moved onto the custom role", func(t *testing.T) {
		fixture := givenACustomRole(t)
		userID := givenProductUser(t, fixture.productID)
		_, err := MembershipService.AddMember(t.Context(), organization.AddMemberInput{
			ProductID: fixture.productID, OrganizationID: fixture.otherOrgID,
			ProductUserID: userID, RoleID: &fixture.managedID,
		})
		require.NoError(t, err)

		_, err = MembershipService.UpdateMemberRole(t.Context(), organization.UpdateMemberRoleInput{
			ProductID: fixture.productID, OrganizationID: fixture.otherOrgID,
			ProductUserID: userID, RoleID: &fixture.customID,
		})

		assertFaultCode(t, err, "ROLE_NOT_FOUND_IN_REQUEST")
	})

	t.Run("An invitation to another Organization may not name the custom role", func(t *testing.T) {
		fixture := givenACustomRole(t)

		_, err := InvitationService.Create(t.Context(), organizationinvitation.CreateInput{
			ProductID: fixture.productID, OrganizationID: fixture.otherOrgID,
			Email: uuid.NewString() + "@example.com", RoleID: &fixture.customID,
		})

		assertFaultCode(t, err, "ROLE_NOT_FOUND_IN_REQUEST")
	})

	t.Run("An invitation to another Organization may not be moved onto the custom role", func(t *testing.T) {
		fixture := givenACustomRole(t)
		invitation, err := InvitationService.Create(t.Context(), organizationinvitation.CreateInput{
			ProductID: fixture.productID, OrganizationID: fixture.otherOrgID,
			Email: uuid.NewString() + "@example.com", RoleID: &fixture.managedID,
		})
		require.NoError(t, err)

		_, err = InvitationService.Update(t.Context(), organizationinvitation.UpdateInput{
			ProductID: fixture.productID, OrganizationID: fixture.otherOrgID, InvitationID: invitation.ID,
			RoleID: &fixture.customID, ExpiresAt: time.Now().Add(24 * time.Hour),
		})

		assertFaultCode(t, err, "ROLE_NOT_FOUND_IN_REQUEST")
	})

	t.Run("A new Organization's founding member may not hold a custom role", func(t *testing.T) {
		fixture := givenACustomRole(t)

		_, err := OrganizationService.CreateWithMember(t.Context(), organization.CreateOrganizationWithMemberInput{
			ProductID: fixture.productID, Name: "Org " + uuid.NewString()[:8],
			ProductUserID: givenProductUser(t, fixture.productID), RoleID: &fixture.customID,
		})

		assertFaultCode(t, err, "ROLE_NOT_FOUND_IN_REQUEST")
	})

	t.Run("A custom role is not counted as a use of its type, and is listed apart, unheld", func(t *testing.T) {
		fixture := givenACustomRole(t)

		usage, err := ResourceTypeRepo.CountUsage(t.Context(), fixture.productID, fixture.scopeID)
		require.NoError(t, err)
		var custom []rtrepository.CustomRole
		err = Transactor.InTx(t.Context(), func(txCtx context.Context) error {
			custom, err = ResourceTypeRepo.LockCustomRoles(txCtx, fixture.productID, fixture.scopeID)
			return err
		})

		require.NoError(t, err)
		assert.Equal(t, 1, usage.Roles)
		assert.Equal(t, []rtrepository.CustomRole{
			{ID: fixture.customID, OrganizationID: fixture.ownerOrgID, Held: false},
		}, custom)
	})

	t.Run("A deleted permission leaves the custom roles that bundle it", func(t *testing.T) {
		fixture := givenACustomRole(t)

		require.NoError(t, ResourcePermissionSvc.Delete(t.Context(),
			resourcepermission.DeleteProductResourcePermissionInput{
				ProductID: fixture.productID, Name: fixture.permission,
			}))

		assert.Empty(t, rolePermissions(t, fixture.customID))
	})
}

// The next release adds members and invitations with no role. This release
// runs beside it during that deploy, so it reads them.
func TestMembersWithoutARoleWrittenByTheNextRelease(t *testing.T) {
	t.Run("A member with no role still lists", func(t *testing.T) {
		fixture := givenAMemberWithoutARole(t)

		found, err := MembershipService.SearchMembers(t.Context(), organization.SearchMembersInput{
			ProductID: fixture.productID, OrganizationID: fixture.organizationID,
			Request: search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
				Pagination: search.Pagination{Limit: 10},
			},
		})

		require.NoError(t, err)
		require.Len(t, found.Items, 1)
		assert.Equal(t, fixture.userID, found.Items[0].ProductUserID)
		assert.Nil(t, found.Items[0].Role)
	})

	t.Run("A member with no role holds no permission", func(t *testing.T) {
		fixture := givenAMemberWithoutARole(t)

		member, err := MembershipService.GetMember(t.Context(), organization.GetMemberInput{
			ProductID: fixture.productID, OrganizationID: fixture.organizationID,
			ProductUserID: fixture.userID, IncludePermissions: true,
		})

		require.NoError(t, err)
		assert.Nil(t, member.Role)
		assert.Empty(t, member.RolePermissions)
	})

	t.Run("An invitation with no role reads", func(t *testing.T) {
		fixture := givenAMemberWithoutARole(t)
		invitationID := "oinv_" + uuid.NewString()
		_, err := TestDB.ExecContext(t.Context(), `INSERT INTO organization_invitations
			(id, product_id, organization_id, email, expires_at) VALUES ($1, $2, $3, $4, NOW() + INTERVAL '1 day')`,
			invitationID, fixture.productID, fixture.organizationID, uuid.NewString()+"@example.com")
		require.NoError(t, err)

		invitation, err := InvitationService.Get(t.Context(), organizationinvitation.GetInput{
			ProductID: fixture.productID, OrganizationID: fixture.organizationID, InvitationID: invitationID,
		})

		require.NoError(t, err)
		assert.Nil(t, invitation.RoleID)
	})
}

type customRoleFixture struct {
	productID  string
	scopeID    string
	managedID  string
	customID   string
	ownerOrgID string
	otherOrgID string
	permission string
}

// givenACustomRole writes what the next releases write: a managed role, and a
// custom role of one Organization that bundles a permission, beside a second
// Organization of the same Product.
func givenACustomRole(t *testing.T) customRoleFixture {
	t.Helper()
	ctx := t.Context()
	productID := GivenATenantAndProduct(t).Product.ID
	scopeID := organizationTypeID(t, productID)
	managed := role.NewManaged(productID, "Viewer "+uuid.NewString(), "")
	managed.ResourceTypeID = scopeID
	_, err := ProductRoleRepo.Create(ctx, managed)
	require.NoError(t, err)
	owner := givenOrganization(t, productID)
	custom := role.NewCustom(productID, owner, "Auditor "+uuid.NewString(), "")
	_, err = TestDB.ExecContext(ctx, `INSERT INTO product_roles
		(id, product_id, organization_id, type, name, resource_type_id) VALUES ($1, $2, $3, $4, $5, $6)`,
		custom.ID, productID, owner, string(custom.Type), custom.Name, scopeID)
	require.NoError(t, err)
	permission := givenPermission(t, productID)
	givenRolePermission(t, productID, custom.ID, permission)
	return customRoleFixture{
		productID: productID, scopeID: scopeID, managedID: managed.ID, customID: custom.ID,
		ownerOrgID: owner, otherOrgID: givenOrganization(t, productID), permission: permission,
	}
}

type roleLessMemberFixture struct {
	productID      string
	organizationID string
	userID         string
}

// givenAMemberWithoutARole writes a membership with no role, as the next
// release writes it.
func givenAMemberWithoutARole(t *testing.T) roleLessMemberFixture {
	t.Helper()
	productID := GivenATenantAndProduct(t).Product.ID
	organizationID := givenOrganization(t, productID)
	userID := givenProductUser(t, productID)
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO organization_memberships
		(organization_id, product_user_id) VALUES ($1, $2)`, organizationID, userID)
	require.NoError(t, err)
	return roleLessMemberFixture{productID: productID, organizationID: organizationID, userID: userID}
}

func givenOrganization(t *testing.T, productID string) string {
	t.Helper()
	org := organization.Organization{ProductID: productID, Name: "Org " + uuid.NewString()}
	org.GenerateID()
	_, err := OrganizationRepo.Create(t.Context(), org)
	require.NoError(t, err)
	return org.ID
}

func givenPermission(t *testing.T, productID string) string {
	t.Helper()
	name := "doc:" + uuid.NewString()
	_, err := TestDB.ExecContext(t.Context(),
		`INSERT INTO product_resource_permissions (product_id, name, resource_type_id) VALUES ($1, $2, $3)`,
		productID, name, organizationTypeID(t, productID))
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
