package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

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
		ctx context.Context, productID string, id string,
	) (*organization.Organization, error)
	Create(ctx context.Context, org organization.Organization) (
		organization.Organization, error,
	)
	Update(
		ctx context.Context, productID string, organization organization.Organization,
	) (organization.Organization, error)
	DeleteByID(ctx context.Context, productID string, id string) error
	SearchByProductID(
		ctx context.Context, productID string,
		input search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization],
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
	ctx context.Context, productID string, id string,
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

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		r.organizationMapper.ToDomain,
	).Value()
}

func (r *organizationRepositoryImpl) Create(
	ctx context.Context, org organization.Organization,
) (organization.Organization, error) {
	entity := r.organizationMapper.ToEntity(org)

	stmt := table.Organizations.INSERT(
		organizationsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.Organizations.AllColumns)

	return transactor.QueryMap(
		ctx, r.db, stmt, r.organizationMapper.ToDomain,
	).Value()
}

func (r *organizationRepositoryImpl) Update(
	ctx context.Context, productID string, currentOrg organization.Organization,
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

	return transactor.QueryMap(
		ctx, r.db, updateStmt, r.organizationMapper.ToDomain,
	).Value()
}

func (r *organizationRepositoryImpl) DeleteByID(
	ctx context.Context, productID string, id string,
) error {
	stmt := table.Organizations.DELETE().WHERE(
		table.Organizations.ID.EQ(postgres.String(id)).AND(
			table.Organizations.ProductID.EQ(postgres.String(productID)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *organizationRepositoryImpl) SearchByProductID(
	ctx context.Context, productID string,
	input search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization],
) (search.Result[organization.Organization], error) {
	whereStmt := table.Organizations.ProductID.EQ(postgres.String(productID))

	if input.Filter != nil {
		filterBuilder := jetx.NewFilterBuilder()

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
		filterBuilder := jetx.NewFilterBuilder()
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

	total, err := transactor.QueryCount(
		ctx,
		r.db,
		table.Organizations.SELECT(postgres.COUNT(postgres.STAR)).WHERE(whereStmt),
	).Value()
	if err != nil {
		return search.Result[organization.Organization]{}, err
	}

	if len(input.Sort) > 0 {
		for _, sort := range input.Sort {
			switch sort.Field {
			case organization.SortFieldProductOrganizationCreatedAt:
				query = query.ORDER_BY(
					jetx.OrderBy(
						table.Organizations.CreatedAt, sort.Direction,
					),
				)
			case organization.SortFieldProductOrganizationUpdatedAt:
				query = query.ORDER_BY(
					jetx.OrderBy(
						table.Organizations.UpdatedAt, sort.Direction,
					),
				)
			case organization.SortFieldProductOrganizationName:
				query = query.ORDER_BY(jetx.OrderBy(table.Organizations.Name, sort.Direction))
			}
		}
	}

	query = query.LIMIT(int64(input.Pagination.Limit)).OFFSET(int64(input.Pagination.Offset))

	entities, err := transactor.QueryMapSlice(ctx, r.db, query, r.organizationMapper.ToDomain).Value()
	if err != nil {
		return search.Result[organization.Organization]{}, err
	}

	return search.Result[organization.Organization]{
		Items: entities,
		Total: total,
		Count: len(entities),
	}, nil
}
