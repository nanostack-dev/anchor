// Package repository stores the resources a product registers in its
// Organizations, and walks their parent chains.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/pgkit/pglock"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/resource"
	"anchor/internal/domain/resourcetype"
)

// Node is one resource of a chain or a subtree, with its distance from the
// resource the walk started at.
type Node struct {
	RowID    string
	Ref      resource.Ref
	Distance int
}

type Repository interface {
	WorkspaceMirrors
	// LockOrganization holds the Organization's resources until the
	// transaction ends, so registrations, moves and deletes in one
	// Organization run one at a time and each sees the chains the previous
	// one left.
	LockOrganization(ctx context.Context, organizationID string) error
	// HoldOrganization reads the Organization FOR KEY SHARE, so it can't be
	// deleted before the transaction ends, and reports whether it exists.
	HoldOrganization(ctx context.Context, productID, organizationID string) (bool, error)
	// ShareOrganization holds the Organization's resources in share mode until
	// the transaction ends: role assignment writes run side by side, and never
	// beside a registration, a move or a delete.
	ShareOrganization(ctx context.Context, organizationID string) error
	Find(ctx context.Context, organizationID, typeID, externalID string) (functional.Option[resource.Resource], error)
	// RegisteredElsewhere reports whether another Organization of the
	// Product registered a resource of that type and identifier.
	RegisteredElsewhere(ctx context.Context, productID, organizationID, typeID, externalID string) (bool, error)
	Create(ctx context.Context, r resource.Resource) error
	Update(ctx context.Context, organizationID, rowID, name string, parentRowID *string) error
	// Ancestors walks up from the resource, itself first, and stops after
	// limit resources.
	Ancestors(ctx context.Context, organizationID, rowID string, limit int) ([]Node, error)
	// Subtree lists the resource and every descendant, deepest first.
	Subtree(ctx context.Context, organizationID, rowID string) ([]Node, error)
	// SubtreeHeight is how many levels sit below the resource: 0 for a leaf.
	SubtreeHeight(ctx context.Context, organizationID, rowID string) (int, error)
	CountChildren(ctx context.Context, organizationID, rowID string) (int, error)
	// Delete removes the resource; its descendants go with it through the
	// parent foreign key.
	Delete(ctx context.Context, organizationID, rowID string) error
	Search(
		ctx context.Context, productID, organizationID string,
		request search.Request[resource.Filter, resource.SortField],
	) (search.Result[resource.Resource], error)
}

type repositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repositoryImpl{db: db}
}

// resourceRow is a resource with the slug of its type and the reference of
// its parent, read in the same statement.
type resourceRow struct {
	model.Resources

	Type             string  `alias:"resource_type.slug"`
	ParentExternalID *string `alias:"parent.external_id"`
	ParentType       *string `alias:"parent_type.slug"`
}

func (row resourceRow) toDomain() resource.Resource {
	parent := resource.OrganizationRef(row.OrganizationID)
	if row.ParentResourceID != nil {
		parent = resource.Ref{
			Type: functional.FromPtr(row.ParentType).OrElse(""),
			ID:   functional.FromPtr(row.ParentExternalID).OrElse(""),
		}
	}
	return resource.Resource{
		RowID:          row.ID,
		ProductID:      row.ProductID,
		OrganizationID: row.OrganizationID,
		TypeID:         row.ResourceTypeID,
		Type:           row.Type,
		ExternalID:     row.ExternalID,
		Name:           row.Name,
		ParentRowID:    row.ParentResourceID,
		Parent:         parent,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

// aliases names the tables a resource read joins: its type, its parent, and
// its parent's type.
func aliases() (*table.ProductResourceTypesTable, *table.ResourcesTable, *table.ProductResourceTypesTable) {
	return table.ProductResourceTypes.AS("resource_type"),
		table.Resources.AS("parent"),
		table.ProductResourceTypes.AS("parent_type")
}

func resourceColumns() []postgres.Projection {
	resourceTypes, parents, parentTypes := aliases()
	return []postgres.Projection{
		table.Resources.AllColumns,
		resourceTypes.Slug,
		parents.ExternalID,
		parentTypes.Slug,
	}
}

func resourceJoins() postgres.ReadableTable {
	resourceTypes, parents, parentTypes := aliases()
	return table.Resources.
		INNER_JOIN(resourceTypes, resourceTypes.ID.EQ(table.Resources.ResourceTypeID)).
		LEFT_JOIN(parents, parents.ID.EQ(table.Resources.ParentResourceID)).
		LEFT_JOIN(parentTypes, parentTypes.ID.EQ(parents.ResourceTypeID))
}

func (r *repositoryImpl) LockOrganization(ctx context.Context, organizationID string) error {
	_, err := transactor.Executor(ctx, r.db).ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`,
		pglock.KeyHash("resources:"+organizationID))
	return err
}

func (r *repositoryImpl) HoldOrganization(ctx context.Context, productID, organizationID string) (bool, error) {
	found, err := transactor.QueryOptional[model.Organizations](
		transactor.WithRowLock(ctx, postgres.KEY_SHARE()), r.db,
		table.Organizations.SELECT(table.Organizations.ID).WHERE(
			table.Organizations.ID.EQ(postgres.String(organizationID)).
				AND(table.Organizations.ProductID.EQ(postgres.String(productID))),
		),
	)
	return found.IsPresent(), err
}

func (r *repositoryImpl) ShareOrganization(ctx context.Context, organizationID string) error {
	_, err := transactor.Executor(ctx, r.db).ExecContext(ctx, `SELECT pg_advisory_xact_lock_shared($1)`,
		pglock.KeyHash("resources:"+organizationID))
	return err
}

func (r *repositoryImpl) Find(
	ctx context.Context, organizationID, typeID, externalID string,
) (functional.Option[resource.Resource], error) {
	columns := resourceColumns()
	return transactor.QueryOptionalMap(ctx, r.db,
		postgres.SELECT(columns[0], columns[1:]...).
			FROM(resourceJoins()).
			WHERE(
				table.Resources.OrganizationID.EQ(postgres.String(organizationID)).
					AND(table.Resources.ResourceTypeID.EQ(postgres.String(typeID))).
					AND(table.Resources.ExternalID.EQ(postgres.String(externalID))),
			),
		resourceRow.toDomain,
	)
}

func (r *repositoryImpl) RegisteredElsewhere(
	ctx context.Context, productID, organizationID, typeID, externalID string,
) (bool, error) {
	found, err := transactor.QueryOptional[model.Resources](ctx, r.db,
		table.Resources.SELECT(table.Resources.ID).WHERE(
			table.Resources.ProductID.EQ(postgres.String(productID)).
				AND(table.Resources.ResourceTypeID.EQ(postgres.String(typeID))).
				AND(table.Resources.ExternalID.EQ(postgres.String(externalID))).
				AND(table.Resources.OrganizationID.NOT_EQ(postgres.String(organizationID))),
		).LIMIT(1),
	)
	return found.IsPresent(), err
}

func (r *repositoryImpl) Create(ctx context.Context, res resource.Resource) error {
	stmt := table.Resources.INSERT(
		table.Resources.ID, table.Resources.ProductID, table.Resources.OrganizationID,
		table.Resources.ResourceTypeID, table.Resources.ExternalID, table.Resources.Name,
		table.Resources.ParentResourceID,
	).VALUES(
		res.RowID, res.ProductID, res.OrganizationID, res.TypeID, res.ExternalID, res.Name, res.ParentRowID,
	)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *repositoryImpl) Update(
	ctx context.Context, organizationID, rowID, name string, parentRowID *string,
) error {
	stmt := table.Resources.UPDATE(table.Resources.Name, table.Resources.ParentResourceID).
		SET(name, parentRowID).
		WHERE(
			table.Resources.ID.EQ(postgres.String(rowID)).
				AND(table.Resources.OrganizationID.EQ(postgres.String(organizationID))),
		)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

const ancestorsQuery = `
WITH RECURSIVE chain AS (
    SELECT id, parent_resource_id, resource_type_id, external_id, 0 AS distance
    FROM resources
    WHERE id = $1 AND organization_id = $2
    UNION ALL
    SELECT r.id, r.parent_resource_id, r.resource_type_id, r.external_id, chain.distance + 1
    FROM resources r
    JOIN chain ON r.id = chain.parent_resource_id
    WHERE chain.distance + 1 < $3
)
SELECT chain.id, t.slug, chain.external_id, chain.distance
FROM chain
JOIN product_resource_types t ON t.id = chain.resource_type_id
ORDER BY chain.distance`

func (r *repositoryImpl) Ancestors(
	ctx context.Context, organizationID, rowID string, limit int,
) ([]Node, error) {
	return r.nodes(ctx, ancestorsQuery, rowID, organizationID, limit)
}

// subtreeQuery has no depth guard: a stored chain never loops, because every
// move is checked against its new parent's ancestors under the Organization
// lock.
const subtreeQuery = `
WITH RECURSIVE subtree AS (
    SELECT id, resource_type_id, external_id, 0 AS distance
    FROM resources
    WHERE id = $1 AND organization_id = $2
    UNION ALL
    SELECT r.id, r.resource_type_id, r.external_id, subtree.distance + 1
    FROM resources r
    JOIN subtree ON r.parent_resource_id = subtree.id
)
SELECT subtree.id, t.slug, subtree.external_id, subtree.distance
FROM subtree
JOIN product_resource_types t ON t.id = subtree.resource_type_id
ORDER BY subtree.distance DESC, subtree.id`

func (r *repositoryImpl) Subtree(ctx context.Context, organizationID, rowID string) ([]Node, error) {
	return r.nodes(ctx, subtreeQuery, rowID, organizationID)
}

const subtreeHeightQuery = `
WITH RECURSIVE subtree AS (
    SELECT id, 0 AS distance
    FROM resources
    WHERE id = $1 AND organization_id = $2
    UNION ALL
    SELECT r.id, subtree.distance + 1
    FROM resources r
    JOIN subtree ON r.parent_resource_id = subtree.id
)
SELECT COALESCE(max(distance), 0) FROM subtree`

func (r *repositoryImpl) SubtreeHeight(ctx context.Context, organizationID, rowID string) (int, error) {
	rows, err := transactor.Executor(ctx, r.db).QueryContext(ctx, subtreeHeightQuery, rowID, organizationID)
	if err != nil {
		return 0, fmt.Errorf("measure resource subtree: %w", err)
	}
	defer func() { _ = rows.Close() }()
	height := 0
	if rows.Next() {
		if err = rows.Scan(&height); err != nil {
			return 0, err
		}
	}
	return height, rows.Err()
}

func (r *repositoryImpl) nodes(ctx context.Context, query string, args ...any) ([]Node, error) {
	rows, err := transactor.Executor(ctx, r.db).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("walk resources: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var nodes []Node
	for rows.Next() {
		var node Node
		if err = rows.Scan(&node.RowID, &node.Ref.Type, &node.Ref.ID, &node.Distance); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

func (r *repositoryImpl) CountChildren(ctx context.Context, organizationID, rowID string) (int, error) {
	count, err := transactor.QueryCount(ctx, r.db,
		table.Resources.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.Resources.OrganizationID.EQ(postgres.String(organizationID)).
				AND(table.Resources.ParentResourceID.EQ(postgres.String(rowID))),
		),
	).Value()
	return int(count), err
}

func (r *repositoryImpl) Delete(ctx context.Context, organizationID, rowID string) error {
	return transactor.Exec(ctx, r.db, table.Resources.DELETE().WHERE(
		table.Resources.ID.EQ(postgres.String(rowID)).
			AND(table.Resources.OrganizationID.EQ(postgres.String(organizationID))),
	)).Err()
}

func (r *repositoryImpl) Search(
	ctx context.Context, productID, organizationID string,
	request search.Request[resource.Filter, resource.SortField],
) (search.Result[resource.Resource], error) {
	order, err := transactor.StrictKeys(
		request.Sort,
		map[resource.SortField]transactor.Key[resourceRow]{
			resource.SortFieldCreatedAt: transactor.TimeKey(
				table.Resources.CreatedAt, func(row resourceRow) time.Time { return row.CreatedAt },
			),
			resource.SortFieldName: transactor.StringKey(
				table.Resources.Name, func(row resourceRow) string { return row.Name },
			),
		},
		transactor.StringKey(table.Resources.ExternalID, func(row resourceRow) string { return row.ExternalID }),
		transactor.StringKey(table.Resources.ID, func(row resourceRow) string { return row.ID }),
	)
	if err != nil {
		return search.Result[resource.Resource]{}, err
	}

	where := table.Resources.OrganizationID.EQ(postgres.String(organizationID)).
		AND(table.Resources.ProductID.EQ(postgres.String(productID)))
	if request.Filter != nil {
		where = where.AND(filterCondition(productID, organizationID, *request.Filter))
	}

	page, err := transactor.Page(r.db, resourceRow.toDomain, resourceColumns()...).
		From(resourceJoins()).
		Where(where).
		Keyset(order).
		Run(ctx, request.Pagination).
		Value()
	if err != nil {
		return search.Result[resource.Resource]{}, err
	}
	return search.Result[resource.Resource]{
		Items: page.Items, Total: page.Total, Count: len(page.Items), NextCursor: page.NextCursor,
	}, nil
}

// filterCondition resolves the slugs and the parent reference inside the
// product and the Organization, so a slug or a parent it lacks matches
// nothing instead of widening the search.
func filterCondition(productID, organizationID string, filter resource.Filter) postgres.BoolExpression {
	condition := postgres.Bool(true)
	if len(filter.ResourceTypes) > 0 {
		condition = condition.AND(table.Resources.ResourceTypeID.IN(
			table.ProductResourceTypes.SELECT(table.ProductResourceTypes.ID).WHERE(
				table.ProductResourceTypes.ProductID.EQ(postgres.String(productID)).
					AND(table.ProductResourceTypes.Slug.IN(jetx.ToStringExpressions(filter.ResourceTypes)...)),
			),
		))
	}
	if filter.Parent != nil {
		condition = condition.AND(parentCondition(productID, organizationID, *filter.Parent))
	}
	if len(filter.IDs) > 0 {
		condition = condition.AND(table.Resources.ExternalID.IN(jetx.ToStringExpressions(filter.IDs)...))
	}
	if filter.NameContains != nil {
		pattern := "%" + escapeLike(strings.ToLower(*filter.NameContains)) + "%"
		condition = condition.AND(postgres.LOWER(table.Resources.Name).LIKE(postgres.String(pattern)))
	}
	return condition
}

func parentCondition(productID, organizationID string, parent resource.Ref) postgres.BoolExpression {
	if parent.Type == resourcetype.OrganizationSlug {
		if parent.ID != organizationID {
			return postgres.Bool(false)
		}
		return table.Resources.ParentResourceID.IS_NULL()
	}
	candidates := table.Resources.AS("candidate")
	return table.Resources.ParentResourceID.IN(
		candidates.SELECT(candidates.ID).WHERE(
			candidates.OrganizationID.EQ(postgres.String(organizationID)).
				AND(candidates.ExternalID.EQ(postgres.String(parent.ID))).
				AND(candidates.ResourceTypeID.IN(
					table.ProductResourceTypes.SELECT(table.ProductResourceTypes.ID).WHERE(
						table.ProductResourceTypes.ProductID.EQ(postgres.String(productID)).
							AND(table.ProductResourceTypes.Slug.EQ(postgres.String(parent.Type))),
					),
				)),
		),
	)
}

// escapeLike makes % and _ match themselves under PostgreSQL's default LIKE
// escape character.
func escapeLike(text string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(text)
}
