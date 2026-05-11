package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"
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
	FindByID(
		ctx context.Context,
		productID string,
		organizationID string,
		workspaceID string,
		options *toolkit.DBOptions,
	) (*workspace.Workspace, error)
	FindByOrganizationIDAndName(
		ctx context.Context,
		productID string,
		organizationID string,
		name string,
		options *toolkit.DBOptions,
	) (*workspace.Workspace, error)
	Create(
		ctx context.Context,
		workspace workspace.Workspace,
		options *toolkit.DBOptions,
	) (workspace.Workspace, error)
	Update(
		ctx context.Context,
		productID string,
		organizationID string,
		workspace workspace.Workspace,
		options *toolkit.DBOptions,
	) (workspace.Workspace, error)
	DeleteByID(
		ctx context.Context,
		productID string,
		organizationID string,
		workspaceID string,
		options *toolkit.DBOptions,
	) error
	SearchByOrganizationID(
		ctx context.Context,
		productID string,
		organizationID string,
		input search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace],
		options *toolkit.DBOptions,
	) (search.Result[workspace.Workspace], error)
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
	options *toolkit.DBOptions,
) (*workspace.Workspace, error) {
	stmt := table.Workspaces.SELECT(
		table.Workspaces.AllColumns,
	).FROM(
		r.joinOrganizations(),
	).WHERE(
		r.scopedWhere(productID, organizationID).AND(
			table.Workspaces.ID.EQ(postgres.String(workspaceID)),
		),
	).LIMIT(1)

	return toolkit.QueryOptionalMap[model.Workspaces, workspace.Workspace](
		ctx,
		r.db,
		stmt,
		r.workspaceMapper.ToDomain,
		options,
	)
}

func (r *workspaceRepositoryImpl) FindByOrganizationIDAndName(
	ctx context.Context,
	productID string,
	organizationID string,
	name string,
	options *toolkit.DBOptions,
) (*workspace.Workspace, error) {
	stmt := table.Workspaces.SELECT(
		table.Workspaces.AllColumns,
	).FROM(
		r.joinOrganizations(),
	).WHERE(
		r.scopedWhere(productID, organizationID).AND(
			table.Workspaces.Name.EQ(postgres.String(name)),
		),
	).LIMIT(1)

	return toolkit.QueryOptionalMap[model.Workspaces, workspace.Workspace](
		ctx,
		r.db,
		stmt,
		r.workspaceMapper.ToDomain,
		options,
	)
}

func (r *workspaceRepositoryImpl) Create(
	ctx context.Context,
	newWorkspace workspace.Workspace,
	options *toolkit.DBOptions,
) (workspace.Workspace, error) {
	entity := r.workspaceMapper.ToEntity(newWorkspace)

	stmt := table.Workspaces.INSERT(
		workspacesUpdatableColumns(),
	).MODEL(entity).RETURNING(table.Workspaces.AllColumns)

	created, err := toolkit.QueryMap[model.Workspaces, workspace.Workspace](
		ctx,
		r.db,
		stmt,
		r.workspaceMapper.ToDomain,
		options,
	)
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
	options *toolkit.DBOptions,
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

	if err := toolkit.Exec(ctx, r.db, updateStmt, options); err != nil {
		return workspace.Workspace{}, err
	}

	updated, err := r.FindByID(ctx, productID, organizationID, currentWorkspace.ID, options)
	if err != nil {
		return workspace.Workspace{}, err
	}
	if updated == nil {
		return workspace.Workspace{}, toolkit.ErrNotFound
	}

	return *updated, nil
}

func (r *workspaceRepositoryImpl) DeleteByID(
	ctx context.Context,
	productID string,
	organizationID string,
	workspaceID string,
	options *toolkit.DBOptions,
) error {
	deleteStmt := table.Workspaces.DELETE().USING(table.Organizations).WHERE(
		table.Workspaces.ID.EQ(postgres.String(workspaceID)).AND(
			r.mutationScopedWhere(productID, organizationID),
		),
	)

	if err := toolkit.Exec(ctx, r.db, deleteStmt, options); err != nil {
		return err
	}

	found, err := r.FindByID(ctx, productID, organizationID, workspaceID, options)
	if err != nil {
		return err
	}
	if found != nil {
		return toolkit.ErrUnexpected
	}

	return nil
}

func (r *workspaceRepositoryImpl) SearchByOrganizationID(
	ctx context.Context,
	productID string,
	organizationID string,
	input search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace],
	options *toolkit.DBOptions,
) (search.Result[workspace.Workspace], error) {
	whereStmt := r.scopedWhere(productID, organizationID)

	if input.Filter != nil {
		filterBuilder := search.NewFilterBuilder()

		if ids := filterBuilder.BuildIDFilter(table.Workspaces.ID, input.Filter.IDs); ids != nil {
			whereStmt = whereStmt.AND(ids)
		}
		if names := filterBuilder.BuildStringArrayFilter(table.Workspaces.Name, input.Filter.Names); names != nil {
			whereStmt = whereStmt.AND(names)
		}
	}

	if input.FullTextSearch != nil && *input.FullTextSearch != "" {
		filterBuilder := search.NewFilterBuilder()
		fullTextFilter := filterBuilder.BuildFullTextSearchFilter(
			[]postgres.ColumnString{table.Workspaces.Name},
			*input.FullTextSearch,
		)
		if fullTextFilter != nil {
			whereStmt = whereStmt.AND(fullTextFilter)
		}
	}

	countStmt := postgres.SELECT(
		postgres.COUNT(postgres.STAR).AS("count_result.count"),
	).FROM(r.joinOrganizations()).WHERE(whereStmt)

	total, err := toolkit.QueryCountWithStatement(ctx, r.db, countStmt, options)
	if err != nil {
		return search.Result[workspace.Workspace]{}, err
	}

	query := table.Workspaces.SELECT(
		table.Workspaces.AllColumns,
	).FROM(
		r.joinOrganizations(),
	).WHERE(whereStmt)

	if len(input.Sort) > 0 {
		for _, sort := range input.Sort {
			switch sort.Field {
			case workspace.SortFieldProductWorkspaceCreatedAt:
				query = query.ORDER_BY(search.OrderBy(table.Workspaces.CreatedAt, sort.Direction))
			case workspace.SortFieldProductWorkspaceUpdatedAt:
				query = query.ORDER_BY(search.OrderBy(table.Workspaces.UpdatedAt, sort.Direction))
			case workspace.SortFieldProductWorkspaceName:
				query = query.ORDER_BY(search.OrderBy(table.Workspaces.Name, sort.Direction))
			}
		}
	}

	query = query.LIMIT(int64(input.Pagination.Limit)).OFFSET(int64(input.Pagination.Offset))

	items, err := toolkit.QueryMapSlice(ctx, r.db, query, r.workspaceMapper.ToDomain, options)
	if err != nil {
		return search.Result[workspace.Workspace]{}, err
	}

	return search.Result[workspace.Workspace]{
		Items: items,
		Total: total,
		Count: len(items),
	}, nil
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
