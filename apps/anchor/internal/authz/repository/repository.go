// Package repository reads what each Product's OpenFGA store mirrors from
// Anchor's tables, and keeps the bookkeeping of the sync: marks and each
// store's model.
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"

	"anchor/internal/authz/model"
	dbmodel "anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	orgapikey "anchor/internal/domain/organization/apikey"
)

// ProductScope is the organization ID of a mark that covers the whole Product:
// a catalog change, or a deletion that cascades through many Organizations.
const ProductScope = ""

type Mark struct {
	ProductID      string
	OrganizationID string
	Revision       int64
}

// Store is a Product's OpenFGA store and the model its tuples follow.
type Store struct {
	StoreID      string
	ModelID      string
	ModelHash    string
	ReconciledAt *time.Time
}

type Repository interface {
	// Mark records, in the caller's transaction, that the store may lag the
	// tables for the Organization, or for the Product with ProductScope.
	Mark(ctx context.Context, productID, organizationID string) error
	// Pending lists a Product's marks.
	Pending(ctx context.Context, productID string) ([]Mark, error)
	// Clear deletes a mark only if no write bumped it since it was read.
	Clear(ctx context.Context, mark Mark) error
	// OldestInternal lists the oldest marks of every Product, for the sweeper.
	OldestInternal(ctx context.Context, limit int64) ([]Mark, error)
	// PostponeInternal moves a Product's marks behind every other's, after a
	// sync of it failed, so one broken Product never starves the rest.
	PostponeInternal(ctx context.Context, productID string, until time.Time) error

	FindStore(ctx context.Context, productID string) (functional.Option[Store], error)
	SetModel(ctx context.Context, productID, modelID, modelHash string) error
	SetReconciled(ctx context.Context, productID string, at time.Time) error
	// DueForReconcileInternal names the Product rebuilt longest ago, if that
	// was before the given time; a Product never rebuilt, with or without a
	// store, comes first.
	DueForReconcileInternal(ctx context.Context, before time.Time) (functional.Option[string], error)
	// ProductExists reports whether the Product is still there.
	ProductExists(ctx context.Context, productID string) (bool, error)

	// Permissions lists the Product's permissions with their scope's ID.
	Permissions(ctx context.Context, productID string) ([]PermissionScope, error)
	// OrganizationIDs lists every Organization of the Product.
	OrganizationIDs(ctx context.Context, productID string) ([]string, error)
	// HeldTemplates reads the role templates a membership still holds itself,
	// as a release before copies wrote, with the permissions each holds.
	HeldTemplates(ctx context.Context, productID string) ([]model.RoleGrants, error)
	// OrganizationState reads what the Organization's tuples come from.
	OrganizationState(
		ctx context.Context,
		productID, organizationID string,
		now time.Time,
	) (model.OrganizationState, error)
}

type PermissionScope struct {
	Name           string
	ResourceTypeID string
}

type repositoryImpl struct {
	db *sql.DB
}

func New(db *sql.DB) Repository {
	return &repositoryImpl{db: db}
}

func (r *repositoryImpl) Mark(ctx context.Context, productID, organizationID string) error {
	stmt := table.AuthzSyncMarks.INSERT(table.AuthzSyncMarks.ProductID, table.AuthzSyncMarks.OrganizationID).
		VALUES(productID, organizationID).
		ON_CONFLICT(table.AuthzSyncMarks.ProductID, table.AuthzSyncMarks.OrganizationID).
		DO_UPDATE(postgres.SET(
			table.AuthzSyncMarks.Revision.SET(postgres.IntExp(postgres.Raw("nextval('authz_sync_mark_revisions')"))),
			table.AuthzSyncMarks.MarkedAt.SET(postgres.TimestampzExp(postgres.NOW())),
		))
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func toMark(row dbmodel.AuthzSyncMarks) Mark {
	return Mark{ProductID: row.ProductID, OrganizationID: row.OrganizationID, Revision: row.Revision}
}

func (r *repositoryImpl) Pending(ctx context.Context, productID string) ([]Mark, error) {
	return transactor.QueryMapSlice(ctx, r.db,
		table.AuthzSyncMarks.SELECT(table.AuthzSyncMarks.AllColumns).
			WHERE(table.AuthzSyncMarks.ProductID.EQ(postgres.String(productID))).
			ORDER_BY(table.AuthzSyncMarks.OrganizationID.ASC()),
		toMark,
	).Value()
}

func (r *repositoryImpl) Clear(ctx context.Context, mark Mark) error {
	return transactor.Exec(ctx, r.db, table.AuthzSyncMarks.DELETE().WHERE(
		table.AuthzSyncMarks.ProductID.EQ(postgres.String(mark.ProductID)).
			AND(table.AuthzSyncMarks.OrganizationID.EQ(postgres.String(mark.OrganizationID))).
			AND(table.AuthzSyncMarks.Revision.EQ(postgres.Int(mark.Revision))),
	)).Err()
}

func (r *repositoryImpl) OldestInternal(ctx context.Context, limit int64) ([]Mark, error) {
	return transactor.QueryMapSlice(ctx, r.db,
		table.AuthzSyncMarks.SELECT(table.AuthzSyncMarks.AllColumns).
			ORDER_BY(table.AuthzSyncMarks.MarkedAt.ASC()).
			LIMIT(limit),
		toMark,
	).Value()
}

func (r *repositoryImpl) PostponeInternal(ctx context.Context, productID string, until time.Time) error {
	return transactor.Exec(ctx, r.db,
		table.AuthzSyncMarks.UPDATE(table.AuthzSyncMarks.MarkedAt).
			SET(postgres.TimestampzT(until)).
			WHERE(table.AuthzSyncMarks.ProductID.EQ(postgres.String(productID))),
	).Err()
}

func (r *repositoryImpl) FindStore(ctx context.Context, productID string) (functional.Option[Store], error) {
	return transactor.QueryOptionalMap(ctx, r.db,
		table.AuthzStores.SELECT(table.AuthzStores.AllColumns).
			WHERE(table.AuthzStores.ProductID.EQ(postgres.String(productID))),
		func(row dbmodel.AuthzStores) Store {
			return Store{
				StoreID:      row.StoreID,
				ModelID:      functional.FromPtr(row.ModelID).OrElse(""),
				ModelHash:    functional.FromPtr(row.ModelHash).OrElse(""),
				ReconciledAt: row.ReconciledAt,
			}
		},
	)
}

func (r *repositoryImpl) SetModel(ctx context.Context, productID, modelID, modelHash string) error {
	return transactor.Exec(ctx, r.db,
		table.AuthzStores.UPDATE(table.AuthzStores.ModelID, table.AuthzStores.ModelHash).
			SET(postgres.String(modelID), postgres.String(modelHash)).
			WHERE(table.AuthzStores.ProductID.EQ(postgres.String(productID))),
	).Err()
}

func (r *repositoryImpl) SetReconciled(ctx context.Context, productID string, at time.Time) error {
	return transactor.Exec(ctx, r.db,
		table.AuthzStores.UPDATE(table.AuthzStores.ReconciledAt).
			SET(postgres.TimestampzT(at)).
			WHERE(table.AuthzStores.ProductID.EQ(postgres.String(productID))),
	).Err()
}

func (r *repositoryImpl) DueForReconcileInternal(
	ctx context.Context, before time.Time,
) (functional.Option[string], error) {
	return transactor.QueryOptionalMap(ctx, r.db,
		postgres.SELECT(table.Products.ID).
			FROM(table.Products.LEFT_JOIN(table.AuthzStores, table.AuthzStores.ProductID.EQ(table.Products.ID))).
			WHERE(table.AuthzStores.ReconciledAt.IS_NULL().
				OR(table.AuthzStores.ReconciledAt.LT(postgres.TimestampzT(before)))).
			ORDER_BY(table.AuthzStores.ReconciledAt.ASC().NULLS_FIRST(), table.Products.ID.ASC()).
			LIMIT(1),
		func(row dbmodel.Products) string { return row.ID },
	)
}

func (r *repositoryImpl) ProductExists(ctx context.Context, productID string) (bool, error) {
	count, err := transactor.QueryCount(ctx, r.db,
		table.Products.SELECT(postgres.COUNT(postgres.STAR)).
			WHERE(table.Products.ID.EQ(postgres.String(productID))),
	).Value()
	return count > 0, err
}

func (r *repositoryImpl) Permissions(ctx context.Context, productID string) ([]PermissionScope, error) {
	return transactor.QueryMapSlice(ctx, r.db,
		table.ProductResourcePermissions.SELECT(
			table.ProductResourcePermissions.Name, table.ProductResourcePermissions.ResourceTypeID,
		).WHERE(table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID))),
		func(row dbmodel.ProductResourcePermissions) PermissionScope {
			return PermissionScope{Name: row.Name, ResourceTypeID: row.ResourceTypeID}
		},
	).Value()
}

func (r *repositoryImpl) OrganizationIDs(ctx context.Context, productID string) ([]string, error) {
	return transactor.QueryMapSlice(ctx, r.db,
		table.Organizations.SELECT(table.Organizations.ID).
			WHERE(table.Organizations.ProductID.EQ(postgres.String(productID))).
			ORDER_BY(table.Organizations.ID.ASC()),
		func(row dbmodel.Organizations) string { return row.ID },
	).Value()
}

func (r *repositoryImpl) OrganizationState(
	ctx context.Context, productID, organizationID string, now time.Time,
) (model.OrganizationState, error) {
	state := model.OrganizationState{OrganizationID: organizationID}
	exists, err := transactor.QueryCount(ctx, r.db,
		table.Organizations.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.Organizations.ID.EQ(postgres.String(organizationID)).
				AND(table.Organizations.ProductID.EQ(postgres.String(productID))),
		),
	).Value()
	if err != nil || exists == 0 {
		return state, err
	}
	state.Exists = true

	memberships, err := transactor.Query[[]dbmodel.OrganizationMemberships](ctx, r.db,
		table.OrganizationMemberships.SELECT(
			table.OrganizationMemberships.ProductUserID, table.OrganizationMemberships.ProductRoleID,
		).WHERE(table.OrganizationMemberships.OrganizationID.EQ(postgres.String(organizationID))),
	).Value()
	if err != nil {
		return state, err
	}
	held := make([]string, 0, len(memberships))
	for _, m := range memberships {
		state.Memberships = append(state.Memberships, model.Membership{
			ProductUserID: m.ProductUserID, RoleRowID: m.ProductRoleID,
		})
		held = append(held, m.ProductRoleID)
	}

	if state.Roles, err = r.roleGrants(ctx, productID, organizationID, held); err != nil {
		return state, err
	}
	state.APIKeys, err = r.apiKeyGrants(ctx, productID, organizationID, now)
	return state, err
}

func (r *repositoryImpl) HeldTemplates(ctx context.Context, productID string) ([]model.RoleGrants, error) {
	held, err := transactor.QueryMapSlice(ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.ID).DISTINCT().
			FROM(table.ProductRoles.INNER_JOIN(table.OrganizationMemberships,
				table.OrganizationMemberships.ProductRoleID.EQ(table.ProductRoles.ID))).
			WHERE(table.ProductRoles.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductRoles.OrganizationID.IS_NULL())),
		func(row dbmodel.ProductRoles) string { return row.ID },
	).Value()
	if err != nil || len(held) == 0 {
		return nil, err
	}
	return r.roleGrants(ctx, productID, "", held)
}

// roleGrants reads the Organization's own roles and every role a membership
// names, which may still be a role template a release before copies wrote.
func (r *repositoryImpl) roleGrants(
	ctx context.Context, productID, organizationID string, held []string,
) ([]model.RoleGrants, error) {
	owned := table.ProductRoles.OrganizationID.EQ(postgres.String(organizationID))
	if len(held) > 0 {
		owned = owned.OR(table.ProductRoles.ID.IN(jetx.ToStringExpressions(functional.Distinct(held))...))
	}
	roles, err := transactor.Query[[]dbmodel.ProductRoles](ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.ID, table.ProductRoles.OrganizationID).WHERE(
			table.ProductRoles.ProductID.EQ(postgres.String(productID)).AND(owned),
		).ORDER_BY(table.ProductRoles.ID.ASC()),
	).Value()
	if err != nil || len(roles) == 0 {
		return nil, err
	}
	roleIDs := functional.Slice(roles).Map(func(row dbmodel.ProductRoles) string { return row.ID })
	ownedBy := functional.Slice(roles).ToMap(func(row dbmodel.ProductRoles) string { return row.ID })
	grants, err := transactor.Query[[]dbmodel.ProductRoleResourcePermissions](ctx, r.db,
		table.ProductRoleResourcePermissions.SELECT(
			table.ProductRoleResourcePermissions.ProductRoleID, table.ProductRoleResourcePermissions.PermissionName,
		).WHERE(table.ProductRoleResourcePermissions.ProductRoleID.IN(jetx.ToStringExpressions(roleIDs)...)),
	).Value()
	if err != nil {
		return nil, err
	}
	byRole := functional.Slice(grants).GroupBy(func(g dbmodel.ProductRoleResourcePermissions) string {
		return g.ProductRoleID
	})
	return functional.Slice(roleIDs).Map(func(id string) model.RoleGrants {
		return model.RoleGrants{
			RowID: id,
			Owned: functional.FromPtr(ownedBy[id].OrganizationID).OrElse("") == organizationID,
			Permissions: functional.Slice(byRole[id]).Map(
				func(g dbmodel.ProductRoleResourcePermissions) string { return g.PermissionName },
			),
		}
	}), nil
}

// apiKeyGrants reads the Organization's active, unexpired API keys.
func (r *repositoryImpl) apiKeyGrants(
	ctx context.Context, productID, organizationID string, now time.Time,
) ([]model.APIKeyGrants, error) {
	keys, err := transactor.Query[[]dbmodel.OrganizationAPIKeys](ctx, r.db,
		table.OrganizationAPIKeys.SELECT(table.OrganizationAPIKeys.ID).WHERE(
			table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(organizationID)).
				AND(table.OrganizationAPIKeys.Status.EQ(postgres.String(string(orgapikey.StatusActive)))).
				AND(table.OrganizationAPIKeys.ExpiresAt.IS_NULL().
					OR(table.OrganizationAPIKeys.ExpiresAt.GT(postgres.TimestampzT(now)))),
		).ORDER_BY(table.OrganizationAPIKeys.ID.ASC()),
	).Value()
	if err != nil || len(keys) == 0 {
		return nil, err
	}
	keyIDs := functional.Slice(keys).Map(func(row dbmodel.OrganizationAPIKeys) string { return row.ID })
	grants, err := transactor.Query[[]dbmodel.OrganizationAPIKeyPermissions](ctx, r.db,
		table.OrganizationAPIKeyPermissions.SELECT(
			table.OrganizationAPIKeyPermissions.APIKeyID, table.OrganizationAPIKeyPermissions.PermissionName,
		).WHERE(
			table.OrganizationAPIKeyPermissions.ProductID.EQ(postgres.String(productID)).
				AND(table.OrganizationAPIKeyPermissions.APIKeyID.IN(jetx.ToStringExpressions(keyIDs)...)),
		),
	).Value()
	if err != nil {
		return nil, err
	}
	byKey := functional.Slice(grants).
		GroupBy(func(g dbmodel.OrganizationAPIKeyPermissions) string { return g.APIKeyID })
	return functional.Slice(keyIDs).Map(func(id string) model.APIKeyGrants {
		return model.APIKeyGrants{ID: id, Permissions: functional.Slice(byKey[id]).Map(
			func(g dbmodel.OrganizationAPIKeyPermissions) string { return g.PermissionName },
		)}
	}), nil
}
