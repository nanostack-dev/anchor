package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/organization"
	"anchor/internal/domain/product/role"
	"anchor/internal/domain/product/user"
	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
)

func _() {
	nanorepo.Implement[OrganizationMembershipRepository, organizationMembershipRepositoryImpl](
		table.OrganizationMemberships, nanorepo.NoMapper[model.OrganizationMemberships],
	)

	// POST /v1/products/{product_id}/product-users/{id}/organizations/search. The
	// membership is covered through its INNER JOIN to the organization (its
	// organization_id is the key of the organization, which the product scopes);
	// the role is the membership's own data (product_role_id is the key of the
	// role). The route names the product user.
	nanorepo.Search[userOrganizationRow]("UserOrganization", table.OrganizationMemberships,
		nanorepo.Map(userOrganizationToDomain),
		nanorepo.From(membershipsWithTheirOrganization),
		nanorepo.Scope(table.Organizations.ProductID),
		nanorepo.Scope(table.OrganizationMemberships.ProductUserID),
		nanorepo.Filter(table.OrganizationMemberships.OrganizationID, nanorepo.In.API("organization_ids")),
		nanorepo.Filter(table.OrganizationMemberships.ProductRoleID, nanorepo.In.API("role_ids")),
		nanorepo.SortKey(table.OrganizationMemberships.CreatedAt).API("joined_at"),
		nanorepo.SortKey(table.Organizations.Name).API("name"),
		nanorepo.DefaultSort(table.OrganizationMemberships.CreatedAt, nanorepo.Asc),
		nanorepo.DefaultLimit(defaultPageLimit),
	)

	// POST /v1/products/{product_id}/organizations/{organization_id}/members/search.
	// The product user is covered directly by the product; the membership by the
	// organization; the role is the membership's own data.
	nanorepo.Search[orgMemberRow]("OrganizationMember", table.OrganizationMemberships,
		nanorepo.Map(memberToDomain),
		nanorepo.From(membershipsWithTheirUser),
		nanorepo.Scope(table.OrganizationMemberships.OrganizationID),
		nanorepo.Scope(table.ProductUsers.ProductID),
		nanorepo.Filter(table.OrganizationMemberships.ProductUserID, nanorepo.In),
		nanorepo.Filter(table.ProductUsers.ExternalID, nanorepo.In),
		nanorepo.Filter(table.ProductUsers.Email, nanorepo.In),
		nanorepo.Filter(table.OrganizationMemberships.ProductRoleID, nanorepo.In.API("role_ids")),
		nanorepo.FullText(table.ProductUsers.Email, table.ProductUsers.Name),
		nanorepo.SortKey(table.OrganizationMemberships.CreatedAt).API("joined_at"),
		nanorepo.SortKey(table.ProductUsers.Email).API("email"),
		nanorepo.DefaultSort(table.OrganizationMemberships.CreatedAt, nanorepo.Desc),
		nanorepo.DefaultLimit(defaultPageLimit),
	)
}

// membershipsWithTheirOrganization reads each membership with its organization
// and, when it holds one, its role.
func membershipsWithTheirOrganization() postgres.ReadableTable {
	return table.OrganizationMemberships.
		INNER_JOIN(
			table.Organizations,
			table.OrganizationMemberships.OrganizationID.EQ(table.Organizations.ID),
		).
		LEFT_JOIN(
			table.ProductRoles,
			table.OrganizationMemberships.ProductRoleID.EQ(table.ProductRoles.ID),
		)
}

// membershipsWithTheirUser reads each membership with its product user and,
// when it holds one, its role.
func membershipsWithTheirUser() postgres.ReadableTable {
	return table.OrganizationMemberships.
		INNER_JOIN(
			table.ProductUsers,
			table.OrganizationMemberships.ProductUserID.EQ(table.ProductUsers.ID),
		).
		LEFT_JOIN(
			table.ProductRoles,
			table.OrganizationMemberships.ProductRoleID.EQ(table.ProductRoles.ID),
		)
}

// userOrganizationToDomain maps a search row. The role permissions of a page
// load in a second statement (RolePermissionsByRoleID), only when the caller
// asks for them.
func userOrganizationToDomain(row userOrganizationRow) user.OrganizationMembership {
	return user.OrganizationMembership{
		OrganizationID:           row.OrganizationID,
		OrganizationName:         row.OrganizationName,
		OrganizationDescription:  row.OrganizationDescription,
		OrganizationMetadataJSON: mapper.MetadataJSONToDomain(row.OrganizationMetadataJSON),
		Role: heldRole(
			row.OrganizationMemberships,
			model.ProductRoles{Name: derefString(row.RoleName)},
		),
		RolePermissions: []string{},
		JoinedAt:        row.CreatedAt,
	}
}

// memberToDomain maps a search row.
func memberToDomain(row orgMemberRow) organization.Membership {
	return organization.Membership{
		OrganizationID:  row.OrganizationID,
		ProductUserID:   row.ProductUserID,
		UserEmail:       row.UserEmail,
		UserName:        derefString(row.UserName),
		UserExternalID:  row.UserExternalID,
		Role:            heldRole(row.OrganizationMemberships, model.ProductRoles{Name: derefString(row.RoleName)}),
		RolePermissions: []string{},
		JoinedAt:        row.CreatedAt,
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// userOrgMembershipRow is the composite struct for scanning the joined query result
// from the product user's perspective.
type userOrgMembershipRow struct {
	model.OrganizationMemberships
	Organization model.Organizations                    `alias:"organizations"`
	Role         model.ProductRoles                     `alias:"product_roles"`
	Permissions  []model.ProductRoleResourcePermissions `alias:"product_role_resource_permissions"`
}

// userOrganizationRow is one row of the paginated search of a user's
// organizations: the membership with the columns of its organization and of its
// role that the domain type needs. Role permissions load for the page in a
// second statement, so the row carries none.
type userOrganizationRow struct {
	model.OrganizationMemberships
	OrganizationName         string  `alias:"organizations.name"`
	OrganizationDescription  *string `alias:"organizations.description"`
	OrganizationMetadataJSON *string `alias:"organizations.metadata_json"`
	RoleName                 *string `alias:"product_roles.name"`
}

// orgMembershipRow is the composite struct for scanning the joined query result
// from the organization's perspective (includes product user details).
type orgMembershipRow struct {
	model.OrganizationMemberships
	ProductUser model.ProductUsers                     `alias:"product_users"`
	Role        model.ProductRoles                     `alias:"product_roles"`
	Permissions []model.ProductRoleResourcePermissions `alias:"product_role_resource_permissions"`
}

// orgMemberRow is one row of the paginated search of an organization's members:
// the membership with the columns of its product user and of its role that the
// domain type needs. A keyset page needs one SQL row per item, so the row carries
// no permission slice.
type orgMemberRow struct {
	model.OrganizationMemberships
	UserEmail      string  `alias:"product_users.email"`
	UserName       *string `alias:"product_users.name"`
	UserExternalID *string `alias:"product_users.external_id"`
	RoleName       *string `alias:"product_roles.name"`
}

// OrganizationMembershipRepository provides access to organization memberships.
type OrganizationMembershipRepository interface {
	// FindByProductUserID returns all organizations a product user belongs to,
	// including role details. When includePermissions is true, role permissions are included.
	FindByProductUserID(
		ctx context.Context,
		productID string,
		productUserID string,
		includePermissions bool,
	) ([]user.OrganizationMembership, error)

	// SearchByProductUserID pages the organizations a product user belongs to
	// with a cursor. When includePermissions is true, role permissions are
	// loaded for the page.
	SearchByProductUserID(
		ctx context.Context, q UserOrganizationSearch,
	) (search.Result[user.OrganizationMembership], error)

	// RolePermissionsByRoleID reads the permission names of each role, keyed by
	// role ID. A role that holds none has no entry.
	RolePermissionsByRoleID(ctx context.Context, roleIDs []string) (map[string][]string, error)

	// FindByProductUserIDAndOrgID returns a specific organization membership for a product user.
	// When includePermissions is true, role permissions are included.
	FindByProductUserIDAndOrgID(
		ctx context.Context,
		productID string,
		productUserID string,
		organizationID string,
		includePermissions bool,
	) (functional.Option[user.OrganizationMembership], error)

	// FindByOrgIDAndUserID returns a specific member of an organization.
	// When includePermissions is true, role permissions are included.
	FindByOrgIDAndUserID(
		ctx context.Context,
		productID string,
		orgID string,
		productUserID string,
		includePermissions bool,
	) (functional.Option[organization.Membership], error)

	// FindByOrgID returns all members of an organization.
	// When includePermissions is true, role permissions are included.
	FindByOrgID(
		ctx context.Context,
		productID string,
		orgID string,
		includePermissions bool,
	) ([]organization.Membership, error)

	// SearchByOrgID returns paginated, filtered members of an organization.
	// Supports filtering by product_user_ids, external_ids, emails, and role_ids.
	SearchByOrgID(ctx context.Context, q OrganizationMemberSearch) (search.Result[organization.Membership], error)

	// Create adds the member. A nil roleID adds a member who holds no role.
	Create(
		ctx context.Context,
		productID string,
		organizationID string,
		productUserID string,
		roleID *string,
	) (organization.Membership, error)

	// Update sets the member's role. A nil roleID removes it.
	Update(
		ctx context.Context,
		productID string,
		organizationID string,
		productUserID string,
		roleID *string,
	) (organization.Membership, error)

	DeleteByOrganizationIDAndProductUserID(
		ctx context.Context,
		organizationID string,
		productUserID string,
	) error
}

type organizationMembershipRepositoryImpl struct {
	organizationMembershipsBase

	db     *sql.DB
	logger *slog.Logger
}

func NewOrganizationMembershipRepository(
	db *sql.DB,
	logger *slog.Logger,
) OrganizationMembershipRepository {
	return &organizationMembershipRepositoryImpl{
		organizationMembershipsBase: newOrganizationMembershipsBase(db),
		db:                          db,
		logger:                      logger.With(slog.String("component", "organization_membership_repository")),
	}
}

func (r *organizationMembershipRepositoryImpl) FindByProductUserID(
	ctx context.Context,
	productID string,
	productUserID string,
	includePermissions bool,
) ([]user.OrganizationMembership, error) {
	stmt := r.buildQuery(includePermissions).WHERE(
		table.OrganizationMemberships.ProductUserID.EQ(postgres.String(productUserID)).AND(
			table.Organizations.ProductID.EQ(postgres.String(productID)),
		),
	).ORDER_BY(table.OrganizationMemberships.CreatedAt.ASC(), table.OrganizationMemberships.OrganizationID.ASC())

	return transactor.QueryMapSlice(
		ctx, r.db, stmt,
		userOrgMembershipToDomain,
	).Value()
}

func (r *organizationMembershipRepositoryImpl) RolePermissionsByRoleID(
	ctx context.Context, roleIDs []string,
) (map[string][]string, error) {
	if len(roleIDs) == 0 {
		return map[string][]string{}, nil
	}
	permissions, err := transactor.Query[[]model.ProductRoleResourcePermissions](
		ctx, r.db,
		table.ProductRoleResourcePermissions.SELECT(table.ProductRoleResourcePermissions.AllColumns).WHERE(
			table.ProductRoleResourcePermissions.ProductRoleID.IN(jetx.ToStringExpressions(roleIDs)...),
		),
	).Value()
	if err != nil {
		return nil, err
	}

	rolePermissions := map[string][]string{}
	for _, permission := range permissions {
		rolePermissions[permission.ProductRoleID] = append(
			rolePermissions[permission.ProductRoleID], permission.PermissionName,
		)
	}
	return rolePermissions, nil
}

func (r *organizationMembershipRepositoryImpl) FindByProductUserIDAndOrgID(
	ctx context.Context,
	productID string,
	productUserID string,
	organizationID string,
	includePermissions bool,
) (functional.Option[user.OrganizationMembership], error) {
	stmt := r.buildQuery(includePermissions).WHERE(
		table.OrganizationMemberships.ProductUserID.EQ(postgres.String(productUserID)).AND(
			table.OrganizationMemberships.OrganizationID.EQ(postgres.String(organizationID)),
		).AND(
			table.Organizations.ProductID.EQ(postgres.String(productID)),
		),
	)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		userOrgMembershipToDomain,
	)
}

const organizationMembershipPrimaryKey = "organization_memberships_pkey"

// ErrOrganizationMembershipExists is returned by Create when the product user
// is already a member: the (organization, product user) primary key settles the
// race the service pre-check cannot.
var ErrOrganizationMembershipExists = errors.New("organization membership already exists")

// ErrOrganizationMembershipRoleGone is returned by Create and Update when the
// role was deleted after the service found it: a role delete holds the role's
// row FOR UPDATE, so the write waits and then trips the foreign key.
var ErrOrganizationMembershipRoleGone = errors.New("organization membership role no longer exists")

const organizationMembershipRoleForeignKey = "organization_memberships_product_role_id_fkey"

func (r *organizationMembershipRepositoryImpl) Create(
	ctx context.Context,
	productID string,
	organizationID string,
	productUserID string,
	roleID *string,
) (organization.Membership, error) {
	entity := model.OrganizationMemberships{
		OrganizationID: organizationID,
		ProductUserID:  productUserID,
		ProductRoleID:  roleID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	stmt := table.OrganizationMemberships.INSERT(
		table.OrganizationMemberships.AllColumns,
	).MODEL(entity)

	if err := transactor.Exec(ctx, r.db, stmt).
		OnUnique(ErrOrganizationMembershipExists, organizationMembershipPrimaryKey).
		OnForeignKey(ErrOrganizationMembershipRoleGone, organizationMembershipRoleForeignKey).
		Err(); err != nil {
		return organization.Membership{}, err
	}

	found, err := r.FindByOrgIDAndUserID(
		ctx,
		productID,
		organizationID,
		productUserID,
		organization.WithoutRolePermissions,
	)
	if err != nil {
		return organization.Membership{}, err
	}
	if found.IsAbsent() {
		return organization.Membership{}, organization.ErrMembershipNotFound
	}

	return found.Value(), nil
}

func (r *organizationMembershipRepositoryImpl) Update(
	ctx context.Context,
	productID string,
	organizationID string,
	productUserID string,
	roleID *string,
) (organization.Membership, error) {
	entity := model.OrganizationMemberships{
		OrganizationID: organizationID,
		ProductUserID:  productUserID,
		ProductRoleID:  roleID,
		UpdatedAt:      time.Now(),
	}

	stmt := table.OrganizationMemberships.UPDATE(
		table.OrganizationMemberships.ProductRoleID,
		table.OrganizationMemberships.UpdatedAt,
	).MODEL(
		entity,
	).WHERE(
		table.OrganizationMemberships.OrganizationID.EQ(postgres.String(organizationID)).AND(
			table.OrganizationMemberships.ProductUserID.EQ(postgres.String(productUserID)),
		),
	)

	if err := transactor.Exec(ctx, r.db, stmt).
		OnForeignKey(ErrOrganizationMembershipRoleGone, organizationMembershipRoleForeignKey).
		Err(); err != nil {
		return organization.Membership{}, err
	}

	found, err := r.FindByOrgIDAndUserID(
		ctx,
		productID,
		organizationID,
		productUserID,
		organization.WithoutRolePermissions,
	)
	if err != nil {
		return organization.Membership{}, err
	}
	if found.IsAbsent() {
		return organization.Membership{}, organization.ErrMembershipNotFound
	}

	return found.Value(), nil
}

func (r *organizationMembershipRepositoryImpl) buildQuery(includePermissions bool) postgres.SelectStatement {
	joinExpr := table.OrganizationMemberships.
		INNER_JOIN(
			table.Organizations,
			table.OrganizationMemberships.OrganizationID.EQ(table.Organizations.ID),
		).
		LEFT_JOIN(
			table.ProductRoles,
			table.OrganizationMemberships.ProductRoleID.EQ(table.ProductRoles.ID),
		)

	if includePermissions {
		joinExpr = joinExpr.LEFT_JOIN(
			table.ProductRoleResourcePermissions,
			table.ProductRoles.ID.EQ(table.ProductRoleResourcePermissions.ProductRoleID),
		)
		return postgres.SELECT(
			table.OrganizationMemberships.AllColumns,
			table.Organizations.AllColumns,
			table.ProductRoles.AllColumns,
			table.ProductRoleResourcePermissions.AllColumns,
		).FROM(joinExpr)
	}

	return postgres.SELECT(
		table.OrganizationMemberships.AllColumns,
		table.Organizations.AllColumns,
		table.ProductRoles.AllColumns,
	).FROM(joinExpr)
}

func userOrgMembershipToDomain(row userOrgMembershipRow) user.OrganizationMembership {
	permissions := functional.Slice(row.Permissions).Map(
		func(p model.ProductRoleResourcePermissions) string { return p.PermissionName },
	)

	return user.OrganizationMembership{
		OrganizationID:           row.Organization.ID,
		OrganizationName:         row.Organization.Name,
		OrganizationDescription:  row.Organization.Description,
		OrganizationMetadataJSON: mapper.MetadataJSONToDomain(row.Organization.MetadataJSON),
		Role:                     heldRole(row.OrganizationMemberships, row.Role),
		RolePermissions:          permissions,
		JoinedAt:                 row.OrganizationMemberships.CreatedAt,
	}
}

// toDomainMembership maps an orgMembershipRow (org-perspective join) to organization.Membership.
func orgMembershipToDomain(row orgMembershipRow) organization.Membership {
	permissions := functional.Slice(row.Permissions).Map(
		func(p model.ProductRoleResourcePermissions) string { return p.PermissionName },
	)

	userName := functional.FromPtr(row.ProductUser.Name).OrElse("")

	return organization.Membership{
		OrganizationID:  row.OrganizationMemberships.OrganizationID,
		ProductUserID:   row.OrganizationMemberships.ProductUserID,
		UserEmail:       row.ProductUser.Email,
		UserName:        userName,
		UserExternalID:  row.ProductUser.ExternalID,
		Role:            heldRole(row.OrganizationMemberships, row.Role),
		RolePermissions: permissions,
		JoinedAt:        row.OrganizationMemberships.CreatedAt,
	}
}

// buildOrgQuery builds the base SELECT for org-perspective membership queries.
// It joins: organization_memberships → product_users → product_roles [→ product_role_resource_permissions].
func (r *organizationMembershipRepositoryImpl) buildOrgQuery(includePermissions bool) postgres.SelectStatement {
	joinExpr := table.OrganizationMemberships.
		INNER_JOIN(
			table.ProductUsers,
			table.OrganizationMemberships.ProductUserID.EQ(table.ProductUsers.ID),
		).
		LEFT_JOIN(
			table.ProductRoles,
			table.OrganizationMemberships.ProductRoleID.EQ(table.ProductRoles.ID),
		)

	if includePermissions {
		joinExpr = joinExpr.LEFT_JOIN(
			table.ProductRoleResourcePermissions,
			table.ProductRoles.ID.EQ(table.ProductRoleResourcePermissions.ProductRoleID),
		)
		return postgres.SELECT(
			table.OrganizationMemberships.AllColumns,
			table.ProductUsers.AllColumns,
			table.ProductRoles.AllColumns,
			table.ProductRoleResourcePermissions.AllColumns,
		).FROM(joinExpr)
	}

	return postgres.SELECT(
		table.OrganizationMemberships.AllColumns,
		table.ProductUsers.AllColumns,
		table.ProductRoles.AllColumns,
	).FROM(joinExpr)
}

func (r *organizationMembershipRepositoryImpl) FindByOrgIDAndUserID(
	ctx context.Context,
	productID string,
	orgID string,
	productUserID string,
	includePermissions bool,
) (functional.Option[organization.Membership], error) {
	stmt := r.buildOrgQuery(includePermissions).WHERE(
		table.OrganizationMemberships.OrganizationID.EQ(postgres.String(orgID)).AND(
			table.OrganizationMemberships.ProductUserID.EQ(postgres.String(productUserID)),
		).AND(
			table.ProductUsers.ProductID.EQ(postgres.String(productID)),
		),
	)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		orgMembershipToDomain,
	)
}

func (r *organizationMembershipRepositoryImpl) FindByOrgID(
	ctx context.Context,
	productID string,
	orgID string,
	includePermissions bool,
) ([]organization.Membership, error) {
	stmt := r.buildOrgQuery(includePermissions).WHERE(
		table.OrganizationMemberships.OrganizationID.EQ(postgres.String(orgID)).AND(
			table.ProductUsers.ProductID.EQ(postgres.String(productID)),
		),
	).ORDER_BY(table.OrganizationMemberships.CreatedAt.ASC())

	return transactor.QueryMapSlice(
		ctx, r.db, stmt,
		orgMembershipToDomain,
	).Value()
}

// heldRole is the membership's role, or nil for a member who holds none: the
// role joins with LEFT JOIN, so such a member still lists.
func heldRole(membership model.OrganizationMemberships, joined model.ProductRoles) *role.Ref {
	if membership.ProductRoleID == nil {
		return nil
	}
	return &role.Ref{ID: *membership.ProductRoleID, Name: joined.Name}
}
