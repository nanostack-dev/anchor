package source

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/model"
	dbmodel "anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/events"
)

// RoleGrants syncs the permissions each role holds, managed or custom: one
// grants tuple per permission, given to every user. An assignment then holds a
// permission when its assignee does and its role grants it.
type RoleGrants struct{ tables Tables }

func NewRoleGrants(tables Tables) RoleGrants { return RoleGrants{tables: tables} }

func (RoleGrants) Name() string { return "role_grants" }

func (RoleGrants) Stale(event events.Event) ([]string, error) {
	if isOneOf(event, events.ProductRoleCreated, events.ProductRoleUpdated, events.ProductRoleDeleted,
		events.OrganizationRoleCreated, events.OrganizationRoleUpdated, events.OrganizationRoleDeleted) {
		return keyOf(event, events.FieldRoleID)
	}
	return nil, nil
}

func (r RoleGrants) Keys(ctx context.Context, productID string) ([]string, error) {
	return transactor.QueryMapSlice(ctx, r.tables.db,
		table.ProductRoles.SELECT(table.ProductRoles.ID).
			WHERE(table.ProductRoles.ProductID.EQ(postgres.String(productID))).
			ORDER_BY(table.ProductRoles.ID.ASC()),
		func(row dbmodel.ProductRoles) string { return row.ID },
	).Value()
}

func (r RoleGrants) Desired(ctx context.Context, productID, roleRowID string) ([]fga.Tuple, error) {
	permissions, err := transactor.QueryMapSlice(ctx, r.tables.db,
		table.ProductRoleResourcePermissions.SELECT(table.ProductRoleResourcePermissions.PermissionName).
			WHERE(table.ProductRoleResourcePermissions.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductRoleResourcePermissions.ProductRoleID.EQ(postgres.String(roleRowID)))).
			ORDER_BY(table.ProductRoleResourcePermissions.PermissionName.ASC()),
		func(row dbmodel.ProductRoleResourcePermissions) string { return row.PermissionName },
	).Value()
	if err != nil {
		return nil, err
	}
	return roleGrantTuples(roleRowID, permissions), nil
}

func (RoleGrants) Stored(ctx context.Context, store fga.Reader, roleRowID string) ([]fga.Tuple, error) {
	return store.Read(ctx, fga.Filter{Object: model.Role(roleRowID)})
}

func roleGrantTuples(roleRowID string, permissions []string) []fga.Tuple {
	tuples := make([]fga.Tuple, 0, len(permissions))
	for _, permission := range permissions {
		tuples = append(tuples, fga.Tuple{
			Object: model.Role(roleRowID), Relation: model.GrantsRelation(permission), User: model.AllUsers,
		})
	}
	return tuples
}
