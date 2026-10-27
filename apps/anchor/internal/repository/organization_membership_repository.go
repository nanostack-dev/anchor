package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/organization"
	"anchor/internal/domain/product/role"
	"anchor/internal/domain/product/user"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"
)

var _ OrganizationMembershipRepository = (*organizationMembershipRepositoryImpl)(nil)

// userOrgMembershipRow is the composite struct for scanning the joined query result
// from the product user's perspective.
type userOrgMembershipRow struct {
	model.OrganizationMemberships
	Organization model.Organizations                    `alias:"organizations"`
	Role         model.ProductRoles                     `alias:"product_roles"`
	Permissions  []model.ProductRoleResourcePermissions `alias:"product_role_resource_permissions"`
}

// userOrgMembershipSearchRow scans one membership per row for the paginated
// search of a user's organizations. Role permissions load for the page in a
// second statement, so the row carries no permission slice.
type userOrgMembershipSearchRow struct {
	model.OrganizationMemberships
	Organization model.Organizations `alias:"organizations"`
	Role         model.ProductRoles  `alias:"product_roles"`
}

// orgMembershipRow is the composite struct for scanning the joined query result
// from the organization's perspective (includes product user details).
type orgMembershipRow struct {
	model.OrganizationMemberships
	ProductUser model.ProductUsers                     `alias:"product_users"`
	Role        model.ProductRoles                     `alias:"product_roles"`
	Permissions []model.ProductRoleResourcePermissions `alias:"product_role_resource_permissions"`
}

// orgMembershipSearchRow scans one membership per row for the paginated search.
// It carries no permission slice: a keyset page needs one SQL row per item.
type orgMembershipSearchRow struct {
	model.OrganizationMemberships
	ProductUser model.ProductUsers `alias:"product_users"`
	Role        model.ProductRoles `alias:"product_roles"`
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
		ctx context.Context,
		productID string,
		productUserID string,
		includePermissions bool,
		req search.Request[user.SearchUserOrganizationsFilter, user.SortFieldUserOrganization],
	) (search.Result[user.OrganizationMembership], error)

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
	SearchByOrgID(
		ctx context.Context,
		productID string,
		orgID string,
		req search.Request[organization.SearchMembersFilter, organization.SortFieldMember],
	) (search.Result[organization.Membership], error)

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

	Delete(
		ctx context.Context,
		organizationID string,
		productUserID string,
	) error
}

type organizationMembershipRepositoryImpl struct {
	db     *sql.DB
	logger zerolog.Logger
}

func NewOrganizationMembershipRepository(
	db *sql.DB,
	logger zerolog.Logger,
) OrganizationMembershipRepository {
	return &organizationMembershipRepositoryImpl{
		db: db,
		logger: logger.With().Str(
			"component", "organization_membership_repository",
		).Logger(),
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
		r.toDomain,
	).Value()
}

func (r *organizationMembershipRepositoryImpl) SearchByProductUserID(
	ctx context.Context,
	productID string,
	productUserID string,
	includePermissions bool,
	req search.Request[user.SearchUserOrganizationsFilter, user.SortFieldUserOrganization],
) (search.Result[user.OrganizationMembership], error) {
	whereStmt := table.OrganizationMemberships.ProductUserID.EQ(postgres.String(productUserID)).AND(
		table.Organizations.ProductID.EQ(postgres.String(productID)),
	)

	if req.Filter != nil {
		if ids := jetx.BuildIDFilter(
			table.OrganizationMemberships.OrganizationID, req.Filter.OrganizationIDs,
		); ids != nil {
			whereStmt = whereStmt.AND(ids)
		}
		if roleIDs := jetx.BuildIDFilter(
			table.OrganizationMemberships.ProductRoleID, req.Filter.RoleIDs,
		); roleIDs != nil {
			whereStmt = whereStmt.AND(roleIDs)
		}
	}

	order, err := transactor.StrictKeys(
		req.Sort,
		map[user.SortFieldUserOrganization]transactor.Key[userOrgMembershipSearchRow]{
			user.SortFieldUserOrganizationJoinedAt: transactor.TimeKey(
				table.OrganizationMemberships.CreatedAt,
				func(row userOrgMembershipSearchRow) time.Time { return row.OrganizationMemberships.CreatedAt },
			),
			user.SortFieldUserOrganizationName: transactor.StringKey(
				table.Organizations.Name,
				func(row userOrgMembershipSearchRow) string { return row.Organization.Name },
			),
		},
		transactor.StringKey(
			table.OrganizationMemberships.OrganizationID,
			func(row userOrgMembershipSearchRow) string { return row.OrganizationMemberships.OrganizationID },
		),
	)
	if err != nil {
		return search.Result[user.OrganizationMembership]{}, err
	}

	result, err := transactor.Page(
		r.db,
		func(row userOrgMembershipSearchRow) user.OrganizationMembership {
			return r.toDomain(userOrgMembershipRow{
				OrganizationMemberships: row.OrganizationMemberships,
				Organization:            row.Organization,
				Role:                    row.Role,
			})
		},
		table.OrganizationMemberships.AllColumns,
		table.Organizations.AllColumns,
		table.ProductRoles.AllColumns,
	).
		From(
			table.OrganizationMemberships.
				INNER_JOIN(
					table.Organizations,
					table.OrganizationMemberships.OrganizationID.EQ(table.Organizations.ID),
				).
				LEFT_JOIN(
					table.ProductRoles,
					table.OrganizationMemberships.ProductRoleID.EQ(table.ProductRoles.ID),
				),
		).
		Where(whereStmt).
		Keyset(order).
		Run(ctx, req.Pagination).
		Value()
	if err != nil || !includePermissions || len(result.Items) == 0 {
		return result, err
	}

	var roleIDs []string
	for _, item := range result.Items {
		if item.Role != nil {
			roleIDs = append(roleIDs, item.Role.ID)
		}
	}
	if len(roleIDs) == 0 {
		return result, nil
	}
	permissions, err := transactor.Query[[]model.ProductRoleResourcePermissions](
		ctx, r.db,
		table.ProductRoleResourcePermissions.SELECT(table.ProductRoleResourcePermissions.AllColumns).WHERE(
			table.ProductRoleResourcePermissions.ProductRoleID.IN(jetx.ToStringExpressions(roleIDs)...),
		),
	).Value()
	if err != nil {
		return search.Result[user.OrganizationMembership]{}, err
	}

	rolePermissions := map[string][]string{}
	for _, permission := range permissions {
		rolePermissions[permission.ProductRoleID] = append(
			rolePermissions[permission.ProductRoleID], permission.PermissionName,
		)
	}
	for i := range result.Items {
		if held := result.Items[i].Role; held != nil {
			result.Items[i].RolePermissions = rolePermissions[held.ID]
		}
	}

	return result, nil
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
		r.toDomain,
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

func (r *organizationMembershipRepositoryImpl) Delete(
	ctx context.Context,
	organizationID string,
	productUserID string,
) error {
	stmt := table.OrganizationMemberships.DELETE().WHERE(
		table.OrganizationMemberships.OrganizationID.EQ(postgres.String(organizationID)).AND(
			table.OrganizationMemberships.ProductUserID.EQ(postgres.String(productUserID)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt).Err()
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

func (r *organizationMembershipRepositoryImpl) toDomain(row userOrgMembershipRow) user.OrganizationMembership {
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
func (r *organizationMembershipRepositoryImpl) toDomainMembership(row orgMembershipRow) organization.Membership {
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
		r.toDomainMembership,
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
		r.toDomainMembership,
	).Value()
}

func (r *organizationMembershipRepositoryImpl) SearchByOrgID(
	ctx context.Context,
	productID string,
	orgID string,
	req search.Request[organization.SearchMembersFilter, organization.SortFieldMember],
) (search.Result[organization.Membership], error) {
	whereStmt := table.OrganizationMemberships.OrganizationID.EQ(postgres.String(orgID)).AND(
		table.ProductUsers.ProductID.EQ(postgres.String(productID)),
	)

	if req.Filter != nil {
		if ids := jetx.BuildIDFilter(
			table.OrganizationMemberships.ProductUserID, req.Filter.ProductUserIDs,
		); ids != nil {
			whereStmt = whereStmt.AND(ids)
		}
		if extIDs := jetx.BuildIDFilter(
			table.ProductUsers.ExternalID, req.Filter.ExternalIDs,
		); extIDs != nil {
			whereStmt = whereStmt.AND(extIDs)
		}
		if emails := jetx.BuildStringArrayFilter(
			table.ProductUsers.Email, req.Filter.Emails,
		); emails != nil {
			whereStmt = whereStmt.AND(emails)
		}
		if roleIDs := jetx.BuildIDFilter(
			table.OrganizationMemberships.ProductRoleID, req.Filter.RoleIDs,
		); roleIDs != nil {
			whereStmt = whereStmt.AND(roleIDs)
		}
	}

	if req.FullTextSearch != nil && *req.FullTextSearch != "" {
		searchColumns := []postgres.ColumnString{
			table.ProductUsers.Email,
			table.ProductUsers.Name,
		}
		fullTextFilter := jetx.BuildSubstringFilter(searchColumns, *req.FullTextSearch)
		if fullTextFilter != nil {
			whereStmt = whereStmt.AND(fullTextFilter)
		}
	}

	order, err := transactor.StrictKeys(
		req.Sort,
		map[organization.SortFieldMember]transactor.Key[orgMembershipSearchRow]{
			organization.SortFieldMemberJoinedAt: transactor.TimeKey(
				table.OrganizationMemberships.CreatedAt,
				func(row orgMembershipSearchRow) time.Time { return row.OrganizationMemberships.CreatedAt },
			),
			organization.SortFieldMemberEmail: transactor.StringKey(
				table.ProductUsers.Email,
				func(row orgMembershipSearchRow) string { return row.ProductUser.Email },
			),
		},
		transactor.StringKey(
			table.OrganizationMemberships.ProductUserID,
			func(row orgMembershipSearchRow) string { return row.OrganizationMemberships.ProductUserID },
		),
	)
	if err != nil {
		return search.Result[organization.Membership]{}, err
	}

	return transactor.Page(
		r.db,
		func(row orgMembershipSearchRow) organization.Membership {
			return r.toDomainMembership(orgMembershipRow{
				OrganizationMemberships: row.OrganizationMemberships,
				ProductUser:             row.ProductUser,
				Role:                    row.Role,
			})
		},
		table.OrganizationMemberships.AllColumns,
		table.ProductUsers.AllColumns,
		table.ProductRoles.AllColumns,
	).
		From(
			table.OrganizationMemberships.
				INNER_JOIN(
					table.ProductUsers,
					table.OrganizationMemberships.ProductUserID.EQ(table.ProductUsers.ID),
				).
				LEFT_JOIN(
					table.ProductRoles,
					table.OrganizationMemberships.ProductRoleID.EQ(table.ProductRoles.ID),
				),
		).
		Where(whereStmt).
		Keyset(order).
		Run(ctx, req.Pagination).
		Value()
}

// heldRole is the membership's role, or nil for a member who holds none: the
// role joins with LEFT JOIN, so such a member still lists.
func heldRole(membership model.OrganizationMemberships, joined model.ProductRoles) *role.Ref {
	if membership.ProductRoleID == nil {
		return nil
	}
	return &role.Ref{ID: *membership.ProductRoleID, Name: joined.Name}
}
