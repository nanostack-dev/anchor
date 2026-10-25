// Package repository stores the roles members hold on registered resources.
package repository

import (
	"context"
	"database/sql"
	"slices"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/pgkit/pglock"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/roleassignment"
)

type Repository interface {
	// LockMember holds one member's assignments on one resource for the
	// transaction, so concurrent writes for them run one at a time and each
	// replaces what the previous one left.
	LockMember(ctx context.Context, resourceRowID, productUserID string) error
	// ShareMembership holds the membership under a key share lock, so the
	// member can't leave the Organization before the write commits, and
	// reports whether it exists.
	ShareMembership(ctx context.Context, organizationID, productUserID string) (bool, error)
	// LockMembership holds the membership row for the transaction, so no
	// assignment is added while the member leaves.
	LockMembership(ctx context.Context, organizationID, productUserID string) error
	// Find reads the roles a member holds on a resource, in role order.
	Find(ctx context.Context, resourceRowID, productUserID string) (functional.Option[roleassignment.Member], error)
	Insert(ctx context.Context, organizationID, resourceRowID, productUserID string, roleRowIDs []string) error
	// Reassign moves one assignment to another role and keeps when it was made.
	Reassign(ctx context.Context, resourceRowID, productUserID, fromRoleRowID, toRoleRowID string) error
	Delete(ctx context.Context, resourceRowID, productUserID string, roleRowIDs []string) error
	// HeldByMember lists every assignment a member holds in an Organization.
	HeldByMember(ctx context.Context, organizationID, productUserID string) ([]roleassignment.Held, error)
	// HeldOnResources lists every assignment held on the resources.
	HeldOnResources(ctx context.Context, organizationID string, resourceRowIDs []string) ([]roleassignment.Held, error)
	CountOnResource(ctx context.Context, resourceRowID string) (int, error)
	Search(
		ctx context.Context, resourceRowID string,
		request search.Request[roleassignment.Filter, roleassignment.SortField],
	) (search.Result[roleassignment.Member], error)
}

type repositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repositoryImpl{db: db}
}

func (r *repositoryImpl) LockMember(ctx context.Context, resourceRowID, productUserID string) error {
	_, err := transactor.Executor(ctx, r.db).ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`,
		pglock.KeyHash("resource-member:"+resourceRowID+":"+productUserID))
	return err
}

func membership(organizationID, productUserID string) postgres.BoolExpression {
	return table.OrganizationMemberships.OrganizationID.EQ(postgres.String(organizationID)).
		AND(table.OrganizationMemberships.ProductUserID.EQ(postgres.String(productUserID)))
}

func (r *repositoryImpl) ShareMembership(ctx context.Context, organizationID, productUserID string) (bool, error) {
	found, err := transactor.QueryOptional[model.OrganizationMemberships](ctx, r.db,
		table.OrganizationMemberships.SELECT(table.OrganizationMemberships.ProductUserID).
			WHERE(membership(organizationID, productUserID)).
			FOR(postgres.KEY_SHARE()),
	)
	return found.IsPresent(), err
}

func (r *repositoryImpl) LockMembership(ctx context.Context, organizationID, productUserID string) error {
	return transactor.Exec(ctx, r.db,
		table.OrganizationMemberships.SELECT(table.OrganizationMemberships.ProductUserID).
			WHERE(membership(organizationID, productUserID)).
			FOR(postgres.UPDATE()),
	).Err()
}

// assignmentRow is an assignment with its role's public ID and name.
type assignmentRow struct {
	model.ResourceRoleAssignments

	RolePublicID string `alias:"role.public_id"`
	RoleName     string `alias:"role.name"`
}

func assignmentColumns() []postgres.Projection {
	roles := table.ProductRoles.AS("role")
	return []postgres.Projection{
		table.ResourceRoleAssignments.AllColumns,
		postgres.COALESCE(roles.PublicID, roles.ID).AS("role.public_id"),
		roles.Name.AS("role.name"),
	}
}

func assignmentJoins() postgres.ReadableTable {
	roles := table.ProductRoles.AS("role")
	return table.ResourceRoleAssignments.INNER_JOIN(roles, roles.ID.EQ(table.ResourceRoleAssignments.RoleID))
}

func onMember(resourceRowID, productUserID string) postgres.BoolExpression {
	return table.ResourceRoleAssignments.ResourceID.EQ(postgres.String(resourceRowID)).
		AND(table.ResourceRoleAssignments.ProductUserID.EQ(postgres.String(productUserID)))
}

// members groups assignment rows by member, keeping the members' order.
func members(rows []assignmentRow) []roleassignment.Member {
	var grouped []roleassignment.Member
	index := map[string]int{}
	for _, row := range rows {
		at, seen := index[row.ProductUserID]
		if !seen {
			at = len(grouped)
			index[row.ProductUserID] = at
			grouped = append(grouped, roleassignment.Member{
				ProductUserID: row.ProductUserID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			})
		}
		member := &grouped[at]
		member.Roles = append(member.Roles, roleassignment.Role{
			RowID: row.RoleID, ID: row.RolePublicID, Name: row.RoleName,
		})
		if row.CreatedAt.Before(member.CreatedAt) {
			member.CreatedAt = row.CreatedAt
		}
		if row.UpdatedAt.After(member.UpdatedAt) {
			member.UpdatedAt = row.UpdatedAt
		}
	}
	return grouped
}

func (r *repositoryImpl) Find(
	ctx context.Context, resourceRowID, productUserID string,
) (functional.Option[roleassignment.Member], error) {
	columns := assignmentColumns()
	rows, err := transactor.Query[[]assignmentRow](ctx, r.db,
		postgres.SELECT(columns[0], columns[1:]...).
			FROM(assignmentJoins()).
			WHERE(onMember(resourceRowID, productUserID)).
			ORDER_BY(table.ResourceRoleAssignments.RoleID.ASC()),
	).Value()
	if err != nil || len(rows) == 0 {
		return functional.None[roleassignment.Member](), err
	}
	return functional.Some(members(rows)[0]), nil
}

func (r *repositoryImpl) Insert(
	ctx context.Context, organizationID, resourceRowID, productUserID string, roleRowIDs []string,
) error {
	if len(roleRowIDs) == 0 {
		return nil
	}
	rows := functional.Slice(roleRowIDs).Map(func(roleRowID string) model.ResourceRoleAssignments {
		return model.ResourceRoleAssignments{
			ResourceID: resourceRowID, ProductUserID: productUserID, OrganizationID: organizationID, RoleID: roleRowID,
		}
	})
	return transactor.Exec(ctx, r.db, table.ResourceRoleAssignments.INSERT(
		table.ResourceRoleAssignments.ResourceID, table.ResourceRoleAssignments.ProductUserID,
		table.ResourceRoleAssignments.OrganizationID, table.ResourceRoleAssignments.RoleID,
	).MODELS(rows)).Err()
}

func (r *repositoryImpl) Reassign(
	ctx context.Context, resourceRowID, productUserID, fromRoleRowID, toRoleRowID string,
) error {
	return transactor.Exec(ctx, r.db,
		table.ResourceRoleAssignments.UPDATE(table.ResourceRoleAssignments.RoleID).
			SET(postgres.String(toRoleRowID)).
			WHERE(onMember(resourceRowID, productUserID).
				AND(table.ResourceRoleAssignments.RoleID.EQ(postgres.String(fromRoleRowID)))),
	).Err()
}

func (r *repositoryImpl) Delete(ctx context.Context, resourceRowID, productUserID string, roleRowIDs []string) error {
	if len(roleRowIDs) == 0 {
		return nil
	}
	return transactor.Exec(ctx, r.db, table.ResourceRoleAssignments.DELETE().WHERE(
		onMember(resourceRowID, productUserID).
			AND(table.ResourceRoleAssignments.RoleID.IN(jetx.ToStringExpressions(roleRowIDs)...)),
	)).Err()
}

// heldRow is an assignment with the product's names of its resource and the
// public ID of its role.
type heldRow struct {
	ResourceRowID      string `alias:"held.resource_row_id"`
	ResourceType       string `alias:"held.resource_type"`
	ResourceExternalID string `alias:"held.resource_external_id"`
	OrganizationID     string `alias:"held.organization_id"`
	ProductUserID      string `alias:"held.product_user_id"`
	RoleRowID          string `alias:"held.role_row_id"`
	RoleID             string `alias:"held.role_id"`
}

func (r *repositoryImpl) held(ctx context.Context, where postgres.BoolExpression) ([]roleassignment.Held, error) {
	resources := table.Resources.AS("resource")
	types := table.ProductResourceTypes.AS("resource_type")
	roles := table.ProductRoles.AS("role")
	a := table.ResourceRoleAssignments
	return transactor.QueryMapSlice(ctx, r.db,
		postgres.SELECT(
			a.ResourceID.AS("held.resource_row_id"),
			types.Slug.AS("held.resource_type"),
			resources.ExternalID.AS("held.resource_external_id"),
			a.OrganizationID.AS("held.organization_id"),
			a.ProductUserID.AS("held.product_user_id"),
			a.RoleID.AS("held.role_row_id"),
			postgres.COALESCE(roles.PublicID, roles.ID).AS("held.role_id"),
		).FROM(
			a.INNER_JOIN(resources, resources.ID.EQ(a.ResourceID)).
				INNER_JOIN(types, types.ID.EQ(resources.ResourceTypeID)).
				INNER_JOIN(roles, roles.ID.EQ(a.RoleID)),
		).WHERE(where).ORDER_BY(a.ResourceID.ASC(), a.ProductUserID.ASC(), a.RoleID.ASC()),
		func(row heldRow) roleassignment.Held { return roleassignment.Held(row) },
	).Value()
}

func (r *repositoryImpl) HeldByMember(
	ctx context.Context, organizationID, productUserID string,
) ([]roleassignment.Held, error) {
	return r.held(ctx, table.ResourceRoleAssignments.OrganizationID.EQ(postgres.String(organizationID)).
		AND(table.ResourceRoleAssignments.ProductUserID.EQ(postgres.String(productUserID))))
}

func (r *repositoryImpl) HeldOnResources(
	ctx context.Context, organizationID string, resourceRowIDs []string,
) ([]roleassignment.Held, error) {
	if len(resourceRowIDs) == 0 {
		return nil, nil
	}
	return r.held(ctx, table.ResourceRoleAssignments.OrganizationID.EQ(postgres.String(organizationID)).
		AND(table.ResourceRoleAssignments.ResourceID.IN(jetx.ToStringExpressions(resourceRowIDs)...)))
}

func (r *repositoryImpl) CountOnResource(ctx context.Context, resourceRowID string) (int, error) {
	count, err := transactor.QueryCount(ctx, r.db,
		table.ResourceRoleAssignments.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.ResourceRoleAssignments.ResourceID.EQ(postgres.String(resourceRowID)),
		),
	).Value()
	return int(count), err
}

// Search pages the resource's members. Each member is paged by one of their
// assignments, the one with the lowest role row, so a member holding several
// roles still shows once; the page's members then read all their roles.
func (r *repositoryImpl) Search(
	ctx context.Context, resourceRowID string,
	request search.Request[roleassignment.Filter, roleassignment.SortField],
) (search.Result[roleassignment.Member], error) {
	a := table.ResourceRoleAssignments
	order, err := transactor.StrictKeys(
		request.Sort,
		map[roleassignment.SortField]transactor.Key[model.ResourceRoleAssignments]{
			roleassignment.SortFieldCreatedAt: transactor.TimeKey(
				a.CreatedAt, func(row model.ResourceRoleAssignments) time.Time { return row.CreatedAt },
			),
			roleassignment.SortFieldProductUserID: transactor.StringKey(
				a.ProductUserID, func(row model.ResourceRoleAssignments) string { return row.ProductUserID },
			),
		},
		transactor.StringKey(
			a.ProductUserID,
			func(row model.ResourceRoleAssignments) string { return row.ProductUserID },
		),
	)
	if err != nil {
		return search.Result[roleassignment.Member]{}, err
	}

	others := a.AS("other")
	where := a.ResourceID.EQ(postgres.String(resourceRowID)).
		AND(postgres.NOT(postgres.EXISTS(
			postgres.SELECT(postgres.Int(1)).FROM(others).WHERE(
				others.ResourceID.EQ(a.ResourceID).
					AND(others.ProductUserID.EQ(a.ProductUserID)).
					AND(others.RoleID.LT(a.RoleID)),
			),
		)))
	if request.Filter != nil {
		where = where.AND(filterCondition(*request.Filter))
	}
	page, err := transactor.Page(
		r.db, func(row model.ResourceRoleAssignments) model.ResourceRoleAssignments { return row }, a.AllColumns,
	).
		From(a).
		Where(where).
		Keyset(order).
		Run(ctx, request.Pagination).
		Value()
	if err != nil {
		return search.Result[roleassignment.Member]{}, err
	}
	users := functional.Slice(page.Items).
		Map(func(row model.ResourceRoleAssignments) string { return row.ProductUserID })
	items, err := r.membersOf(ctx, resourceRowID, users)
	if err != nil {
		return search.Result[roleassignment.Member]{}, err
	}
	return search.Result[roleassignment.Member]{
		Items: items, Total: page.Total, Count: len(items), NextCursor: page.NextCursor,
	}, nil
}

// membersOf reads every role of the members, in the order the page named
// them.
func (r *repositoryImpl) membersOf(
	ctx context.Context, resourceRowID string, productUserIDs []string,
) ([]roleassignment.Member, error) {
	if len(productUserIDs) == 0 {
		return []roleassignment.Member{}, nil
	}
	columns := assignmentColumns()
	rows, err := transactor.Query[[]assignmentRow](ctx, r.db,
		postgres.SELECT(columns[0], columns[1:]...).
			FROM(assignmentJoins()).
			WHERE(table.ResourceRoleAssignments.ResourceID.EQ(postgres.String(resourceRowID)).
				AND(table.ResourceRoleAssignments.ProductUserID.IN(jetx.ToStringExpressions(productUserIDs)...))).
			ORDER_BY(table.ResourceRoleAssignments.RoleID.ASC()),
	).Value()
	if err != nil {
		return nil, err
	}
	grouped := members(rows)
	slices.SortStableFunc(grouped, func(x, y roleassignment.Member) int {
		return slices.Index(productUserIDs, x.ProductUserID) - slices.Index(productUserIDs, y.ProductUserID)
	})
	return grouped, nil
}

// filterCondition matches a role by its public ID, so a template's ID names
// the Organization's copy, or by a copy's own row ID.
func filterCondition(filter roleassignment.Filter) postgres.BoolExpression {
	a := table.ResourceRoleAssignments
	condition := postgres.Bool(true)
	if len(filter.ProductUserIDs) > 0 {
		condition = condition.AND(a.ProductUserID.IN(jetx.ToStringExpressions(filter.ProductUserIDs)...))
	}
	if len(filter.RoleIDs) > 0 {
		held := a.AS("held")
		ids := jetx.ToStringExpressions(filter.RoleIDs)
		condition = condition.AND(postgres.EXISTS(
			postgres.SELECT(postgres.Int(1)).
				FROM(held.INNER_JOIN(table.ProductRoles, table.ProductRoles.ID.EQ(held.RoleID))).
				WHERE(held.ResourceID.EQ(a.ResourceID).
					AND(held.ProductUserID.EQ(a.ProductUserID)).
					AND(table.ProductRoles.PublicID.IN(ids...).OR(table.ProductRoles.ID.IN(ids...)))),
		))
	}
	return condition
}
