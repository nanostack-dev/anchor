package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/workspace"
	"anchor/internal/mapper"
)

var _ WorkspaceRepository = (*workspaceRepositoryImpl)(nil)

func workspacesUpdatableColumns() postgres.ColumnList {
	return table.Workspaces.AllColumns.Except(
		table.Workspaces.CreatedAt,
		table.Workspaces.UpdatedAt,
	)
}

type WorkspaceRepository interface {
	ListByOrganizations(
		ctx context.Context,
		productID string,
		organizationIDs []string,
	) (map[string][]workspace.Workspace, error)
	FindByID(
		ctx context.Context,
		productID string,
		organizationID string,
		workspaceID string,
	) (functional.Option[workspace.Workspace], error)
	FindByOrganizationIDAndName(
		ctx context.Context,
		productID string,
		organizationID string,
		name string,
	) (functional.Option[workspace.Workspace], error)
	Create(
		ctx context.Context,
		workspace workspace.Workspace,
	) (workspace.Workspace, error)
	Update(
		ctx context.Context,
		productID string,
		organizationID string,
		workspace workspace.Workspace,
	) (workspace.Workspace, error)
	DeleteByID(
		ctx context.Context,
		productID string,
		organizationID string,
		workspaceID string,
	) error
	SearchByOrganizationID(
		ctx context.Context,
		productID string,
		organizationID string,
		input search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace],
	) (search.Result[workspace.Workspace], error)
}

// ListByOrganizations reads linked workspaces for a whole organization page in one statement.
func (r *workspaceRepositoryImpl) ListByOrganizations(
	ctx context.Context, productID string, organizationIDs []string,
) (map[string][]workspace.Workspace, error) {
	ids := functional.Slice(organizationIDs).Map(func(id string) postgres.Expression {
		return postgres.String(id)
	}).ToSlice()
	stmt := table.Workspaces.SELECT(table.Workspaces.AllColumns).FROM(r.joinOrganizations()).
		WHERE(table.Organizations.ProductID.EQ(postgres.String(productID)).
			AND(table.Workspaces.OrganizationID.IN(ids...))).
		ORDER_BY(table.Workspaces.CreatedAt.ASC(), table.Workspaces.ID.ASC())
	items, err := transactor.QueryMapSlice(ctx, r.db, stmt, r.workspaceMapper.ToDomain).Value()
	if err != nil {
		return nil, err
	}
	result := make(map[string][]workspace.Workspace)
	for _, item := range items {
		result[item.OrganizationID] = append(result[item.OrganizationID], item)
	}
	return result, nil
}

type workspaceRepositoryImpl struct {
	db              *sql.DB
	workspaceMapper *mapper.WorkspaceMapper
	logger          zerolog.Logger
}

func NewWorkspaceRepository(
	db *sql.DB,
	workspaceMapper *mapper.WorkspaceMapper,
	logger zerolog.Logger,
) WorkspaceRepository {
	return &workspaceRepositoryImpl{
		db:              db,
		workspaceMapper: workspaceMapper,
		logger: logger.With().Str(
			"component", "workspace_repository",
		).Logger(),
	}
}

func (r *workspaceRepositoryImpl) FindByID(
	ctx context.Context,
	productID string,
	organizationID string,
	workspaceID string,
) (functional.Option[workspace.Workspace], error) {
	stmt := table.Workspaces.SELECT(
		table.Workspaces.AllColumns,
	).FROM(
		r.joinOrganizations(),
	).WHERE(
		r.scopedWhere(productID, organizationID).AND(
			table.Workspaces.ID.EQ(postgres.String(workspaceID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(
		ctx,
		r.db,
		stmt,
		r.workspaceMapper.ToDomain,
	)
}

func (r *workspaceRepositoryImpl) FindByOrganizationIDAndName(
	ctx context.Context,
	productID string,
	organizationID string,
	name string,
) (functional.Option[workspace.Workspace], error) {
	stmt := table.Workspaces.SELECT(
		table.Workspaces.AllColumns,
	).FROM(
		r.joinOrganizations(),
	).WHERE(
		r.scopedWhere(productID, organizationID).AND(
			table.Workspaces.Name.EQ(postgres.String(name)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(
		ctx,
		r.db,
		stmt,
		r.workspaceMapper.ToDomain,
	)
}

const workspaceNameUniqueConstraint = "workspaces_organization_id_name_key"

// ErrWorkspaceNameExists is returned by Create and Update when the organization
// already has a workspace with that name. Create serializes on the organization
// row, but a rename does not, so the constraint is the final arbiter.
var ErrWorkspaceNameExists = errors.New("workspace name already exists in organization")

func (r *workspaceRepositoryImpl) Create(
	ctx context.Context,
	newWorkspace workspace.Workspace,
) (workspace.Workspace, error) {
	entity := r.workspaceMapper.ToEntity(newWorkspace)

	stmt := table.Workspaces.INSERT(
		workspacesUpdatableColumns(),
	).MODEL(entity).RETURNING(table.Workspaces.AllColumns)

	created, err := transactor.QueryMap(
		ctx,
		r.db,
		stmt,
		r.workspaceMapper.ToDomain,
	).OnUnique(ErrWorkspaceNameExists, workspaceNameUniqueConstraint).Value()
	if err != nil {
		return workspace.Workspace{}, err
	}

	return created, nil
}

func (r *workspaceRepositoryImpl) Update(
	ctx context.Context,
	productID string,
	organizationID string,
	currentWorkspace workspace.Workspace,
) (workspace.Workspace, error) {
	currentWorkspace.UpdatedAt = time.Now()
	entityToUpdate := r.workspaceMapper.ToEntity(currentWorkspace)

	updateStmt := table.Workspaces.UPDATE(
		table.Workspaces.AllColumns.Except(
			table.Workspaces.ID,
			table.Workspaces.OrganizationID,
			table.Workspaces.CreatedAt,
		),
	).MODEL(
		entityToUpdate,
	).FROM(
		table.Organizations,
	).WHERE(
		table.Workspaces.ID.EQ(postgres.String(currentWorkspace.ID)).AND(
			r.mutationScopedWhere(productID, organizationID),
		),
	)

	if err := transactor.Exec(ctx, r.db, updateStmt).
		OnUnique(ErrWorkspaceNameExists, workspaceNameUniqueConstraint).
		Err(); err != nil {
		return workspace.Workspace{}, err
	}

	updated, err := r.FindByID(ctx, productID, organizationID, currentWorkspace.ID)
	if err != nil {
		return workspace.Workspace{}, err
	}
	if updated.IsAbsent() {
		return workspace.Workspace{}, workspace.ErrWorkspaceNotFound
	}

	return updated.Value(), nil
}

func (r *workspaceRepositoryImpl) DeleteByID(
	ctx context.Context,
	productID string,
	organizationID string,
	workspaceID string,
) error {
	deleteStmt := table.Workspaces.DELETE().USING(table.Organizations).WHERE(
		table.Workspaces.ID.EQ(postgres.String(workspaceID)).AND(
			r.mutationScopedWhere(productID, organizationID),
		),
	)

	if err := transactor.Exec(ctx, r.db, deleteStmt).Err(); err != nil {
		return err
	}

	found, err := r.FindByID(ctx, productID, organizationID, workspaceID)
	if err != nil {
		return err
	}
	if found.IsPresent() {
		return fault.ErrUnexpected
	}

	return nil
}

func (r *workspaceRepositoryImpl) SearchByOrganizationID(
	ctx context.Context,
	productID string,
	organizationID string,
	input search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace],
) (search.Result[workspace.Workspace], error) {
	whereStmt := r.scopedWhere(productID, organizationID)

	if input.Filter != nil {
		if ids := jetx.BuildIDFilter(table.Workspaces.ID, input.Filter.IDs); ids != nil {
			whereStmt = whereStmt.AND(ids)
		}
		if names := jetx.BuildStringArrayFilter(
			table.Workspaces.Name, input.Filter.Names,
		); names != nil {
			whereStmt = whereStmt.AND(names)
		}
	}

	if input.FullTextSearch != nil && *input.FullTextSearch != "" {
		if fullText := jetx.BuildSubstringFilter(
			[]postgres.ColumnString{table.Workspaces.Name}, *input.FullTextSearch,
		); fullText != nil {
			whereStmt = whereStmt.AND(fullText)
		}
	}

	order, err := transactor.StrictKeys(
		input.Sort,
		map[workspace.SortFieldProductWorkspace]transactor.Key[model.Workspaces]{
			workspace.SortFieldProductWorkspaceCreatedAt: transactor.TimeKey(
				table.Workspaces.CreatedAt, func(row model.Workspaces) time.Time { return row.CreatedAt },
			),
			workspace.SortFieldProductWorkspaceUpdatedAt: transactor.TimeKey(
				table.Workspaces.UpdatedAt, func(row model.Workspaces) time.Time { return row.UpdatedAt },
			),
			workspace.SortFieldProductWorkspaceName: transactor.StringKey(
				table.Workspaces.Name, func(row model.Workspaces) string { return row.Name },
			),
		},
		transactor.StringKey(table.Workspaces.ID, func(row model.Workspaces) string { return row.ID }),
	)
	if err != nil {
		return search.Result[workspace.Workspace]{}, err
	}

	return transactor.Page(r.db, r.workspaceMapper.ToDomain, table.Workspaces.AllColumns).
		From(r.joinOrganizations()).
		Where(whereStmt).
		Keyset(order).
		Run(ctx, input.Pagination).
		Value()
}

func (r *workspaceRepositoryImpl) joinOrganizations() postgres.ReadableTable {
	return table.Workspaces.INNER_JOIN(
		table.Organizations,
		table.Workspaces.OrganizationID.EQ(table.Organizations.ID),
	)
}

func (r *workspaceRepositoryImpl) scopedWhere(
	productID string,
	organizationID string,
) postgres.BoolExpression {
	return table.Workspaces.OrganizationID.EQ(postgres.String(organizationID)).AND(
		table.Organizations.ProductID.EQ(postgres.String(productID)),
	)
}

func (r *workspaceRepositoryImpl) mutationScopedWhere(
	productID string,
	organizationID string,
) postgres.BoolExpression {
	return table.Workspaces.OrganizationID.EQ(table.Organizations.ID).AND(
		r.scopedWhere(productID, organizationID),
	)
}
