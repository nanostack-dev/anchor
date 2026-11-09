package repository

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/integration"
	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
)

// The base returns model rows: the repository maps them with the audit log mapper.
func _() {
	nanorepo.Implement[IntegrationAuditLogRepository, integrationAuditLogRepositoryImpl](
		table.IntegrationAuditLogs, nanorepo.NoMapper[model.IntegrationAuditLogs],
	)

	// POST /v1/products/{product_id}/integrations/{id}/audit-logs/search, and the
	// deprecated GET list. The audit log has no tenant column: the join to its
	// instance carries the tenant and the product, and the route names the
	// instance. created_after is inclusive and created_before exclusive.
	nanorepo.Search[model.IntegrationAuditLogs]("IntegrationAuditLog", table.IntegrationAuditLogs,
		nanorepo.Map((*mapper.IntegrationAuditLogMapper).ToDomain),
		nanorepo.From(auditLogsOfTheirInstance),
		nanorepo.Scope(table.IntegrationInstances.PlatformTenantID),
		nanorepo.Scope(table.IntegrationInstances.ProductID),
		nanorepo.Scope(table.IntegrationAuditLogs.IntegrationInstanceID),
		nanorepo.Filter(table.IntegrationAuditLogs.Action, nanorepo.In),
		nanorepo.FilterAs[integration.AuditSeverity](table.IntegrationAuditLogs.Severity, nanorepo.In),
		nanorepo.Filter(table.IntegrationAuditLogs.CreatedAt,
			nanorepo.Range("created_after", "created_before").HalfOpen()),
		nanorepo.Sort(
			table.IntegrationAuditLogs.CreatedAt, table.IntegrationAuditLogs.Action,
			table.IntegrationAuditLogs.Severity,
		),
		nanorepo.DefaultSort(table.IntegrationAuditLogs.CreatedAt, nanorepo.Desc),
		nanorepo.DefaultLimit(defaultPageLimit),
	)
}

// auditLogsOfTheirInstance reads the audit logs with the instance that carries
// their tenant and product.
func auditLogsOfTheirInstance() postgres.ReadableTable {
	return table.IntegrationAuditLogs.INNER_JOIN(
		table.IntegrationInstances,
		table.IntegrationAuditLogs.IntegrationInstanceID.EQ(table.IntegrationInstances.ID),
	)
}

type IntegrationAuditLogRepository interface {
	Create(
		ctx context.Context, log integration.AuditLog,
	) (integration.AuditLog, error)
	// SearchByInstanceScoped pages the audit logs of one instance of the
	// tenant's product with a cursor.
	SearchByInstanceScoped(
		ctx context.Context, q IntegrationAuditLogSearch,
	) (search.Result[integration.AuditLog], error)
}

type integrationAuditLogRepositoryImpl struct {
	integrationAuditLogsBase

	db     *sql.DB
	mapper *mapper.IntegrationAuditLogMapper
	logger *slog.Logger
}

func NewIntegrationAuditLogRepository(
	db *sql.DB, m *mapper.IntegrationAuditLogMapper, logger *slog.Logger,
) IntegrationAuditLogRepository {
	return &integrationAuditLogRepositoryImpl{
		integrationAuditLogsBase: newIntegrationAuditLogsBase(db),
		db:                       db,
		mapper:                   m,
		logger:                   logger.With(slog.String("component", "integration_audit_log_repository")),
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
