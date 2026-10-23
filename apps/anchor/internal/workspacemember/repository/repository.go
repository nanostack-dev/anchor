package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/workspace"
)

var _ Repository = (*repositoryImpl)(nil)

// Repository persists the roles organization members hold in workspaces. Every
// method is scoped by product, organization and workspace, so a caller cannot
// reach another Product's membership by guessing a KSUID.
type Repository interface {
	// LockOrganizationMember takes a row lock on the organization membership of
	// the product user that lasts until the surrounding transaction ends, and
	// reports whether the membership exists. A workspace role is written only
	// under this lock, so a concurrent removal from the organization either
	// waits for the write and then deletes its row, or runs first and the write
	// finds no member. Call it inside a transaction.
	LockOrganizationMember(ctx context.Context, productID, organizationID, productUserID string) (bool, error)
	Find(
		ctx context.Context, productID, organizationID, workspaceID, productUserID string,
	) (functional.Option[workspace.Membership], error)
	// Upsert gives the product user the role in the workspace, or replaces the
	// role they hold there, and returns the membership as stored.
	Upsert(
		ctx context.Context, productID, organizationID, workspaceID, productUserID, roleID string,
	) (workspace.Membership, error)
	// Delete removes the role the product user holds in the workspace and
	// returns it, or returns None when they held none.
	Delete(
		ctx context.Context, productID, organizationID, workspaceID, productUserID string,
	) (functional.Option[workspace.RemovedMembership], error)
	// DeleteByOrganizationAndProductUser removes every role the product user
	// holds in the workspaces of the organization and returns them.
	DeleteByOrganizationAndProductUser(
		ctx context.Context, productID, organizationID, productUserID string,
	) ([]workspace.RemovedMembership, error)
	Search(
		ctx context.Context,
		productID, organizationID, workspaceID string,
		req search.Request[workspace.SearchMembersFilter, workspace.SortFieldMember],
	) (search.Result[workspace.Membership], error)
}

type repositoryImpl struct {
	db     *sql.DB
	logger zerolog.Logger
}

func NewRepository(db *sql.DB, logger zerolog.Logger) Repository {
	return &repositoryImpl{
		db:     db,
		logger: logger.With().Str("component", "workspace_member_repository").Logger(),
	}
}

// memberRow scans one workspace role with the member and the role it names. The
// search pages one SQL row per item, so the row carries no permission slice.
type memberRow struct {
	model.WorkspaceMemberships
	Workspace   model.Workspaces   `alias:"workspaces"`
	ProductUser model.ProductUsers `alias:"product_users"`
	Role        model.ProductRoles `alias:"product_roles"`
}

func toDomain(row memberRow) workspace.Membership {
	return workspace.Membership{
		WorkspaceID:    row.WorkspaceMemberships.WorkspaceID,
		OrganizationID: row.Workspace.OrganizationID,
		ProductUserID:  row.WorkspaceMemberships.ProductUserID,
		Email:          row.ProductUser.Email,
		Name:           row.ProductUser.Name,
		RoleID:         row.Role.ID,
		RoleName:       row.Role.Name,
		AssignedAt:     row.WorkspaceMemberships.CreatedAt,
		UpdatedAt:      row.WorkspaceMemberships.UpdatedAt,
	}
}

func toRemoved(entity model.WorkspaceMemberships) workspace.RemovedMembership {
	return workspace.RemovedMembership{
		WorkspaceID:   entity.WorkspaceID,
		ProductUserID: entity.ProductUserID,
		RoleID:        entity.ProductRoleID,
	}
}

// memberships joins a workspace role to everything a membership reads and to
// the organization that scopes it to a product.
func memberships() postgres.ReadableTable {
	return table.WorkspaceMemberships.
		INNER_JOIN(table.Workspaces, table.WorkspaceMemberships.WorkspaceID.EQ(table.Workspaces.ID)).
		INNER_JOIN(table.Organizations, table.Workspaces.OrganizationID.EQ(table.Organizations.ID)).
		INNER_JOIN(table.ProductUsers, table.WorkspaceMemberships.ProductUserID.EQ(table.ProductUsers.ID)).
		INNER_JOIN(table.ProductRoles, table.WorkspaceMemberships.ProductRoleID.EQ(table.ProductRoles.ID))
}

func memberColumns() postgres.ProjectionList {
	return postgres.ProjectionList{
		table.WorkspaceMemberships.AllColumns,
		table.Workspaces.AllColumns,
		table.ProductUsers.AllColumns,
		table.ProductRoles.AllColumns,
	}
}

func organizationScope(productID, organizationID string) postgres.BoolExpression {
	return table.Organizations.ID.EQ(postgres.String(organizationID)).
		AND(table.Organizations.ProductID.EQ(postgres.String(productID)))
}

func workspaceScope(productID, organizationID, workspaceID string) postgres.BoolExpression {
	return organizationScope(productID, organizationID).
		AND(table.Workspaces.ID.EQ(postgres.String(workspaceID)))
}

func memberScope(productID, organizationID, workspaceID, productUserID string) postgres.BoolExpression {
	return workspaceScope(productID, organizationID, workspaceID).
		AND(table.WorkspaceMemberships.ProductUserID.EQ(postgres.String(productUserID)))
}

// workspaceIDsInOrganization selects the workspaces a delete may touch, as a
// delete cannot join the organization that scopes it to a product.
func workspaceIDsInOrganization(productID, organizationID string) postgres.BoolExpression {
	return table.WorkspaceMemberships.WorkspaceID.IN(
		postgres.SELECT(table.Workspaces.ID).
			FROM(table.Workspaces.INNER_JOIN(
				table.Organizations, table.Workspaces.OrganizationID.EQ(table.Organizations.ID),
			)).
			WHERE(organizationScope(productID, organizationID)),
	)
}

func (r *repositoryImpl) LockOrganizationMember(
	ctx context.Context, productID, organizationID, productUserID string,
) (bool, error) {
	stmt := table.OrganizationMemberships.SELECT(table.OrganizationMemberships.AllColumns).
		FROM(table.OrganizationMemberships.INNER_JOIN(
			table.Organizations, table.OrganizationMemberships.OrganizationID.EQ(table.Organizations.ID),
		)).
		WHERE(
			organizationScope(productID, organizationID).
				AND(table.OrganizationMemberships.ProductUserID.EQ(postgres.String(productUserID))),
		).
		FOR(postgres.NO_KEY_UPDATE().OF(table.OrganizationMemberships))
	found, err := transactor.QueryOptional[model.OrganizationMemberships](ctx, r.db, stmt)
	if err != nil {
		return false, err
	}
	return found.IsPresent(), nil
}

func (r *repositoryImpl) Find(
	ctx context.Context, productID, organizationID, workspaceID, productUserID string,
) (functional.Option[workspace.Membership], error) {
	stmt := postgres.SELECT(memberColumns()).
		FROM(memberships()).
		WHERE(memberScope(productID, organizationID, workspaceID, productUserID)).
		LIMIT(1)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, toDomain)
}

func (r *repositoryImpl) Upsert(
	ctx context.Context, productID, organizationID, workspaceID, productUserID, roleID string,
) (workspace.Membership, error) {
	stmt := table.WorkspaceMemberships.
		INSERT(
			table.WorkspaceMemberships.WorkspaceID,
			table.WorkspaceMemberships.ProductUserID,
			table.WorkspaceMemberships.ProductRoleID,
		).
		QUERY(
			postgres.SELECT(
				table.Workspaces.ID,
				postgres.String(productUserID),
				postgres.String(roleID),
			).
				FROM(table.Workspaces.INNER_JOIN(
					table.Organizations, table.Workspaces.OrganizationID.EQ(table.Organizations.ID),
				)).
				WHERE(workspaceScope(productID, organizationID, workspaceID)),
		).
		ON_CONFLICT(table.WorkspaceMemberships.WorkspaceID, table.WorkspaceMemberships.ProductUserID).
		DO_UPDATE(postgres.SET(
			table.WorkspaceMemberships.ProductRoleID.SET(table.WorkspaceMemberships.EXCLUDED.ProductRoleID),
		))
	if err := transactor.Exec(ctx, r.db, stmt).Err(); err != nil {
		return workspace.Membership{}, err
	}

	found, err := r.Find(ctx, productID, organizationID, workspaceID, productUserID)
	if err != nil {
		return workspace.Membership{}, err
	}
	return found.ToResult(workspace.ErrMembershipNotFound).Value()
}

func (r *repositoryImpl) Delete(
	ctx context.Context, productID, organizationID, workspaceID, productUserID string,
) (functional.Option[workspace.RemovedMembership], error) {
	stmt := table.WorkspaceMemberships.DELETE().
		WHERE(
			workspaceIDsInOrganization(productID, organizationID).
				AND(table.WorkspaceMemberships.WorkspaceID.EQ(postgres.String(workspaceID))).
				AND(table.WorkspaceMemberships.ProductUserID.EQ(postgres.String(productUserID))),
		).
		RETURNING(table.WorkspaceMemberships.AllColumns)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, toRemoved)
}

func (r *repositoryImpl) DeleteByOrganizationAndProductUser(
	ctx context.Context, productID, organizationID, productUserID string,
) ([]workspace.RemovedMembership, error) {
	stmt := table.WorkspaceMemberships.DELETE().
		WHERE(
			workspaceIDsInOrganization(productID, organizationID).
				AND(table.WorkspaceMemberships.ProductUserID.EQ(postgres.String(productUserID))),
		).
		RETURNING(table.WorkspaceMemberships.AllColumns)
	return transactor.QueryMapSlice(ctx, r.db, stmt, toRemoved).Value()
}

func (r *repositoryImpl) Search(
	ctx context.Context,
	productID, organizationID, workspaceID string,
	req search.Request[workspace.SearchMembersFilter, workspace.SortFieldMember],
) (search.Result[workspace.Membership], error) {
	order, err := transactor.StrictKeys(
		req.Sort,
		map[workspace.SortFieldMember]transactor.Key[memberRow]{
			workspace.SortFieldMemberAssignedAt: transactor.TimeKey(
				table.WorkspaceMemberships.CreatedAt,
				func(row memberRow) time.Time { return row.WorkspaceMemberships.CreatedAt },
			),
			workspace.SortFieldMemberEmail: transactor.StringKey(
				table.ProductUsers.Email,
				func(row memberRow) string { return row.ProductUser.Email },
			),
		},
		transactor.StringKey(
			table.WorkspaceMemberships.ProductUserID,
			func(row memberRow) string { return row.WorkspaceMemberships.ProductUserID },
		),
	)
	if err != nil {
		return search.Result[workspace.Membership]{}, err
	}

	return transactor.Page(
		r.db,
		toDomain,
		table.WorkspaceMemberships.AllColumns,
		table.Workspaces.AllColumns,
		table.ProductUsers.AllColumns,
		table.ProductRoles.AllColumns,
	).
		From(memberships()).
		Where(searchWhere(productID, organizationID, workspaceID, req)).
		Keyset(order).
		Run(ctx, req.Pagination).
		Value()
}

func searchWhere(
	productID, organizationID, workspaceID string,
	req search.Request[workspace.SearchMembersFilter, workspace.SortFieldMember],
) postgres.BoolExpression {
	where := workspaceScope(productID, organizationID, workspaceID)

	if req.Filter != nil {
		if ids := jetx.BuildIDFilter(table.WorkspaceMemberships.ProductUserID, req.Filter.ProductUserIDs); ids != nil {
			where = where.AND(ids)
		}
		if roleIDs := jetx.BuildIDFilter(table.WorkspaceMemberships.ProductRoleID, req.Filter.RoleIDs); roleIDs != nil {
			where = where.AND(roleIDs)
		}
	}

	if req.FullTextSearch != nil {
		columns := []postgres.ColumnString{table.ProductUsers.Email, table.ProductUsers.Name}
		if text := jetx.BuildSubstringFilter(columns, *req.FullTextSearch); text != nil {
			where = where.AND(text)
		}
	}

	return where
}
