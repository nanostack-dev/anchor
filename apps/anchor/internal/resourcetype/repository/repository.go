// Package repository stores a product's resource types and their parent edges.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/resourcetype"
)

var (
	// ErrSlugExists is returned when a create trips the (product_id, slug)
	// unique constraint: the guard settles the race the service pre-check cannot.
	ErrSlugExists = errors.New("resource type slug already exists in product")
	// ErrInUse is returned when a delete trips a foreign key: a role,
	// permission or child type created after the service counted usage.
	ErrInUse = errors.New("resource type is still referenced")
)

const slugUniqueConstraint = "product_resource_types_product_id_slug_key"

// RolePermissionScopes is one role of a product with the scopes of the permissions it
// holds, all as resource type IDs.
type RolePermissionScopes struct {
	RoleID            string
	ResourceTypeID    string
	PermissionTypeIDs []string
}

type Repository interface {
	// FindTree returns every type of the product, organization included.
	FindTree(ctx context.Context, productID string) ([]resourcetype.ResourceType, error)
	// EnsureOrganization seeds the built-in root type when the product lacks it.
	EnsureOrganization(ctx context.Context, productID string) error
	// FindOrganization reads the product's organization type row; the caller
	// passes the row lock that guards its catalog write.
	FindOrganization(ctx context.Context, productID string) error
	Create(ctx context.Context, rt resourcetype.ResourceType) error
	Update(ctx context.Context, rt resourcetype.ResourceType) error
	Delete(ctx context.Context, productID string, ids ...string) error
	Search(
		ctx context.Context, productID string,
		request search.Request[resourcetype.Filter, resourcetype.SortField],
	) (search.Result[resourcetype.ResourceType], error)
	CountUsage(ctx context.Context, productID, id string) (resourcetype.Usage, error)
	// CountResourcesUnder counts the registered resources of a type whose
	// parent is of the given type; organization counts those at the top.
	CountResourcesUnder(ctx context.Context, productID, id string, parent resourcetype.ResourceType) (int, error)
	FindRolePermissionScopes(ctx context.Context, productID string) ([]RolePermissionScopes, error)
}

type repositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repositoryImpl{db: db}
}

func (r *repositoryImpl) FindTree(ctx context.Context, productID string) ([]resourcetype.ResourceType, error) {
	rows, err := transactor.Query[[]model.ProductResourceTypes](ctx, r.db,
		table.ProductResourceTypes.SELECT(table.ProductResourceTypes.AllColumns).
			WHERE(table.ProductResourceTypes.ProductID.EQ(postgres.String(productID))).
			ORDER_BY(table.ProductResourceTypes.CreatedAt.ASC(), table.ProductResourceTypes.ID.ASC()),
	).Value()
	if err != nil {
		return nil, err
	}
	return r.withParents(ctx, rows)
}

func (r *repositoryImpl) EnsureOrganization(ctx context.Context, productID string) error {
	root := resourcetype.NewOrganization(productID)
	stmt := table.ProductResourceTypes.INSERT(
		table.ProductResourceTypes.ID, table.ProductResourceTypes.ProductID,
		table.ProductResourceTypes.Slug, table.ProductResourceTypes.Name,
		table.ProductResourceTypes.AllowsCustomRoles,
	).VALUES(root.ID, root.ProductID, root.Slug, root.Name, root.AllowsCustomRoles).
		ON_CONFLICT(table.ProductResourceTypes.ProductID, table.ProductResourceTypes.Slug).
		DO_NOTHING()
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *repositoryImpl) FindOrganization(ctx context.Context, productID string) error {
	_, err := transactor.QueryOptional[model.ProductResourceTypes](
		ctx, r.db,
		table.ProductResourceTypes.SELECT(table.ProductResourceTypes.ID).WHERE(
			table.ProductResourceTypes.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductResourceTypes.Slug.EQ(postgres.String(resourcetype.OrganizationSlug))),
		),
	)
	return err
}

func (r *repositoryImpl) Create(ctx context.Context, rt resourcetype.ResourceType) error {
	stmt := table.ProductResourceTypes.INSERT(
		table.ProductResourceTypes.ID, table.ProductResourceTypes.ProductID,
		table.ProductResourceTypes.Slug, table.ProductResourceTypes.Name,
		table.ProductResourceTypes.Description, table.ProductResourceTypes.NestsInItself,
		table.ProductResourceTypes.AllowsCustomRoles,
	).VALUES(rt.ID, rt.ProductID, rt.Slug, rt.Name, rt.Description, rt.NestsInItself, rt.AllowsCustomRoles)
	if err := transactor.Exec(ctx, r.db, stmt).OnUnique(ErrSlugExists, slugUniqueConstraint).Err(); err != nil {
		return err
	}
	return r.insertParents(ctx, rt)
}

func (r *repositoryImpl) Update(ctx context.Context, rt resourcetype.ResourceType) error {
	stmt := table.ProductResourceTypes.UPDATE(
		table.ProductResourceTypes.Name, table.ProductResourceTypes.Description,
		table.ProductResourceTypes.NestsInItself, table.ProductResourceTypes.AllowsCustomRoles,
	).SET(rt.Name, rt.Description, rt.NestsInItself, rt.AllowsCustomRoles).WHERE(
		table.ProductResourceTypes.ID.EQ(postgres.String(rt.ID)).
			AND(table.ProductResourceTypes.ProductID.EQ(postgres.String(rt.ProductID))),
	)
	if err := transactor.Exec(ctx, r.db, stmt).Err(); err != nil {
		return err
	}
	if err := transactor.Exec(ctx, r.db, table.ProductResourceTypeParents.DELETE().WHERE(
		table.ProductResourceTypeParents.ResourceTypeID.EQ(postgres.String(rt.ID)),
	)).Err(); err != nil {
		return err
	}
	return r.insertParents(ctx, rt)
}

// insertParents resolves the parent slugs inside the product, so an edge can
// never point at another product's type.
func (r *repositoryImpl) insertParents(ctx context.Context, rt resourcetype.ResourceType) error {
	if len(rt.ParentTypes) == 0 {
		return nil
	}
	stmt := table.ProductResourceTypeParents.INSERT(
		table.ProductResourceTypeParents.ResourceTypeID, table.ProductResourceTypeParents.ParentTypeID,
	).QUERY(
		postgres.SELECT(postgres.String(rt.ID), table.ProductResourceTypes.ID).
			FROM(table.ProductResourceTypes).
			WHERE(
				table.ProductResourceTypes.ProductID.EQ(postgres.String(rt.ProductID)).
					AND(table.ProductResourceTypes.Slug.IN(jetx.ToStringExpressions(rt.ParentTypes)...)),
			),
	)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

// Delete removes the types and every edge that touches them. The caller has
// checked that no surviving type sits under one of them.
func (r *repositoryImpl) Delete(ctx context.Context, productID string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	idList := jetx.ToStringExpressions(ids)
	if err := transactor.Exec(ctx, r.db, table.ProductResourceTypeParents.DELETE().WHERE(
		table.ProductResourceTypeParents.ResourceTypeID.IN(idList...).
			OR(table.ProductResourceTypeParents.ParentTypeID.IN(idList...)),
	)).Err(); err != nil {
		return err
	}
	stmt := table.ProductResourceTypes.DELETE().WHERE(
		table.ProductResourceTypes.ID.IN(idList...).
			AND(table.ProductResourceTypes.ProductID.EQ(postgres.String(productID))),
	)
	return transactor.Exec(ctx, r.db, stmt).OnForeignKey(ErrInUse).Err()
}

func (r *repositoryImpl) Search(
	ctx context.Context, productID string,
	request search.Request[resourcetype.Filter, resourcetype.SortField],
) (search.Result[resourcetype.ResourceType], error) {
	where := table.ProductResourceTypes.ProductID.EQ(postgres.String(productID))
	if request.FullTextSearch != nil {
		pattern := postgres.String("%" + strings.ToLower(*request.FullTextSearch) + "%")
		where = where.AND(
			postgres.LOWER(table.ProductResourceTypes.Name).LIKE(pattern).
				OR(table.ProductResourceTypes.Slug.LIKE(pattern)),
		)
	}
	if request.Filter != nil {
		if len(request.Filter.IDs) > 0 {
			where = where.AND(table.ProductResourceTypes.ID.IN(jetx.ToStringExpressions(request.Filter.IDs)...))
		}
		if len(request.Filter.Slugs) > 0 {
			where = where.AND(table.ProductResourceTypes.Slug.IN(jetx.ToStringExpressions(request.Filter.Slugs)...))
		}
	}

	idKey := transactor.StringKey(
		table.ProductResourceTypes.ID,
		func(row model.ProductResourceTypes) string { return row.ID },
	)
	order, err := transactor.StrictKeys(
		request.Sort,
		map[resourcetype.SortField]transactor.Key[model.ProductResourceTypes]{
			resourcetype.SortFieldCreatedAt: transactor.TimeKey(
				table.ProductResourceTypes.CreatedAt,
				func(row model.ProductResourceTypes) time.Time { return row.CreatedAt },
			),
			resourcetype.SortFieldUpdatedAt: transactor.TimeKey(
				table.ProductResourceTypes.UpdatedAt,
				func(row model.ProductResourceTypes) time.Time { return row.UpdatedAt },
			),
			resourcetype.SortFieldSlug: transactor.StringKey(
				table.ProductResourceTypes.Slug, func(row model.ProductResourceTypes) string { return row.Slug },
			),
			resourcetype.SortFieldName: transactor.StringKey(
				table.ProductResourceTypes.Name, func(row model.ProductResourceTypes) string { return row.Name },
			),
		},
		idKey,
	)
	if err != nil {
		return search.Result[resourcetype.ResourceType]{}, err
	}

	page, err := transactor.Page(
		r.db, func(row model.ProductResourceTypes) model.ProductResourceTypes { return row },
		table.ProductResourceTypes.AllColumns,
	).
		From(table.ProductResourceTypes).
		Where(where).
		Keyset(order).
		Run(ctx, request.Pagination).
		Value()
	if err != nil {
		return search.Result[resourcetype.ResourceType]{}, err
	}
	items, err := r.withParents(ctx, page.Items)
	if err != nil {
		return search.Result[resourcetype.ResourceType]{}, err
	}
	return search.Result[resourcetype.ResourceType]{
		Items: items, Total: page.Total, Count: len(items), NextCursor: page.NextCursor,
	}, nil
}

type parentEdge struct {
	ResourceTypeID string `alias:"product_resource_type_parents.resource_type_id"`
	ParentSlug     string `alias:"product_resource_types.slug"`
}

// withParents fills the parent slugs of the given types in one statement.
func (r *repositoryImpl) withParents(
	ctx context.Context, rows []model.ProductResourceTypes,
) ([]resourcetype.ResourceType, error) {
	if len(rows) == 0 {
		return []resourcetype.ResourceType{}, nil
	}
	ids := functional.Slice(rows).Map(func(row model.ProductResourceTypes) string { return row.ID })
	edges, err := transactor.Query[[]parentEdge](ctx, r.db,
		postgres.SELECT(
			table.ProductResourceTypeParents.ResourceTypeID.AS("product_resource_type_parents.resource_type_id"),
			table.ProductResourceTypes.Slug.AS("product_resource_types.slug"),
		).FROM(
			table.ProductResourceTypeParents.INNER_JOIN(
				table.ProductResourceTypes,
				table.ProductResourceTypes.ID.EQ(table.ProductResourceTypeParents.ParentTypeID),
			),
		).WHERE(
			table.ProductResourceTypeParents.ResourceTypeID.IN(jetx.ToStringExpressions(ids)...),
		).ORDER_BY(table.ProductResourceTypes.Slug.ASC()),
	).Value()
	if err != nil {
		return nil, err
	}
	parents := functional.Slice(edges).GroupBy(func(edge parentEdge) string { return edge.ResourceTypeID })
	return functional.Slice(rows).Map(func(row model.ProductResourceTypes) resourcetype.ResourceType {
		return resourcetype.ResourceType{
			ID:            row.ID,
			ProductID:     row.ProductID,
			Slug:          row.Slug,
			Name:          row.Name,
			Description:   row.Description,
			NestsInItself: row.NestsInItself,
			// NULL is a type the release before the switch wrote: off.
			AllowsCustomRoles: functional.FromPtr(row.AllowsCustomRoles).OrElse(false),
			ParentTypes: functional.Slice(parents[row.ID]).
				Map(func(edge parentEdge) string { return edge.ParentSlug }),
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		}
	}), nil
}

func (r *repositoryImpl) CountUsage(ctx context.Context, productID, id string) (resourcetype.Usage, error) {
	children, err := transactor.QueryCount(ctx, r.db,
		table.ProductResourceTypeParents.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.ProductResourceTypeParents.ParentTypeID.EQ(postgres.String(id)),
		),
	).Value()
	if err != nil {
		return resourcetype.Usage{}, err
	}
	roles, err := transactor.QueryCount(ctx, r.db,
		table.ProductRoles.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.ProductRoles.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductRoles.ResourceTypeID.EQ(postgres.String(id))),
		),
	).Value()
	if err != nil {
		return resourcetype.Usage{}, err
	}
	permissions, err := transactor.QueryCount(ctx, r.db,
		table.ProductResourcePermissions.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductResourcePermissions.ResourceTypeID.EQ(postgres.String(id))),
		),
	).Value()
	if err != nil {
		return resourcetype.Usage{}, err
	}
	resources, err := transactor.QueryCount(ctx, r.db,
		table.Resources.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.Resources.ProductID.EQ(postgres.String(productID)).
				AND(table.Resources.ResourceTypeID.EQ(postgres.String(id))),
		),
	).Value()
	if err != nil {
		return resourcetype.Usage{}, err
	}
	return resourcetype.Usage{
		ChildTypes: int(children), Roles: int(roles), Permissions: int(permissions), Resources: int(resources),
	}, nil
}

func (r *repositoryImpl) CountResourcesUnder(
	ctx context.Context, productID, id string, parent resourcetype.ResourceType,
) (int, error) {
	where := table.Resources.ProductID.EQ(postgres.String(productID)).
		AND(table.Resources.ResourceTypeID.EQ(postgres.String(id)))
	if parent.BuiltIn() {
		where = where.AND(table.Resources.ParentResourceID.IS_NULL())
	} else {
		parents := table.Resources.AS("parent")
		where = where.AND(table.Resources.ParentResourceID.IN(
			parents.SELECT(parents.ID).WHERE(parents.ResourceTypeID.EQ(postgres.String(parent.ID))),
		))
	}
	count, err := transactor.QueryCount(ctx, r.db,
		table.Resources.SELECT(postgres.COUNT(postgres.STAR)).WHERE(where),
	).Value()
	return int(count), err
}

type rolePermissionScopesRow struct {
	RoleID           string `alias:"product_roles.id"`
	RoleTypeID       string `alias:"product_roles.resource_type_id"`
	PermissionTypeID string `alias:"product_resource_permissions.resource_type_id"`
}

func (r *repositoryImpl) FindRolePermissionScopes(
	ctx context.Context,
	productID string,
) ([]RolePermissionScopes, error) {
	rows, err := transactor.Query[[]rolePermissionScopesRow](ctx, r.db,
		postgres.SELECT(
			table.ProductRoles.ID.AS("product_roles.id"),
			table.ProductRoles.ResourceTypeID.AS("product_roles.resource_type_id"),
			table.ProductResourcePermissions.ResourceTypeID.AS("product_resource_permissions.resource_type_id"),
		).FROM(
			table.ProductRoles.
				INNER_JOIN(table.ProductRoleResourcePermissions,
					table.ProductRoleResourcePermissions.ProductRoleID.EQ(table.ProductRoles.ID)).
				INNER_JOIN(table.ProductResourcePermissions,
					table.ProductResourcePermissions.ProductID.EQ(table.ProductRoleResourcePermissions.ProductID).
						AND(table.ProductResourcePermissions.Name.EQ(table.ProductRoleResourcePermissions.PermissionName)),
				),
		).WHERE(
			table.ProductRoles.ProductID.EQ(postgres.String(productID)),
		).ORDER_BY(table.ProductRoles.ID.ASC()),
	).Value()
	if err != nil {
		return nil, err
	}
	grouped := functional.Slice(rows).GroupBy(func(row rolePermissionScopesRow) string { return row.RoleID })
	scopes := make([]RolePermissionScopes, 0, len(grouped))
	for roleID, permissions := range grouped {
		scopes = append(scopes, RolePermissionScopes{
			RoleID:         roleID,
			ResourceTypeID: permissions[0].RoleTypeID,
			PermissionTypeIDs: functional.Slice(permissions).Map(
				func(row rolePermissionScopesRow) string { return row.PermissionTypeID },
			),
		})
	}
	return scopes, nil
}
