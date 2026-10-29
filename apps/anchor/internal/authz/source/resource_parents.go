package source

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/events"
)

// ResourceParents syncs where each registered resource sits: one parent tuple
// to its parent resource, or to its Organization at the top. A check on a
// resource then walks up to the Organization's assignments. Keyed by the
// resource's object, rt_<slug>:<row ID>, so a write reads that one resource,
// never the Organization's whole registry.
type ResourceParents struct{ tables Tables }

func NewResourceParents(tables Tables) ResourceParents { return ResourceParents{tables: tables} }

func (ResourceParents) Name() string { return "resource_parents" }

func (ResourceParents) Stale(event events.Event) ([]string, error) {
	// A move rewrites the resource's own parent tuple; its children keep
	// theirs. A cascading delete emits one event per resource it removes. A
	// deleted Organization's resources cascade without events, and no key
	// names them any more: the next rebuild deletes their tuples.
	if !isOneOf(event, events.ResourceCreated, events.ResourceUpdated, events.ResourceDeleted) {
		return nil, nil
	}
	slug, err := keyOf(event, events.FieldResourceType)
	if err != nil {
		return nil, err
	}
	rowID, err := keyOf(event, events.FieldResourceRowID)
	if err != nil {
		return nil, err
	}
	return []string{model.ResourceObject(slug[0], rowID[0])}, nil
}

func (r ResourceParents) Keys(ctx context.Context, productID string) ([]string, error) {
	rows, err := r.tables.resources(ctx, table.Resources.ProductID.EQ(postgres.String(productID)))
	return functional.Slice(rows).Map(resourceRow.object), err
}

func (r ResourceParents) Desired(ctx context.Context, productID, object string) ([]fga.Tuple, error) {
	slug, rowID, ok := model.ParseResourceObject(object)
	if !ok {
		return nil, nil
	}
	rows, err := r.tables.resources(ctx, table.Resources.ProductID.EQ(postgres.String(productID)).
		AND(table.Resources.ID.EQ(postgres.String(rowID))))
	if err != nil {
		return nil, err
	}
	var tuples []fga.Tuple
	for _, row := range rows {
		if row.Type == slug {
			tuples = append(tuples, resourceParentTuple(row))
		}
	}
	return tuples, nil
}

func (ResourceParents) Stored(ctx context.Context, store fga.Reader, object string) ([]fga.Tuple, error) {
	return store.Read(ctx, fga.Filter{Object: object, Relation: model.RelationParent})
}

// resourceRow is a registered resource with its type's slug and its parent's.
// An empty ParentID means the Organization is the parent.
type resourceRow struct {
	ID             string  `alias:"resource_row.id"`
	OrganizationID string  `alias:"resource_row.organization_id"`
	Type           string  `alias:"resource_row.type"`
	ParentID       *string `alias:"resource_row.parent_id"`
	ParentType     *string `alias:"resource_row.parent_type"`
}

func (row resourceRow) object() string { return model.ResourceObject(row.Type, row.ID) }

func resourceParentTuple(row resourceRow) fga.Tuple {
	parent := model.Organization(row.OrganizationID)
	if row.ParentID != nil && row.ParentType != nil {
		parent = model.ResourceObject(*row.ParentType, *row.ParentID)
	}
	return fga.Tuple{Object: row.object(), Relation: model.RelationParent, User: parent}
}

func (t Tables) resources(ctx context.Context, where postgres.BoolExpression) ([]resourceRow, error) {
	types := table.ProductResourceTypes.AS("resource_type")
	parents := table.Resources.AS("parent")
	parentTypes := table.ProductResourceTypes.AS("parent_type")
	return transactor.Query[[]resourceRow](ctx, t.db,
		postgres.SELECT(
			table.Resources.ID.AS("resource_row.id"),
			table.Resources.OrganizationID.AS("resource_row.organization_id"),
			types.Slug.AS("resource_row.type"),
			table.Resources.ParentResourceID.AS("resource_row.parent_id"),
			parentTypes.Slug.AS("resource_row.parent_type"),
		).FROM(
			table.Resources.
				INNER_JOIN(types, types.ID.EQ(table.Resources.ResourceTypeID)).
				LEFT_JOIN(parents, parents.ID.EQ(table.Resources.ParentResourceID)).
				LEFT_JOIN(parentTypes, parentTypes.ID.EQ(parents.ResourceTypeID)),
		).WHERE(where).
			ORDER_BY(table.Resources.ID.ASC()),
	).Value()
}
