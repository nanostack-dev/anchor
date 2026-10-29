package projection

import (
	"context"
	"slices"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/mirror"
	"anchor/internal/authz/model"
	dbmodel "anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/events"
)

// Assignments mirrors who holds which role in each Organization: one
// assignment per role held, every member holding it an assignee. A check on an
// Organization then walks its roles, a handful, not its members.
type Assignments struct{ tables Tables }

func NewAssignments(tables Tables) Assignments { return Assignments{tables: tables} }

func (Assignments) Name() string { return "organization_assignments" }

func (Assignments) Stale(event events.Event) ([]string, error) {
	switch {
	case isOneOf(event, events.MembershipCreated, events.MembershipUpdated, events.MembershipDeleted,
		events.OrganizationDeleted,
		// A deleted custom role takes its memberships with it, without events.
		events.OrganizationRoleDeleted):
		return keyOf(event, events.FieldOrganizationID)
	case isOneOf(event, events.ProductRoleDeleted, events.ProductUserDeleted):
		// Their memberships cascade away in any Organization, without events.
		return []string{mirror.EveryKey}, nil
	default:
		return nil, nil
	}
}

func (a Assignments) Keys(ctx context.Context, productID string) ([]string, error) {
	return a.tables.organizationIDs(ctx, productID)
}

func (a Assignments) Desired(ctx context.Context, productID, organizationID string) ([]fga.Tuple, error) {
	members, err := a.tables.members(ctx, productID, organizationID)
	if err != nil {
		return nil, err
	}
	return assignmentTuples(organizationID, members), nil
}

// Stored reads the Organization's assignment links, derives each one's role
// tuple from its ID, and reads its assignees.
func (Assignments) Stored(ctx context.Context, store fga.Reader, organizationID string) ([]fga.Tuple, error) {
	links, err := store.Read(ctx, fga.Filter{
		Object: model.Organization(organizationID), Relation: model.RelationAssignment,
	})
	if err != nil {
		return nil, err
	}
	stored := slices.Clone(links)
	for _, link := range links {
		if roleTuple, ok := assignmentRoleTuple(link.User).Get(); ok {
			stored = append(stored, roleTuple)
		}
		assignees, readErr := store.Read(ctx, fga.Filter{Object: link.User, Relation: model.RelationAssignee})
		if readErr != nil {
			return nil, readErr
		}
		stored = append(stored, assignees...)
	}
	return stored, nil
}

// member is a product user holding a role in an Organization.
type member struct {
	productUserID string
	roleRowID     string
}

func assignmentTuples(organizationID string, members []member) []fga.Tuple {
	var tuples []fga.Tuple
	linked := map[string]bool{}
	for _, m := range members {
		assignment := model.Assignment(organizationID, m.roleRowID)
		if !linked[m.roleRowID] {
			linked[m.roleRowID] = true
			tuples = append(
				tuples,
				fga.Tuple{
					Object:   model.Organization(organizationID),
					Relation: model.RelationAssignment,
					User:     assignment,
				},
				fga.Tuple{Object: assignment, Relation: model.RelationRole, User: model.Role(m.roleRowID)},
			)
		}
		tuples = append(tuples,
			fga.Tuple{Object: assignment, Relation: model.RelationAssignee, User: model.User(m.productUserID)})
	}
	return tuples
}

// assignmentRoleTuple is the role link an assignment's ID encodes, so a sync
// can delete it without reading it.
func assignmentRoleTuple(assignment string) functional.Option[fga.Tuple] {
	parts, ok := model.ParseAssignment(assignment)
	if !ok {
		return functional.None[fga.Tuple]()
	}
	return functional.Some(
		fga.Tuple{Object: assignment, Relation: model.RelationRole, User: model.Role(parts.RoleRowID)},
	)
}

// members lists the Organization's members that hold a role; a member without
// one holds nothing the store mirrors.
func (t Tables) members(ctx context.Context, productID, organizationID string) ([]member, error) {
	return transactor.QueryMapSlice(ctx, t.db,
		postgres.SELECT(table.OrganizationMemberships.ProductUserID, table.OrganizationMemberships.ProductRoleID).
			FROM(table.OrganizationMemberships.INNER_JOIN(table.Organizations,
				table.Organizations.ID.EQ(table.OrganizationMemberships.OrganizationID))).
			WHERE(table.Organizations.ProductID.EQ(postgres.String(productID)).
				AND(table.OrganizationMemberships.OrganizationID.EQ(postgres.String(organizationID))).
				AND(table.OrganizationMemberships.ProductRoleID.IS_NOT_NULL())).
			ORDER_BY(table.OrganizationMemberships.ProductUserID.ASC()),
		func(row dbmodel.OrganizationMemberships) member {
			return member{productUserID: row.ProductUserID, roleRowID: *row.ProductRoleID}
		},
	).Value()
}

func (t Tables) organizationIDs(ctx context.Context, productID string) ([]string, error) {
	return transactor.QueryMapSlice(ctx, t.db,
		table.Organizations.SELECT(table.Organizations.ID).
			WHERE(table.Organizations.ProductID.EQ(postgres.String(productID))).
			ORDER_BY(table.Organizations.ID.ASC()),
		func(row dbmodel.Organizations) string { return row.ID },
	).Value()
}
