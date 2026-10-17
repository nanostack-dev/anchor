package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/integration"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

var _ IntegrationAuditLogRepository = (*integrationAuditLogRepositoryImpl)(nil)

type IntegrationAuditLogRepository interface {
	Create(
		ctx context.Context, log integration.AuditLog,
	) (integration.AuditLog, error)
	// ListByInstanceInternal lists audit logs by instance ID without tenant
	// scoping. Reserved for trusted system-internal paths where no
	// authenticated tenant context exists. Must NOT be called from
	// tenant-facing API handlers. Use SearchByInstanceScoped for API paths.
	ListByInstanceInternal(
		ctx context.Context, instanceID string, limit int64,
	) ([]integration.AuditLog, error)
	// SearchByInstanceScoped pages the audit logs of one instance of the
	// tenant's product with a cursor.
	SearchByInstanceScoped(
		ctx context.Context, input integration.SearchAuditLogsInput,
	) (search.Result[integration.AuditLog], error)
}

type integrationAuditLogRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.IntegrationAuditLogMapper
	logger zerolog.Logger
}

func NewIntegrationAuditLogRepository(
	db *sql.DB, m *mapper.IntegrationAuditLogMapper, logger zerolog.Logger,
) IntegrationAuditLogRepository {
	return &integrationAuditLogRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "integration_audit_log_repository").Logger(),
	}
}

func (r *integrationAuditLogRepositoryImpl) Create(
	ctx context.Context, log integration.AuditLog,
) (integration.AuditLog, error) {
	entity := r.mapper.ToEntity(log)

	// Audit logs are immutable — insert all columns except auto-generated created_at.
	stmt := table.IntegrationAuditLogs.INSERT(
		table.IntegrationAuditLogs.AllColumns.Except(table.IntegrationAuditLogs.CreatedAt),
	).MODEL(entity).RETURNING(table.IntegrationAuditLogs.AllColumns)

	return transactor.QueryMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).Value()
}

func (r *integrationAuditLogRepositoryImpl) ListByInstanceInternal(
	ctx context.Context, instanceID string, limit int64,
) ([]integration.AuditLog, error) {
	stmt := table.IntegrationAuditLogs.SELECT(
		table.IntegrationAuditLogs.AllColumns,
	).FROM(
		table.IntegrationAuditLogs,
	).WHERE(
		table.IntegrationAuditLogs.IntegrationInstanceID.EQ(postgres.String(instanceID)),
	).ORDER_BY(
		table.IntegrationAuditLogs.CreatedAt.DESC(),
	).LIMIT(limit)

	return transactor.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

func (r *integrationAuditLogRepositoryImpl) SearchByInstanceScoped(
	ctx context.Context, input integration.SearchAuditLogsInput,
) (search.Result[integration.AuditLog], error) {
	auditLogsTable := table.IntegrationAuditLogs
	instancesTable := table.IntegrationInstances

	where := auditLogsTable.IntegrationInstanceID.EQ(postgres.String(input.IntegrationInstanceID)).AND(
		instancesTable.PlatformTenantID.EQ(postgres.String(input.TenantID)),
	).AND(
		instancesTable.ProductID.EQ(postgres.String(input.ProductID)),
	)

	if filter := input.Request.Filter; filter != nil {
		if actions := jetx.BuildStringArrayFilter(auditLogsTable.Action, filter.Actions); actions != nil {
			where = where.AND(actions)
		}
		if len(filter.Severities) > 0 {
			severities := functional.Slice(filter.Severities).Map(func(severity integration.AuditSeverity) string {
				return string(severity)
			})
			where = where.AND(auditLogsTable.Severity.IN(jetx.ToStringExpressions(severities)...))
		}
		if filter.CreatedAfter != nil {
			where = where.AND(auditLogsTable.CreatedAt.GT_EQ(postgres.TimestampzT(*filter.CreatedAfter)))
		}
		if filter.CreatedBefore != nil {
			where = where.AND(auditLogsTable.CreatedAt.LT(postgres.TimestampzT(*filter.CreatedBefore)))
		}
	}

	order, err := transactor.StrictKeys(
		input.Request.Sort,
		map[integration.SortFieldAuditLog]transactor.Key[model.IntegrationAuditLogs]{
			integration.SortFieldAuditLogCreatedAt: transactor.TimeKey(
				auditLogsTable.CreatedAt, func(row model.IntegrationAuditLogs) time.Time { return row.CreatedAt },
			),
			integration.SortFieldAuditLogAction: transactor.StringKey(
				auditLogsTable.Action, func(row model.IntegrationAuditLogs) string { return row.Action },
			),
			integration.SortFieldAuditLogSeverity: transactor.StringKey(
				auditLogsTable.Severity, func(row model.IntegrationAuditLogs) string { return row.Severity },
			),
		},
		transactor.StringKey(auditLogsTable.ID, func(row model.IntegrationAuditLogs) string { return row.ID }),
	)
	if err != nil {
		return search.Result[integration.AuditLog]{}, err
	}

	return transactor.Page(r.db, r.mapper.ToDomain, auditLogsTable.AllColumns).
		From(auditLogsTable.INNER_JOIN(instancesTable, auditLogsTable.IntegrationInstanceID.EQ(instancesTable.ID))).
		Where(where).
		Keyset(order).
		Run(ctx, input.Request.Pagination).
		Value()
}
