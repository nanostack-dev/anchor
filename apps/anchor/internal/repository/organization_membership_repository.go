package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/organization"
	"anchor/internal/domain/product/user"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
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

// orgMembershipRow is the composite struct for scanning the joined query result
// from the organization's perspective (includes product user details).
type orgMembershipRow struct {
	model.OrganizationMemberships
	ProductUser model.ProductUsers                     `alias:"product_users"`
	Role        model.ProductRoles                     `alias:"product_roles"`
	Permissions []model.ProductRoleResourcePermissions `alias:"product_role_resource_permissions"`
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

	Create(
		ctx context.Context,
		productID string,
		organizationID string,
		productUserID string,
		roleID string,
	) (organization.Membership, error)

	Update(
		ctx context.Context,
		productID string,
		organizationID string,
		productUserID string,
		roleID string,
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
	).ORDER_BY(table.OrganizationMemberships.CreatedAt.ASC())

	return transactor.QueryMapSlice(
		ctx, r.db, stmt,
		func(row userOrgMembershipRow) user.OrganizationMembership {
			return r.toDomain(row)
		},
	).Value()
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
		func(row userOrgMembershipRow) user.OrganizationMembership {
			return r.toDomain(row)
		},
	)
}

func (r *organizationMembershipRepositoryImpl) Create(
	ctx context.Context,
	productID string,
	organizationID string,
	productUserID string,
	roleID string,
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

	if err := transactor.Exec(ctx, r.db, stmt).Err(); err != nil {
		return organization.Membership{}, err
	}

	found, err := r.FindByOrgIDAndUserID(ctx, productID, organizationID, productUserID, false)
	if err != nil {
		return organization.Membership{}, err
	}
	if found.IsAbsent() {
		return organization.Membership{}, fault.ErrNotFound
	}

	return found.Value(), nil
}

func (r *organizationMembershipRepositoryImpl) Update(
	ctx context.Context,
	productID string,
	organizationID string,
	productUserID string,
	roleID string,
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

	if err := transactor.Exec(ctx, r.db, stmt).Err(); err != nil {
		return organization.Membership{}, err
	}

	found, err := r.FindByOrgIDAndUserID(ctx, productID, organizationID, productUserID, false)
	if err != nil {
		return organization.Membership{}, err
	}
	if found.IsAbsent() {
		return organization.Membership{}, fault.ErrNotFound
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
		INNER_JOIN(
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
	var permissions []string
	for _, p := range row.Permissions {
		permissions = append(permissions, p.PermissionName)
	}

	return user.OrganizationMembership{
		OrganizationID:           row.Organization.ID,
		OrganizationName:         row.Organization.Name,
		OrganizationDescription:  row.Organization.Description,
		OrganizationMetadataJSON: mapper.MetadataJSONToDomain(row.Organization.MetadataJSON),
		RoleID:                   row.Role.ID,
		RoleName:                 row.Role.Name,
		RolePermissions:          permissions,
		JoinedAt:                 row.OrganizationMemberships.CreatedAt,
	}
}

// toDomainMembership maps an orgMembershipRow (org-perspective join) to organization.Membership.
func (r *organizationMembershipRepositoryImpl) toDomainMembership(row orgMembershipRow) organization.Membership {
	var permissions []string
	for _, p := range row.Permissions {
		permissions = append(permissions, p.PermissionName)
	}

	var userName string
	if row.ProductUser.Name != nil {
		userName = *row.ProductUser.Name
	}

	return organization.Membership{
		OrganizationID:  row.OrganizationMemberships.OrganizationID,
		ProductUserID:   row.OrganizationMemberships.ProductUserID,
		UserEmail:       row.ProductUser.Email,
		UserName:        userName,
		UserExternalID:  row.ProductUser.ExternalID,
		RoleID:          row.Role.ID,
		RoleName:        row.Role.Name,
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
		INNER_JOIN(
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
		func(row orgMembershipRow) organization.Membership {
			return r.toDomainMembership(row)
		},
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
		func(row orgMembershipRow) organization.Membership {
			return r.toDomainMembership(row)
		},
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

	return transactor.Page(
		r.db,
		r.toDomainMembership,
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
				INNER_JOIN(
					table.ProductRoles,
					table.OrganizationMemberships.ProductRoleID.EQ(table.ProductRoles.ID),
				),
		).
		Where(whereStmt).
		OrderBy(transactor.SortColumns(
			req.Sort,
			map[organization.SortFieldMember]postgres.Column{
				organization.SortFieldMemberJoinedAt: table.OrganizationMemberships.CreatedAt,
				organization.SortFieldMemberEmail:    table.ProductUsers.Email,
			},
		)...).
		Run(ctx, req.Pagination).
		Value()
}
