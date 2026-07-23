package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/plan"
	"anchor/internal/mapper"
)

var _ PlanRepository = (*planRepositoryImpl)(nil)

func plansUpdatableColumns() postgres.ColumnList {
	return table.Plans.AllColumns.Except(
		table.Plans.CreatedAt, table.Plans.UpdatedAt,
	)
}

type PlanRepository interface {
	FindByID(ctx context.Context, productID string, id string) (*plan.Plan, error)
	FindByKey(ctx context.Context, productID string, key string) (*plan.Plan, error)
	// FindDefault returns the product's default plan, if any.
	FindDefault(ctx context.Context, productID string) (*plan.Plan, error)
	ListByProduct(ctx context.Context, productID string) ([]plan.Plan, error)
	Create(ctx context.Context, p plan.Plan) (plan.Plan, error)
	Update(ctx context.Context, productID string, p plan.Plan) (plan.Plan, error)
	DeleteByID(ctx context.Context, productID string, id string) error
	// ClearDefaultExcept unsets is_default on every plan of the product other
	// than the given plan. Used to keep at most one default plan per product.
	ClearDefaultExcept(ctx context.Context, productID string, planID string) error
}

type planRepositoryImpl struct {
	db         *sql.DB
	planMapper *mapper.PlanMapper
	logger     zerolog.Logger
}

func NewPlanRepository(
	db *sql.DB, planMapper *mapper.PlanMapper, logger zerolog.Logger,
) PlanRepository {
	return &planRepositoryImpl{
		db:         db,
		planMapper: planMapper,
		logger:     logger.With().Str("component", "plan_repository").Logger(),
	}
}

func (r *planRepositoryImpl) FindByID(
	ctx context.Context, productID string, id string,
) (*plan.Plan, error) {
	stmt := table.Plans.SELECT(table.Plans.AllColumns).WHERE(
		table.Plans.ID.EQ(postgres.String(id)).AND(
			table.Plans.ProductID.EQ(postgres.String(productID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.planMapper.ToDomain)
}

func (r *planRepositoryImpl) FindByKey(
	ctx context.Context, productID string, key string,
) (*plan.Plan, error) {
	stmt := table.Plans.SELECT(table.Plans.AllColumns).WHERE(
		table.Plans.ProductID.EQ(postgres.String(productID)).AND(
			table.Plans.Key.EQ(postgres.String(key)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.planMapper.ToDomain)
}

func (r *planRepositoryImpl) FindDefault(
	ctx context.Context, productID string,
) (*plan.Plan, error) {
	stmt := table.Plans.SELECT(table.Plans.AllColumns).WHERE(
		table.Plans.ProductID.EQ(postgres.String(productID)).AND(
			table.Plans.IsDefault.IS_TRUE(),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.planMapper.ToDomain)
}

func (r *planRepositoryImpl) ListByProduct(
	ctx context.Context, productID string,
) ([]plan.Plan, error) {
	stmt := table.Plans.SELECT(table.Plans.AllColumns).WHERE(
		table.Plans.ProductID.EQ(postgres.String(productID)),
	).ORDER_BY(table.Plans.CreatedAt.ASC())

	return transactor.QueryMapSlice(ctx, r.db, stmt, r.planMapper.ToDomain)
}

func (r *planRepositoryImpl) Create(
	ctx context.Context, p plan.Plan,
) (plan.Plan, error) {
	entity := r.planMapper.ToEntity(p)

	stmt := table.Plans.INSERT(
		plansUpdatableColumns(),
	).MODEL(entity).RETURNING(table.Plans.AllColumns)

	created, err := transactor.Query[model.Plans](ctx, r.db, stmt)
	if err != nil {
		return plan.Plan{}, err
	}

	return r.planMapper.ToDomain(created), nil
}

func (r *planRepositoryImpl) Update(
	ctx context.Context, productID string, p plan.Plan,
) (plan.Plan, error) {
	p.UpdatedAt = time.Now()
	entity := r.planMapper.ToEntity(p)

	stmt := table.Plans.UPDATE(
		plansUpdatableColumns().Except(table.Plans.ID, table.Plans.ProductID),
	).MODEL(entity).WHERE(
		table.Plans.ID.EQ(postgres.String(p.ID)).AND(
			table.Plans.ProductID.EQ(postgres.String(productID)),
		),
	).RETURNING(table.Plans.AllColumns)

	updated, err := transactor.Query[model.Plans](ctx, r.db, stmt)
	if err != nil {
		return plan.Plan{}, err
	}

	return r.planMapper.ToDomain(updated), nil
}

func (r *planRepositoryImpl) DeleteByID(
	ctx context.Context, productID string, id string,
) error {
	stmt := table.Plans.DELETE().WHERE(
		table.Plans.ID.EQ(postgres.String(id)).AND(
			table.Plans.ProductID.EQ(postgres.String(productID)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt)
}

func (r *planRepositoryImpl) ClearDefaultExcept(
	ctx context.Context, productID string, planID string,
) error {
	stmt := table.Plans.UPDATE(table.Plans.IsDefault).SET(
		postgres.Bool(false),
	).WHERE(
		table.Plans.ProductID.EQ(postgres.String(productID)).AND(
			table.Plans.ID.NOT_EQ(postgres.String(planID)),
		).AND(table.Plans.IsDefault.IS_TRUE()),
	)

	return transactor.Exec(ctx, r.db, stmt)
}
