package repository

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/shared/toolkit"

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
		ctx context.Context, log integration.AuditLog, options *toolkit.DBOptions,
	) (integration.AuditLog, error)
	// ListByInstanceInternal lists audit logs by instance ID without tenant
	// scoping. Reserved for trusted system-internal paths where no
	// authenticated tenant context exists. Must NOT be called from
	// tenant-facing API handlers. Use ListByInstanceScoped for API paths.
	ListByInstanceInternal(
		ctx context.Context, instanceID string, limit int64, options *toolkit.DBOptions,
	) ([]integration.AuditLog, error)
	ListByInstanceScoped(
		ctx context.Context,
		tenantID string,
		productID string,
		instanceID string,
		limit int64,
		offset int64,
		options *toolkit.DBOptions,
	) ([]integration.AuditLog, error)
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
	ctx context.Context, log integration.AuditLog, options *toolkit.DBOptions,
) (integration.AuditLog, error) {
	entity := r.mapper.ToEntity(log)

	// Audit logs are immutable — insert all columns except auto-generated created_at.
	stmt := table.IntegrationAuditLogs.INSERT(
		table.IntegrationAuditLogs.AllColumns.Except(table.IntegrationAuditLogs.CreatedAt),
	).MODEL(entity).RETURNING(table.IntegrationAuditLogs.AllColumns)

	return toolkit.QueryMap[model.IntegrationAuditLogs, integration.AuditLog](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *integrationAuditLogRepositoryImpl) ListByInstanceInternal(
	ctx context.Context, instanceID string, limit int64, options *toolkit.DBOptions,
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

	return toolkit.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain, options)
}

func (r *integrationAuditLogRepositoryImpl) ListByInstanceScoped(
	ctx context.Context,
	tenantID string,
	productID string,
	instanceID string,
	limit int64,
	offset int64,
	options *toolkit.DBOptions,
) ([]integration.AuditLog, error) {
	auditLogsTable := table.IntegrationAuditLogs
	instancesTable := table.IntegrationInstances

	stmt := auditLogsTable.SELECT(
		auditLogsTable.AllColumns,
	).FROM(
		auditLogsTable.INNER_JOIN(instancesTable, auditLogsTable.IntegrationInstanceID.EQ(instancesTable.ID)),
	).WHERE(
		auditLogsTable.IntegrationInstanceID.EQ(postgres.String(instanceID)).AND(
			instancesTable.PlatformTenantID.EQ(postgres.String(tenantID)),
		).AND(
			instancesTable.ProductID.EQ(postgres.String(productID)),
		),
	).ORDER_BY(
		auditLogsTable.CreatedAt.DESC(),
	).LIMIT(limit).OFFSET(offset)

	return toolkit.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain, options)
}
