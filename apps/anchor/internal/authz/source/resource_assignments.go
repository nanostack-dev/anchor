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

// ResourceAssignments syncs who holds which role on each registered
// resource: one assignment per role held there, linked from the resource's
// object, every member holding it an assignee. Keyed by the resource's
// object, like ResourceParents, so a write reads that one resource's
// assignments. The roles' grants are RoleGrants'.
type ResourceAssignments struct{ tables Tables }

func NewResourceAssignments(tables Tables) ResourceAssignments {
	return ResourceAssignments{tables: tables}
}

func (ResourceAssignments) Name() string { return "resource_assignments" }

func (ResourceAssignments) Stale(event events.Event) ([]string, error) {
	// Every write that removes assignments, a member's removal or a cascade
	// included, emits one deleted event each. A resource delete marks too.
	if !isOneOf(event, events.ResourceRoleAssignmentCreated, events.ResourceRoleAssignmentUpdated,
		events.ResourceRoleAssignmentDeleted, events.ResourceDeleted) {
		return nil, nil
	}
	return resourceKey(event)
}

func (r ResourceAssignments) Keys(ctx context.Context, productID string) ([]string, error) {
	held, err := r.tables.resourceAssignments(ctx, productID, nil)
	if err != nil {
		return nil, err
	}
	var keys []string
	seen := map[string]bool{}
	for _, a := range held {
		if object := model.ResourceObject(a.Type, a.ResourceID); !seen[object] {
			seen[object] = true
			keys = append(keys, object)
		}
	}
	return keys, nil
}

func (r ResourceAssignments) Desired(ctx context.Context, productID, object string) ([]fga.Tuple, error) {
	slug, rowID, ok := model.ParseResourceObject(object)
	if !ok {
		return nil, nil
	}
	held, err := r.tables.resourceAssignments(ctx, productID, table.ResourceRoleAssignments.ResourceID.EQ(
		postgres.String(rowID)))
	if err != nil {
		return nil, err
	}
	members := functional.Slice(held).FilterMap(
		func(a heldAssignment) bool { return a.Type == slug },
		func(a heldAssignment) member { return member{productUserID: a.ProductUserID, roleRowID: a.RoleID} },
	)
	return linkedAssignments(object, rowID, members), nil
}

func (ResourceAssignments) Stored(ctx context.Context, store fga.Reader, object string) ([]fga.Tuple, error) {
	return storedAssignments(ctx, store, object)
}

// resourceKey is the object a resource event names.
func resourceKey(event events.Event) ([]string, error) {
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

// heldAssignment is a member holding a role on a resource of a type.
type heldAssignment struct {
	ResourceID    string `alias:"held_assignment.resource_id"`
	Type          string `alias:"held_assignment.type"`
	ProductUserID string `alias:"held_assignment.product_user_id"`
	RoleID        string `alias:"held_assignment.role_id"`
}

// resourceAssignments lists the Product's role assignments on resources, or
// those the condition keeps, with each resource's type slug.
func (t Tables) resourceAssignments(
	ctx context.Context, productID string, where postgres.BoolExpression,
) ([]heldAssignment, error) {
	a := table.ResourceRoleAssignments
	types := table.ProductResourceTypes.AS("resource_type")
	condition := table.Resources.ProductID.EQ(postgres.String(productID))
	if where != nil {
		condition = condition.AND(where)
	}
	return transactor.Query[[]heldAssignment](ctx, t.db,
		postgres.SELECT(
			a.ResourceID.AS("held_assignment.resource_id"),
			types.Slug.AS("held_assignment.type"),
			a.ProductUserID.AS("held_assignment.product_user_id"),
			a.RoleID.AS("held_assignment.role_id"),
		).FROM(
			a.INNER_JOIN(table.Resources, table.Resources.ID.EQ(a.ResourceID)).
				INNER_JOIN(types, types.ID.EQ(table.Resources.ResourceTypeID)),
		).WHERE(condition).
			ORDER_BY(a.ResourceID.ASC(), a.ProductUserID.ASC(), a.RoleID.ASC()),
	).Value()
}
