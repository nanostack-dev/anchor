// Package repository reads the Postgres records the relationship store mirrors,
// and keeps the bookkeeping that tells which mirrors may lag behind.
package repository

import (
	"context"
	"database/sql"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/authz/model"
	dbmodel "anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	orgapikey "anchor/internal/domain/organization/apikey"
)

// ProductWide is the organization ID of a mark that covers every organization
// of a product, such as after a role or permission change.
const ProductWide = ""

// Mark says the relationship store may lag Postgres for an organization of a
// product, or for the whole product when OrganizationID is ProductWide.
type Mark struct {
	ProductID      string
	OrganizationID string
	Revision       int64
}

// IsProductWide reports whether the mark covers every organization.
func (m Mark) IsProductWide() bool { return m.OrganizationID == ProductWide }

type Repository interface {
	// Catalog reads the roles and permissions of a product.
	Catalog(ctx context.Context, productID string) (model.Catalog, error)
	// OrganizationState reads everything the relationship store derives from
	// one organization. A missing organization reads as not existing.
	OrganizationState(ctx context.Context, productID, organizationID string) (model.OrganizationState, error)
	// OrganizationIDs lists every organization of a product.
	OrganizationIDs(ctx context.Context, productID string) ([]string, error)
	// ProductIDsInternal lists every product of every tenant, for the startup
	// sweep. It bypasses tenant scope and is never reachable from a handler.
	ProductIDsInternal(ctx context.Context) ([]string, error)

	FindStoreID(ctx context.Context, productID string) (functional.Option[string], error)
	// SaveStoreID records the store of a product unless one is already recorded,
	// and returns the recorded one.
	SaveStoreID(ctx context.Context, productID, storeID string) (string, error)

	// Mark records that a product or one of its organizations needs a sync. It
	// joins the transaction in ctx, so the mark commits with the write.
	Mark(ctx context.Context, productID, organizationID string) error
	// PendingFor returns the marks that make a decision about an organization
	// stale: its own mark and the product-wide one.
	PendingFor(ctx context.Context, productID, organizationID string) ([]Mark, error)
	// PendingForProduct returns every mark of a product.
	PendingForProduct(ctx context.Context, productID string) ([]Mark, error)
	// OldestInternal returns the oldest marks of any product, for the sweeper.
	OldestInternal(ctx context.Context, limit int64) ([]Mark, error)
	// Clear deletes a mark unless a write bumped its revision since it was read.
	Clear(ctx context.Context, mark Mark) error
}

// Row shapes scanned with generated models; jet groups the joined rows by key.

type roleRow struct {
	dbmodel.ProductRoles
	Grants []dbmodel.ProductRoleResourcePermissions
}

func (row roleRow) role() model.Role {
	return model.Role{
		ID:   row.ID,
		Name: row.Name,
		Permissions: functional.Slice(row.Grants).Map(func(grant dbmodel.ProductRoleResourcePermissions) string {
			return grant.PermissionName
		}).ToSlice(),
	}
}

type workspaceRow struct {
	dbmodel.Workspaces
	Members []dbmodel.WorkspaceMemberships
}

func (row workspaceRow) workspace() model.WorkspaceState {
	return model.WorkspaceState{
		ID: row.ID,
		Members: functional.Slice(row.Members).Map(func(member dbmodel.WorkspaceMemberships) model.RoleAssignment {
			return model.RoleAssignment{ProductUserID: member.ProductUserID, RoleID: member.ProductRoleID}
		}).ToSlice(),
	}
}

type apiKeyRow struct {
	dbmodel.OrganizationAPIKeys
	Permissions []dbmodel.OrganizationAPIKeyPermissions
}

func (row apiKeyRow) grant() model.APIKeyGrant {
	return model.APIKeyGrant{
		ID: row.ID,
		Permissions: functional.Slice(row.Permissions).Map(func(p dbmodel.OrganizationAPIKeyPermissions) string {
			return p.PermissionName
		}).ToSlice(),
	}
}

func organizationMember(row dbmodel.OrganizationMemberships) model.RoleAssignment {
	return model.RoleAssignment{ProductUserID: row.ProductUserID, RoleID: row.ProductRoleID}
}

type repository struct {
	db *sql.DB
}

func New(db *sql.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Catalog(ctx context.Context, productID string) (model.Catalog, error) {
	product := postgres.String(productID)

	var roles []roleRow
	roleStmt := postgres.SELECT(
		table.ProductRoles.ID,
		table.ProductRoles.Name,
		table.ProductRoleResourcePermissions.ID,
		table.ProductRoleResourcePermissions.PermissionName,
	).FROM(table.ProductRoles.
		LEFT_JOIN(table.ProductRoleResourcePermissions,
			table.ProductRoleResourcePermissions.ProductRoleID.EQ(table.ProductRoles.ID)),
	).WHERE(table.ProductRoles.ProductID.EQ(product))
	if err := roleStmt.QueryContext(ctx, transactor.Executor(ctx, r.db), &roles); err != nil {
		return model.Catalog{}, err
	}

	var permissions []dbmodel.ProductResourcePermissions
	permissionStmt := table.ProductResourcePermissions.SELECT(
		table.ProductResourcePermissions.ProductID,
		table.ProductResourcePermissions.Name,
	).WHERE(table.ProductResourcePermissions.ProductID.EQ(product))
	if err := permissionStmt.QueryContext(ctx, transactor.Executor(ctx, r.db), &permissions); err != nil {
		return model.Catalog{}, err
	}

	return model.Catalog{
		Roles: functional.Slice(roles).Map(roleRow.role).ToSlice(),
		Permissions: functional.Slice(permissions).Map(func(permission dbmodel.ProductResourcePermissions) string {
			return permission.Name
		}).ToSlice(),
	}, nil
}

func (r *repository) OrganizationState(
	ctx context.Context, productID, organizationID string,
) (model.OrganizationState, error) {
	state := model.OrganizationState{ProductID: productID, OrganizationID: organizationID}
	organization := postgres.String(organizationID)

	var found []dbmodel.Organizations
	exists := table.Organizations.SELECT(table.Organizations.ID).
		WHERE(table.Organizations.ID.EQ(organization).
			AND(table.Organizations.ProductID.EQ(postgres.String(productID))))
	if err := exists.QueryContext(ctx, transactor.Executor(ctx, r.db), &found); err != nil {
		return state, err
	}
	if state.Exists = len(found) > 0; !state.Exists {
		return state, nil
	}

	var err error
	if state.Members, err = r.organizationMembers(ctx, organization); err != nil {
		return state, err
	}
	if state.Workspaces, err = r.workspaces(ctx, organization); err != nil {
		return state, err
	}
	if state.APIKeys, err = r.apiKeyGrants(ctx, productID, organization); err != nil {
		return state, err
	}
	return state, nil
}

func (r *repository) organizationMembers(
	ctx context.Context, organization postgres.StringExpression,
) ([]model.RoleAssignment, error) {
	var rows []dbmodel.OrganizationMemberships
	stmt := table.OrganizationMemberships.SELECT(
		table.OrganizationMemberships.OrganizationID,
		table.OrganizationMemberships.ProductUserID,
		table.OrganizationMemberships.ProductRoleID,
	).WHERE(table.OrganizationMemberships.OrganizationID.EQ(organization))
	if err := stmt.QueryContext(ctx, transactor.Executor(ctx, r.db), &rows); err != nil {
		return nil, err
	}
	return functional.Slice(rows).Map(organizationMember).ToSlice(), nil
}

func (r *repository) workspaces(
	ctx context.Context, organization postgres.StringExpression,
) ([]model.WorkspaceState, error) {
	var rows []workspaceRow
	stmt := postgres.SELECT(
		table.Workspaces.ID,
		table.WorkspaceMemberships.WorkspaceID,
		table.WorkspaceMemberships.ProductUserID,
		table.WorkspaceMemberships.ProductRoleID,
	).FROM(table.Workspaces.
		LEFT_JOIN(table.WorkspaceMemberships, table.WorkspaceMemberships.WorkspaceID.EQ(table.Workspaces.ID)),
	).WHERE(table.Workspaces.OrganizationID.EQ(organization))
	if err := stmt.QueryContext(ctx, transactor.Executor(ctx, r.db), &rows); err != nil {
		return nil, err
	}
	return functional.Slice(rows).Map(workspaceRow.workspace).ToSlice(), nil
}

func (r *repository) apiKeyGrants(
	ctx context.Context, productID string, organization postgres.StringExpression,
) ([]model.APIKeyGrant, error) {
	var rows []apiKeyRow
	stmt := postgres.SELECT(
		table.OrganizationAPIKeys.ID,
		table.OrganizationAPIKeyPermissions.APIKeyID,
		table.OrganizationAPIKeyPermissions.OrganizationID,
		table.OrganizationAPIKeyPermissions.ProductID,
		table.OrganizationAPIKeyPermissions.PermissionName,
	).FROM(table.OrganizationAPIKeys.
		INNER_JOIN(table.OrganizationAPIKeyPermissions,
			table.OrganizationAPIKeyPermissions.APIKeyID.EQ(table.OrganizationAPIKeys.ID)),
	).WHERE(table.OrganizationAPIKeys.OrganizationID.EQ(organization).
		AND(table.OrganizationAPIKeys.Status.EQ(postgres.String(string(orgapikey.StatusActive)))).
		AND(table.OrganizationAPIKeyPermissions.ProductID.EQ(postgres.String(productID))))
	if err := stmt.QueryContext(ctx, transactor.Executor(ctx, r.db), &rows); err != nil {
		return nil, err
	}
	return functional.Slice(rows).Map(apiKeyRow.grant).ToSlice(), nil
}

func (r *repository) OrganizationIDs(ctx context.Context, productID string) ([]string, error) {
	var rows []dbmodel.Organizations
	stmt := table.Organizations.SELECT(table.Organizations.ID).
		WHERE(table.Organizations.ProductID.EQ(postgres.String(productID)))
	if err := stmt.QueryContext(ctx, transactor.Executor(ctx, r.db), &rows); err != nil {
		return nil, err
	}
	return functional.Slice(rows).Map(func(row dbmodel.Organizations) string { return row.ID }).ToSlice(), nil
}

func (r *repository) ProductIDsInternal(ctx context.Context) ([]string, error) {
	var rows []dbmodel.Products
	if err := table.Products.SELECT(table.Products.ID).
		QueryContext(ctx, transactor.Executor(ctx, r.db), &rows); err != nil {
		return nil, err
	}
	return functional.Slice(rows).Map(func(row dbmodel.Products) string { return row.ID }).ToSlice(), nil
}

func (r *repository) FindStoreID(ctx context.Context, productID string) (functional.Option[string], error) {
	var rows []dbmodel.AuthzStores
	stmt := table.AuthzStores.SELECT(table.AuthzStores.ProductID, table.AuthzStores.StoreID).
		WHERE(table.AuthzStores.ProductID.EQ(postgres.String(productID)))
	if err := stmt.QueryContext(ctx, transactor.Executor(ctx, r.db), &rows); err != nil {
		return functional.None[string](), err
	}
	if len(rows) == 0 {
		return functional.None[string](), nil
	}
	return functional.Some(rows[0].StoreID), nil
}

func (r *repository) SaveStoreID(ctx context.Context, productID, storeID string) (string, error) {
	insert := table.AuthzStores.INSERT(table.AuthzStores.ProductID, table.AuthzStores.StoreID).
		VALUES(productID, storeID).
		ON_CONFLICT(table.AuthzStores.ProductID).DO_NOTHING()
	if err := transactor.Exec(ctx, r.db, insert).Err(); err != nil {
		return "", err
	}
	saved, err := r.FindStoreID(ctx, productID)
	if err != nil {
		return "", err
	}
	return saved.OrElse(storeID), nil
}

func (r *repository) Mark(ctx context.Context, productID, organizationID string) error {
	marks := table.AuthzSyncMarks
	stmt := marks.INSERT(marks.ProductID, marks.OrganizationID).
		VALUES(productID, organizationID).
		ON_CONFLICT(marks.ProductID, marks.OrganizationID).
		DO_UPDATE(postgres.SET(
			marks.Revision.SET(marks.Revision.ADD(postgres.Int(1))),
			marks.MarkedAt.SET(postgres.NOW()),
		))
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func toMark(row dbmodel.AuthzSyncMarks) Mark {
	return Mark{ProductID: row.ProductID, OrganizationID: row.OrganizationID, Revision: row.Revision}
}

func (r *repository) PendingFor(ctx context.Context, productID, organizationID string) ([]Mark, error) {
	var rows []dbmodel.AuthzSyncMarks
	stmt := table.AuthzSyncMarks.SELECT(table.AuthzSyncMarks.AllColumns).
		WHERE(table.AuthzSyncMarks.ProductID.EQ(postgres.String(productID)).
			AND(table.AuthzSyncMarks.OrganizationID.IN(postgres.String(ProductWide), postgres.String(organizationID))))
	if err := stmt.QueryContext(ctx, transactor.Executor(ctx, r.db), &rows); err != nil {
		return nil, err
	}
	return functional.Slice(rows).Map(toMark).ToSlice(), nil
}

func (r *repository) PendingForProduct(ctx context.Context, productID string) ([]Mark, error) {
	var rows []dbmodel.AuthzSyncMarks
	stmt := table.AuthzSyncMarks.SELECT(table.AuthzSyncMarks.AllColumns).
		WHERE(table.AuthzSyncMarks.ProductID.EQ(postgres.String(productID)))
	if err := stmt.QueryContext(ctx, transactor.Executor(ctx, r.db), &rows); err != nil {
		return nil, err
	}
	return functional.Slice(rows).Map(toMark).ToSlice(), nil
}

func (r *repository) OldestInternal(ctx context.Context, limit int64) ([]Mark, error) {
	var rows []dbmodel.AuthzSyncMarks
	stmt := table.AuthzSyncMarks.SELECT(table.AuthzSyncMarks.AllColumns).
		ORDER_BY(table.AuthzSyncMarks.MarkedAt.ASC()).
		LIMIT(limit)
	if err := stmt.QueryContext(ctx, transactor.Executor(ctx, r.db), &rows); err != nil {
		return nil, err
	}
	return functional.Slice(rows).Map(toMark).ToSlice(), nil
}

func (r *repository) Clear(ctx context.Context, mark Mark) error {
	marks := table.AuthzSyncMarks
	stmt := marks.DELETE().WHERE(
		marks.ProductID.EQ(postgres.String(mark.ProductID)).
			AND(marks.OrganizationID.EQ(postgres.String(mark.OrganizationID))).
			AND(marks.Revision.EQ(postgres.Int(mark.Revision))),
	)
	return transactor.Exec(ctx, r.db, stmt).Err()
}
