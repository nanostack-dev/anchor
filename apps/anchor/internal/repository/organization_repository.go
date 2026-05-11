package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/organization"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

var _ OrganizationRepository = (*organizationRepositoryImpl)(nil)

func organizationsUpdatableColumns() postgres.ColumnList {
	return table.Organizations.AllColumns.Except(
		table.Organizations.CreatedAt, table.Organizations.UpdatedAt,
	)
}

type OrganizationRepository interface {
	FindByID(
		ctx context.Context, productID string, id string, options *toolkit.DBOptions,
	) (*organization.Organization, error)
	Create(ctx context.Context, org organization.Organization, options *toolkit.DBOptions) (
		organization.Organization, error,
	)
	Update(
		ctx context.Context, productID string, organization organization.Organization,
		options *toolkit.DBOptions,
	) (organization.Organization, error)
	DeleteByID(ctx context.Context, productID string, id string, options *toolkit.DBOptions) error
	SearchByProductID(
		ctx context.Context, productID string,
		input search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization],
		options *toolkit.DBOptions,
	) (search.Result[organization.Organization], error)
}

type organizationRepositoryImpl struct {
	db                 *sql.DB
	organizationMapper *mapper.OrganizationMapper
	logger             zerolog.Logger
}

func NewOrganizationRepository(
	db *sql.DB,
	organizationMapper *mapper.OrganizationMapper,
	logger zerolog.Logger,
) OrganizationRepository {
	return &organizationRepositoryImpl{
		db:                 db,
		organizationMapper: organizationMapper,
		logger: logger.With().Str(
			"component", "organization_repository",
		).Logger(),
	}
}

func (r *organizationRepositoryImpl) FindByID(
	ctx context.Context, productID string, id string, options *toolkit.DBOptions,
) (*organization.Organization, error) {
	stmt := table.Organizations.SELECT(
		table.Organizations.AllColumns,
	).FROM(
		table.Organizations,
	).WHERE(
		table.Organizations.ID.EQ(postgres.String(id)).AND(
			table.Organizations.ProductID.EQ(postgres.String(productID)),
		),
	).LIMIT(1)

	return toolkit.QueryOptionalMap[model.Organizations, organization.Organization](
		ctx, r.db, stmt,
		r.organizationMapper.ToDomain, options,
	)
}

func (r *organizationRepositoryImpl) Create(
	ctx context.Context, org organization.Organization, options *toolkit.DBOptions,
) (organization.Organization, error) {
	entity := r.organizationMapper.ToEntity(org)

	stmt := table.Organizations.INSERT(
		organizationsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.Organizations.AllColumns)

	return toolkit.QueryMap[model.Organizations, organization.Organization](
		ctx, r.db, stmt, r.organizationMapper.ToDomain, options,
	)
}

func (r *organizationRepositoryImpl) Update(
	ctx context.Context, productID string, currentOrg organization.Organization,
	options *toolkit.DBOptions,
) (organization.Organization, error) {
	currentOrg.UpdatedAt = time.Now()
	entityToUpdate := r.organizationMapper.ToEntity(currentOrg)

	updateStmt := table.Organizations.UPDATE(
		organizationsUpdatableColumns().Except(
			table.Organizations.ID,
			table.Organizations.ProductID,
		),
	).MODEL(
		entityToUpdate,
	).WHERE(
		table.Organizations.ID.EQ(postgres.String(currentOrg.ID)).AND(
			table.Organizations.ProductID.EQ(postgres.String(productID)),
		),
	).RETURNING(table.Organizations.AllColumns)

	return toolkit.QueryMap[model.Organizations, organization.Organization](
		ctx, r.db, updateStmt, r.organizationMapper.ToDomain, options,
	)
}

func (r *organizationRepositoryImpl) DeleteByID(
	ctx context.Context, productID string, id string, options *toolkit.DBOptions,
) error {
	stmt := table.Organizations.DELETE().WHERE(
		table.Organizations.ID.EQ(postgres.String(id)).AND(
			table.Organizations.ProductID.EQ(postgres.String(productID)),
		),
	)

	return toolkit.Exec(ctx, r.db, stmt, options)
}

func (r *organizationRepositoryImpl) SearchByProductID(
	ctx context.Context, productID string,
	input search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization],
	options *toolkit.DBOptions,
) (search.Result[organization.Organization], error) {
	whereStmt := table.Organizations.ProductID.EQ(postgres.String(productID))

	if input.Filter != nil {
		filterBuilder := search.NewFilterBuilder()

		if ids := filterBuilder.BuildIDFilter(
			table.Organizations.ID, input.Filter.IDs,
		); ids != nil {
			whereStmt = whereStmt.AND(ids)
		}
		if names := filterBuilder.BuildStringArrayFilter(
			table.Organizations.Name, input.Filter.Names,
		); names != nil {
			whereStmt = whereStmt.AND(names)
		}
	}

	if input.FullTextSearch != nil && *input.FullTextSearch != "" {
		filterBuilder := search.NewFilterBuilder()
		searchColumns := []postgres.ColumnString{
			table.Organizations.Name,
		}
		fullTextFilter := filterBuilder.BuildFullTextSearchFilter(
			searchColumns, *input.FullTextSearch,
		)
		if fullTextFilter != nil {
			whereStmt = whereStmt.AND(fullTextFilter)
		}
	}

	query := table.Organizations.SELECT(
		table.Organizations.AllColumns,
	).WHERE(whereStmt)

	total, err := toolkit.QueryCountWithBoolExpression(
		ctx, r.db, table.Organizations, whereStmt, options,
	)
	if err != nil {
		return search.Result[organization.Organization]{}, err
	}

	if len(input.Sort) > 0 {
		for _, sort := range input.Sort {
			switch sort.Field {
			case organization.SortFieldProductOrganizationCreatedAt:
				query = query.ORDER_BY(
					search.OrderBy(
						table.Organizations.CreatedAt, sort.Direction,
					),
				)
			case organization.SortFieldProductOrganizationUpdatedAt:
				query = query.ORDER_BY(
					search.OrderBy(
						table.Organizations.UpdatedAt, sort.Direction,
					),
				)
			case organization.SortFieldProductOrganizationName:
				query = query.ORDER_BY(search.OrderBy(table.Organizations.Name, sort.Direction))
			}
		}
	}

	query = query.LIMIT(int64(input.Pagination.Limit)).OFFSET(int64(input.Pagination.Offset))

	entities, err := toolkit.QueryMapSlice(ctx, r.db, query, r.organizationMapper.ToDomain, options)
	if err != nil {
		return search.Result[organization.Organization]{}, err
	}

	return search.Result[organization.Organization]{
		Items: entities,
		Total: total,
		Count: len(entities),
	}, nil
}
