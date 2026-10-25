package repository

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"

	"anchor/internal/authz/model"
	dbmodel "anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
)

func (r *repositoryImpl) MarkResource(ctx context.Context, productID, organizationID, object string) error {
	stmt := table.AuthzResourceMarks.INSERT(
		table.AuthzResourceMarks.ProductID, table.AuthzResourceMarks.OrganizationID, table.AuthzResourceMarks.Object,
	).VALUES(productID, organizationID, object).
		ON_CONFLICT(table.AuthzResourceMarks.ProductID, table.AuthzResourceMarks.Object).
		DO_UPDATE(postgres.SET(
			table.AuthzResourceMarks.Revision.SET(
				postgres.IntExp(postgres.Raw("nextval('authz_sync_mark_revisions')")),
			),
			table.AuthzResourceMarks.MarkedAt.SET(postgres.TimestampzExp(postgres.NOW())),
		))
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func toResourceMark(row dbmodel.AuthzResourceMarks) ResourceMark {
	return ResourceMark{
		ProductID: row.ProductID, OrganizationID: row.OrganizationID, Object: row.Object,
		Revision: row.Revision, MarkedAt: row.MarkedAt,
	}
}

func (r *repositoryImpl) PendingResources(
	ctx context.Context, productID string, objects []string,
) ([]ResourceMark, error) {
	if len(objects) == 0 {
		return nil, nil
	}
	return transactor.QueryMapSlice(ctx, r.db,
		table.AuthzResourceMarks.SELECT(table.AuthzResourceMarks.AllColumns).
			WHERE(table.AuthzResourceMarks.ProductID.EQ(postgres.String(productID)).
				AND(table.AuthzResourceMarks.Object.IN(jetx.ToStringExpressions(objects)...))).
			ORDER_BY(table.AuthzResourceMarks.Object.ASC()),
		toResourceMark,
	).Value()
}

func (r *repositoryImpl) ClearResource(ctx context.Context, mark ResourceMark) error {
	return transactor.Exec(ctx, r.db, table.AuthzResourceMarks.DELETE().WHERE(
		table.AuthzResourceMarks.ProductID.EQ(postgres.String(mark.ProductID)).
			AND(table.AuthzResourceMarks.Object.EQ(postgres.String(mark.Object))).
			AND(table.AuthzResourceMarks.Revision.EQ(postgres.Int(mark.Revision))),
	)).Err()
}

func (r *repositoryImpl) OldestResourceMarksInternal(ctx context.Context, limit int64) ([]ResourceMark, error) {
	return transactor.QueryMapSlice(ctx, r.db,
		table.AuthzResourceMarks.SELECT(table.AuthzResourceMarks.AllColumns).
			ORDER_BY(table.AuthzResourceMarks.MarkedAt.ASC()).
			LIMIT(limit),
		toResourceMark,
	).Value()
}

// resourceRow is a registered resource with its type's slug and its parent's.
type resourceRow struct {
	ID               string  `alias:"resource_row.id"`
	OrganizationID   string  `alias:"resource_row.organization_id"`
	Type             string  `alias:"resource_row.type"`
	ParentResourceID *string `alias:"resource_row.parent_resource_id"`
	ParentType       *string `alias:"resource_row.parent_type"`
}

func (row resourceRow) toModel() model.Resource {
	return model.Resource{
		OrganizationID: row.OrganizationID,
		RowID:          row.ID,
		Type:           row.Type,
		ParentRowID:    functional.FromPtr(row.ParentResourceID).OrElse(""),
		ParentType:     functional.FromPtr(row.ParentType).OrElse(""),
	}
}

func (r *repositoryImpl) readResources(
	ctx context.Context, where postgres.BoolExpression,
) ([]model.Resource, error) {
	types := table.ProductResourceTypes.AS("resource_type")
	parents := table.Resources.AS("parent")
	parentTypes := table.ProductResourceTypes.AS("parent_type")
	return transactor.QueryMapSlice(ctx, r.db,
		postgres.SELECT(
			table.Resources.ID.AS("resource_row.id"),
			table.Resources.OrganizationID.AS("resource_row.organization_id"),
			types.Slug.AS("resource_row.type"),
			table.Resources.ParentResourceID.AS("resource_row.parent_resource_id"),
			parentTypes.Slug.AS("resource_row.parent_type"),
		).FROM(
			table.Resources.
				INNER_JOIN(types, types.ID.EQ(table.Resources.ResourceTypeID)).
				LEFT_JOIN(parents, parents.ID.EQ(table.Resources.ParentResourceID)).
				LEFT_JOIN(parentTypes, parentTypes.ID.EQ(parents.ResourceTypeID)),
		).WHERE(where).
			ORDER_BY(table.Resources.ID.ASC()),
		resourceRow.toModel,
	).Value()
}

func (r *repositoryImpl) ResourceStates(
	ctx context.Context, productID string, objects []string,
) ([]model.ResourceState, error) {
	states := make([]model.ResourceState, 0, len(objects))
	var rowIDs []string
	for _, object := range objects {
		states = append(states, model.ResourceState{Object: object})
		if _, rowID, ok := model.ParseResourceObject(object); ok {
			rowIDs = append(rowIDs, rowID)
		}
	}
	if len(rowIDs) == 0 {
		return states, nil
	}
	found, err := r.readResources(ctx, table.Resources.ProductID.EQ(postgres.String(productID)).
		AND(table.Resources.ID.IN(jetx.ToStringExpressions(rowIDs)...)))
	if err != nil {
		return nil, err
	}
	byObject := functional.Slice(found).ToMap(model.Resource.Object)
	assignments, err := r.resourceAssignments(ctx, table.Resources.ProductID.EQ(postgres.String(productID)).
		AND(table.ResourceRoleAssignments.ResourceID.IN(jetx.ToStringExpressions(rowIDs)...)))
	if err != nil {
		return nil, err
	}
	for i, state := range states {
		if row, ok := byObject[state.Object]; ok {
			states[i].Exists = true
			states[i].Resource = row
			states[i].Assignments = assignments[row.RowID]
		}
	}
	return states, nil
}

// assignmentRow is a role assignment on a resource, read for its tuples.
type assignmentRow struct {
	ResourceID    string `alias:"assignment_row.resource_id"`
	ProductUserID string `alias:"assignment_row.product_user_id"`
	RoleID        string `alias:"assignment_row.role_id"`
}

// resourceAssignments reads the role assignments on the resources the
// condition keeps, by resource row ID.
func (r *repositoryImpl) resourceAssignments(
	ctx context.Context, where postgres.BoolExpression,
) (map[string][]model.ResourceAssignment, error) {
	a := table.ResourceRoleAssignments
	rows, err := transactor.Query[[]assignmentRow](ctx, r.db,
		postgres.SELECT(
			a.ResourceID.AS("assignment_row.resource_id"),
			a.ProductUserID.AS("assignment_row.product_user_id"),
			a.RoleID.AS("assignment_row.role_id"),
		).FROM(a.INNER_JOIN(table.Resources, table.Resources.ID.EQ(a.ResourceID))).
			WHERE(where).
			ORDER_BY(a.ResourceID.ASC(), a.RoleID.ASC(), a.ProductUserID.ASC()),
	).Value()
	if err != nil {
		return nil, err
	}
	byResource := map[string][]model.ResourceAssignment{}
	for _, row := range rows {
		byResource[row.ResourceID] = append(byResource[row.ResourceID], model.ResourceAssignment{
			ProductUserID: row.ProductUserID, RoleRowID: row.RoleID,
		})
	}
	return byResource, nil
}

func (r *repositoryImpl) ProductResourceStates(ctx context.Context, productID string) ([]model.ResourceState, error) {
	found, err := r.readResources(ctx, table.Resources.ProductID.EQ(postgres.String(productID)))
	if err != nil {
		return nil, err
	}
	assignments, err := r.resourceAssignments(ctx, table.Resources.ProductID.EQ(postgres.String(productID)))
	if err != nil {
		return nil, err
	}
	return functional.Slice(found).Map(func(row model.Resource) model.ResourceState {
		return model.ResourceState{
			Object: row.Object(), Exists: true, Resource: row, Assignments: assignments[row.RowID],
		}
	}), nil
}
