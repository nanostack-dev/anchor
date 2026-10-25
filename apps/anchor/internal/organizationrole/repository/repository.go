// Package repository stores the roles each Organization owns. They live in
// product_roles beside the role templates, told apart by organization_id.
package repository

import (
	"context"
	"database/sql"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/organizationrole"
	"anchor/internal/mapper"
)

type Repository interface {
	// Resolve reads the role a request names inside an Organization: the
	// Organization's own role with that public ID (a template's ID names its
	// copy), or else the Product's role template itself, for an Organization
	// that has no copy of it yet.
	Resolve(
		ctx context.Context,
		productID, organizationID, roleID string,
	) (functional.Option[organizationrole.Role], error)
	// SyncCopies makes every copy of the template match it: key, name,
	// description, scope and permissions.
	SyncCopies(ctx context.Context, productID, templateID string) error
	// BackfillTemplateKeysInternal gives a key to every template a release
	// before keys wrote. Only the startup backfill calls it.
	BackfillTemplateKeysInternal(ctx context.Context, productID string) (int64, error)
}

type repositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repositoryImpl{db: db}
}

type roleRow struct {
	model.ProductRoles
	Permissions []model.ProductRoleResourcePermissions `alias:"product_role_resource_permissions"`
}

func toDomain(row model.ProductRoles, permissions []model.ProductRoleResourcePermissions) organizationrole.Role {
	return organizationrole.Role{
		RowID:          row.ID,
		ProductID:      row.ProductID,
		OrganizationID: functional.FromPtr(row.OrganizationID).OrElse(""),
		TemplateID:     row.TemplateID,
		Key:            mapper.RoleKey(row),
		Name:           row.Name,
		Description:    functional.FromPtr(row.Description).OrElse(""),
		ResourceTypeID: row.ResourceTypeID,
		Permissions: functional.Slice(permissions).Map(
			func(p model.ProductRoleResourcePermissions) string { return p.PermissionName },
		),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func (r *repositoryImpl) findOne(
	ctx context.Context, where postgres.BoolExpression,
) (functional.Option[organizationrole.Role], error) {
	stmt := postgres.SELECT(
		table.ProductRoles.AllColumns,
		table.ProductRoleResourcePermissions.AllColumns,
	).FROM(
		table.ProductRoles.LEFT_JOIN(
			table.ProductRoleResourcePermissions,
			table.ProductRoles.ID.EQ(table.ProductRoleResourcePermissions.ProductRoleID),
		),
	).WHERE(where)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, func(row roleRow) organizationrole.Role {
		return toDomain(row.ProductRoles, row.Permissions)
	})
}

func (r *repositoryImpl) Resolve(
	ctx context.Context, productID, organizationID, roleID string,
) (functional.Option[organizationrole.Role], error) {
	if roleID == "" {
		return functional.None[organizationrole.Role](), nil
	}
	inProduct := table.ProductRoles.ProductID.EQ(postgres.String(productID))
	own, err := r.findOne(ctx, inProduct.
		AND(table.ProductRoles.OrganizationID.EQ(postgres.String(organizationID))).
		AND(table.ProductRoles.PublicID.EQ(postgres.String(roleID)).OR(table.ProductRoles.ID.EQ(postgres.String(roleID)))),
	)
	if err != nil || own.IsPresent() {
		return own, err
	}
	return r.findOne(ctx, inProduct.
		AND(table.ProductRoles.OrganizationID.IS_NULL()).
		AND(table.ProductRoles.ID.EQ(postgres.String(roleID))))
}

func (r *repositoryImpl) insertPermissions(ctx context.Context, rows []model.ProductRoleResourcePermissions) error {
	if len(rows) == 0 {
		return nil
	}
	return transactor.Exec(ctx, r.db,
		table.ProductRoleResourcePermissions.INSERT(
			table.ProductRoleResourcePermissions.ID, table.ProductRoleResourcePermissions.ProductID,
			table.ProductRoleResourcePermissions.ProductRoleID, table.ProductRoleResourcePermissions.PermissionName,
		).MODELS(rows).ON_CONFLICT().DO_NOTHING(),
	).Err()
}

type copyPermission struct {
	CopyID         string `alias:"copies.id"`
	PermissionName string `alias:"product_role_resource_permissions.permission_name"`
}

func (r *repositoryImpl) SyncCopies(ctx context.Context, productID, templateID string) error {
	template, err := transactor.QueryOptional[model.ProductRoles](ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.AllColumns).WHERE(
			table.ProductRoles.ID.EQ(postgres.String(templateID)).
				AND(table.ProductRoles.ProductID.EQ(postgres.String(productID))).
				AND(table.ProductRoles.OrganizationID.IS_NULL()),
		),
	)
	if err != nil || template.IsAbsent() {
		return err
	}
	tpl := template.Value()
	key := mapper.RoleKey(tpl)
	isCopy := table.ProductRoles.TemplateID.EQ(postgres.String(templateID)).
		AND(table.ProductRoles.ProductID.EQ(postgres.String(productID)))
	synced := model.ProductRoles{
		Key:            &key,
		Name:           tpl.Name,
		Description:    tpl.Description,
		ResourceTypeID: tpl.ResourceTypeID,
	}
	if err = transactor.Exec(ctx, r.db,
		table.ProductRoles.UPDATE(
			table.ProductRoles.Key, table.ProductRoles.Name,
			table.ProductRoles.Description, table.ProductRoles.ResourceTypeID,
		).
			MODEL(synced).
			WHERE(isCopy.AND(
				table.ProductRoles.Key.IS_DISTINCT_FROM(postgres.String(key)).
					OR(table.ProductRoles.Name.IS_DISTINCT_FROM(postgres.String(tpl.Name))).
					OR(table.ProductRoles.Description.IS_DISTINCT_FROM(nullableString(tpl.Description))).
					OR(table.ProductRoles.ResourceTypeID.IS_DISTINCT_FROM(nullableString(tpl.ResourceTypeID))),
			)),
	).Err(); err != nil {
		return err
	}

	granted := table.ProductRoleResourcePermissions.SELECT(table.ProductRoleResourcePermissions.PermissionName).
		WHERE(table.ProductRoleResourcePermissions.ProductRoleID.EQ(postgres.String(templateID)))
	if err = transactor.Exec(ctx, r.db,
		table.ProductRoleResourcePermissions.DELETE().WHERE(
			table.ProductRoleResourcePermissions.ProductRoleID.IN(
				table.ProductRoles.SELECT(table.ProductRoles.ID).WHERE(isCopy),
			).AND(table.ProductRoleResourcePermissions.PermissionName.NOT_IN(granted)),
		),
	).Err(); err != nil {
		return err
	}

	copies := table.ProductRoles.AS("copies")
	held := table.ProductRoleResourcePermissions.AS("held")
	missing, err := transactor.Query[[]copyPermission](ctx, r.db,
		postgres.SELECT(
			copies.ID.AS("copies.id"),
			table.ProductRoleResourcePermissions.PermissionName.AS("product_role_resource_permissions.permission_name"),
		).FROM(
			copies.INNER_JOIN(table.ProductRoleResourcePermissions,
				table.ProductRoleResourcePermissions.ProductRoleID.EQ(postgres.String(templateID))),
		).WHERE(
			copies.TemplateID.EQ(postgres.String(templateID)).
				AND(postgres.NOT(postgres.EXISTS(
					postgres.SELECT(postgres.Int(1)).FROM(held).WHERE(
						held.ProductRoleID.
							EQ(copies.ID).
							AND(held.PermissionName.
								EQ(table.ProductRoleResourcePermissions.PermissionName)),
					),
				))),
		),
	).Value()
	if err != nil {
		return err
	}
	return r.insertPermissions(ctx, functional.Slice(missing).Map(
		func(m copyPermission) model.ProductRoleResourcePermissions {
			return model.ProductRoleResourcePermissions{
				ID:             ids.MustNew("product_role_resource_permission"),
				ProductID:      productID,
				ProductRoleID:  m.CopyID,
				PermissionName: m.PermissionName,
			}
		}))
}

func (r *repositoryImpl) BackfillTemplateKeysInternal(ctx context.Context, productID string) (int64, error) {
	others := table.ProductRoles.AS("others")
	unkeyed := table.ProductRoles.ProductID.EQ(postgres.String(productID)).
		AND(table.ProductRoles.OrganizationID.IS_NULL()).
		AND(table.ProductRoles.Key.IS_NULL())
	byName, err := execRows(ctx, r.db,
		table.ProductRoles.UPDATE(table.ProductRoles.Key).SET(postgres.LOWER(table.ProductRoles.Name)).WHERE(
			unkeyed.AND(postgres.NOT(postgres.EXISTS(
				postgres.SELECT(postgres.Int(1)).FROM(others).WHERE(
					others.ProductID.EQ(postgres.String(productID)).
						AND(others.OrganizationID.IS_NULL()).
						AND(postgres.LOWER(others.Key).
							EQ(postgres.LOWER(table.ProductRoles.Name))),
				),
			))),
		),
	)
	if err != nil {
		return byName, err
	}
	// A name another template already uses as its key falls back to the
	// template's own ID, which nothing else can hold.
	byID, err := execRows(ctx, r.db,
		table.ProductRoles.UPDATE(table.ProductRoles.Key).SET(table.ProductRoles.ID).WHERE(unkeyed),
	)
	return byName + byID, err
}

// nullableString is a value for a comparison; a fresh NULL each time, since
// jet's shared NULL must not be wrapped.
func nullableString(value *string) postgres.StringExpression {
	if value == nil {
		return postgres.StringExp(postgres.Raw("NULL"))
	}
	return postgres.String(*value)
}

func execRows(ctx context.Context, db *sql.DB, stmt postgres.Statement) (int64, error) {
	result, err := stmt.ExecContext(ctx, transactor.Executor(ctx, db))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
