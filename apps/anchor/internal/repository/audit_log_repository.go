package repository

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/audit"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

var _ AuditLogRepository = (*auditLogRepositoryImpl)(nil)

// AuditLogRepository provides access to the general audit log.
// The audit log is append-only: this interface intentionally has no
// update or delete methods, and none may be added.
type AuditLogRepository interface {
	Create(ctx context.Context, log audit.Log) (audit.Log, error)

	// Search returns paginated, filtered audit log entries scoped to a tenant and product.
	Search(
		ctx context.Context,
		tenantID string,
		productID string,
		req search.Request[audit.SearchFilter, audit.SortField],
	) (search.Result[audit.Log], error)
}

type auditLogRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.AuditLogMapper
	logger zerolog.Logger
}

func NewAuditLogRepository(
	db *sql.DB,
	auditLogMapper *mapper.AuditLogMapper,
	logger zerolog.Logger,
) AuditLogRepository {
	return &auditLogRepositoryImpl{
		db:     db,
		mapper: auditLogMapper,
		logger: logger.With().Str("component", "audit_log_repository").Logger(),
	}
}

func (r *auditLogRepositoryImpl) Create(
	ctx context.Context, log audit.Log,
) (audit.Log, error) {
	entity := r.mapper.ToEntity(log)

	stmt := table.AuditLogs.INSERT(
		table.AuditLogs.AllColumns.Except(table.AuditLogs.CreatedAt),
	).MODEL(entity).RETURNING(table.AuditLogs.AllColumns)

	return transactor.QueryMap[model.AuditLogs, audit.Log](ctx, r.db, stmt, r.mapper.ToDomain)
}

func (r *auditLogRepositoryImpl) Search(
	ctx context.Context,
	tenantID string,
	productID string,
	req search.Request[audit.SearchFilter, audit.SortField],
) (search.Result[audit.Log], error) {
	whereStmt := table.AuditLogs.ProductID.EQ(postgres.String(productID)).AND(
		table.AuditLogs.PlatformTenantID.EQ(postgres.String(tenantID)),
	)

	if req.Filter != nil {
		if filterExpr := r.buildFilter(*req.Filter); filterExpr != nil {
			whereStmt = whereStmt.AND(filterExpr)
		}
	}

	if req.FullTextSearch != nil && *req.FullTextSearch != "" {
		filterBuilder := jetx.NewFilterBuilder()
		searchColumns := []postgres.ColumnString{
			table.AuditLogs.Action,
			table.AuditLogs.ActorName,
			table.AuditLogs.TargetName,
		}
		if fullTextFilter := filterBuilder.BuildFullTextSearchFilter(
			searchColumns, *req.FullTextSearch,
		); fullTextFilter != nil {
			whereStmt = whereStmt.AND(fullTextFilter)
		}
	}

	total, err := transactor.QueryCount(
		ctx,
		r.db,
		table.AuditLogs.SELECT(postgres.COUNT(postgres.STAR)).WHERE(whereStmt),
	)
	if err != nil {
		return search.Result[audit.Log]{}, err
	}

	direction := search.SortDescending
	if len(req.Sort) > 0 {
		direction = req.Sort[0].Direction
	}

	query := table.AuditLogs.SELECT(table.AuditLogs.AllColumns).
		WHERE(whereStmt).
		ORDER_BY(jetx.OrderBy(table.AuditLogs.CreatedAt, direction)).
		LIMIT(int64(req.Pagination.Limit)).
		OFFSET(int64(req.Pagination.Offset))

	entities, err := transactor.QueryMapSlice(ctx, r.db, query, r.mapper.ToDomain)
	if err != nil {
		return search.Result[audit.Log]{}, err
	}

	return search.Result[audit.Log]{
		Items: entities,
		Total: total,
		Count: len(entities),
	}, nil
}

func (r *auditLogRepositoryImpl) buildFilter(filter audit.SearchFilter) postgres.BoolExpression {
	filterBuilder := jetx.NewFilterBuilder()
	conditions := []postgres.BoolExpression{}

	if filter.OrganizationID != nil {
		conditions = append(
			conditions, table.AuditLogs.OrganizationID.EQ(postgres.String(*filter.OrganizationID)),
		)
	}
	if actions := filterBuilder.BuildStringArrayFilter(
		table.AuditLogs.Action, filter.Actions,
	); actions != nil {
		conditions = append(conditions, actions)
	}
	if len(filter.ActorTypes) > 0 {
		actorTypes := make([]string, 0, len(filter.ActorTypes))
		for _, actorType := range filter.ActorTypes {
			actorTypes = append(actorTypes, string(actorType))
		}
		if actorTypeFilter := filterBuilder.BuildStringArrayFilter(
			table.AuditLogs.ActorType, actorTypes,
		); actorTypeFilter != nil {
			conditions = append(conditions, actorTypeFilter)
		}
	}
	if filter.ActorID != nil {
		conditions = append(conditions, table.AuditLogs.ActorID.EQ(postgres.String(*filter.ActorID)))
	}
	if filter.TargetType != nil {
		conditions = append(
			conditions, table.AuditLogs.TargetType.EQ(postgres.String(*filter.TargetType)),
		)
	}
	if filter.TargetID != nil {
		conditions = append(conditions, table.AuditLogs.TargetID.EQ(postgres.String(*filter.TargetID)))
	}
	if filter.Outcome != nil {
		conditions = append(
			conditions, table.AuditLogs.Outcome.EQ(postgres.String(string(*filter.Outcome))),
		)
	}
	if dateRange := filterBuilder.BuildDateRangeFilter(
		table.AuditLogs.CreatedAt, filter.CreatedAfter, filter.CreatedBefore,
	); dateRange != nil {
		conditions = append(conditions, dateRange)
	}

	return filterBuilder.CombineFilters(conditions...)
}
