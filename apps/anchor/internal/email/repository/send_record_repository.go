package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/email"
	"anchor/internal/mapper"
)

var _ SendRecordRepository = (*sendRecordRepositoryImpl)(nil)

func emailSendRecordsUpdatableColumns() postgres.ColumnList {
	return table.EmailSendRecords.AllColumns.Except(
		table.EmailSendRecords.CreatedAt, table.EmailSendRecords.UpdatedAt,
	)
}

type sendRecordRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.EmailSendRecordMapper
	logger zerolog.Logger
}

func NewSendRecordRepository(
	db *sql.DB, m *mapper.EmailSendRecordMapper, logger zerolog.Logger,
) SendRecordRepository {
	return &sendRecordRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "email_send_record_repository").Logger(),
	}
}

func (r *sendRecordRepositoryImpl) Create(
	ctx context.Context, _ string, _ string, record email.SendRecord, options *jetx.DBOptions,
) (email.SendRecord, error) {
	return r.CreateInternal(ctx, record, options)
}

func (r *sendRecordRepositoryImpl) CreateInternal(
	ctx context.Context, record email.SendRecord, options *jetx.DBOptions,
) (email.SendRecord, error) {
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	entity := r.mapper.ToEntity(record)
	stmt := table.EmailSendRecords.INSERT(emailSendRecordsUpdatableColumns()).
		MODEL(entity).
		RETURNING(table.EmailSendRecords.AllColumns)
	return jetx.QueryMap[model.EmailSendRecords, email.SendRecord](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *sendRecordRepositoryImpl) UpdateStatus(
	ctx context.Context,
	tenantID string,
	id string,
	status email.SendStatus,
	lastError *string,
	sentAt *time.Time,
	options *jetx.DBOptions,
) error {
	now := time.Now()
	entity := model.EmailSendRecords{
		Status:    string(status),
		LastError: lastError,
		SentAt:    sentAt,
		UpdatedAt: now,
	}
	cols := postgres.ColumnList{
		table.EmailSendRecords.Status,
		table.EmailSendRecords.LastError,
		table.EmailSendRecords.SentAt,
		table.EmailSendRecords.UpdatedAt,
	}
	stmt := table.EmailSendRecords.UPDATE(cols).MODEL(entity).WHERE(
		table.EmailSendRecords.ID.EQ(postgres.String(id)).
			AND(table.EmailSendRecords.PlatformTenantID.EQ(postgres.String(tenantID))),
	)
	return jetx.Exec(ctx, r.db, stmt, options)
}

func (r *sendRecordRepositoryImpl) UpdateStatusInternal(
	ctx context.Context,
	id string,
	status email.SendStatus,
	lastError *string,
	sentAt *time.Time,
	options *jetx.DBOptions,
) error {
	now := time.Now()
	entity := model.EmailSendRecords{
		Status:    string(status),
		LastError: lastError,
		SentAt:    sentAt,
		UpdatedAt: now,
	}
	cols := postgres.ColumnList{
		table.EmailSendRecords.Status,
		table.EmailSendRecords.LastError,
		table.EmailSendRecords.SentAt,
		table.EmailSendRecords.UpdatedAt,
	}
	stmt := table.EmailSendRecords.UPDATE(cols).MODEL(entity).WHERE(
		table.EmailSendRecords.ID.EQ(postgres.String(id)),
	)
	return jetx.Exec(ctx, r.db, stmt, options)
}

func (r *sendRecordRepositoryImpl) FindByDedupeKey(
	ctx context.Context, tenantID string, productID string, dedupeKey string, options *jetx.DBOptions,
) (*email.SendRecord, error) {
	stmt := table.EmailSendRecords.SELECT(table.EmailSendRecords.AllColumns).
		FROM(table.EmailSendRecords).
		WHERE(
			table.EmailSendRecords.PlatformTenantID.EQ(postgres.String(tenantID)).
				AND(table.EmailSendRecords.ProductID.EQ(postgres.String(productID))).
				AND(table.EmailSendRecords.DedupeKey.EQ(postgres.String(dedupeKey))),
		).LIMIT(1)
	return jetx.QueryOptionalMap[model.EmailSendRecords, email.SendRecord](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *sendRecordRepositoryImpl) FindByDedupeKeyInternal(
	ctx context.Context, productID string, dedupeKey string, options *jetx.DBOptions,
) (*email.SendRecord, error) {
	stmt := table.EmailSendRecords.SELECT(table.EmailSendRecords.AllColumns).
		FROM(table.EmailSendRecords).
		WHERE(
			table.EmailSendRecords.ProductID.EQ(postgres.String(productID)).
				AND(table.EmailSendRecords.DedupeKey.EQ(postgres.String(dedupeKey))),
		).LIMIT(1)
	return jetx.QueryOptionalMap[model.EmailSendRecords, email.SendRecord](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *sendRecordRepositoryImpl) FindByID(
	ctx context.Context, tenantID string, productID string, id string, options *jetx.DBOptions,
) (*email.SendRecord, error) {
	stmt := table.EmailSendRecords.SELECT(table.EmailSendRecords.AllColumns).
		FROM(table.EmailSendRecords).
		WHERE(
			table.EmailSendRecords.ID.EQ(postgres.String(id)).
				AND(table.EmailSendRecords.PlatformTenantID.EQ(postgres.String(tenantID))).
				AND(table.EmailSendRecords.ProductID.EQ(postgres.String(productID))),
		).LIMIT(1)
	return jetx.QueryOptionalMap[model.EmailSendRecords, email.SendRecord](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *sendRecordRepositoryImpl) List(
	ctx context.Context, input email.ListSendsInput, options *jetx.DBOptions,
) ([]email.SendRecord, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	where := table.EmailSendRecords.PlatformTenantID.EQ(postgres.String(input.TenantID)).
		AND(table.EmailSendRecords.ProductID.EQ(postgres.String(input.ProductID)))
	if input.TemplateID != nil {
		where = where.AND(table.EmailSendRecords.TemplateID.EQ(postgres.String(*input.TemplateID)))
	}
	if input.Status != nil {
		where = where.AND(table.EmailSendRecords.Status.EQ(postgres.String(string(*input.Status))))
	}

	stmt := table.EmailSendRecords.SELECT(table.EmailSendRecords.AllColumns).
		FROM(table.EmailSendRecords).
		WHERE(where).
		ORDER_BY(table.EmailSendRecords.CreatedAt.DESC()).
		LIMIT(limit).OFFSET(input.Offset)
	return jetx.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain, options)
}

func (r *sendRecordRepositoryImpl) CountSince(
	ctx context.Context, tenantID string, productID string, since time.Time, options *jetx.DBOptions,
) (int64, error) {
	stmt := postgres.SELECT(postgres.COUNT(postgres.STAR).AS("count_result.count")).
		FROM(table.EmailSendRecords).
		WHERE(
			table.EmailSendRecords.PlatformTenantID.EQ(postgres.String(tenantID)).
				AND(table.EmailSendRecords.ProductID.EQ(postgres.String(productID))).
				AND(table.EmailSendRecords.CreatedAt.GT_EQ(postgres.TimestampzT(since))),
		)
	return jetx.QueryCountWithStatement(ctx, r.db, stmt, options)
}

func (r *sendRecordRepositoryImpl) CountSinceInternal(
	ctx context.Context, productID string, since time.Time, options *jetx.DBOptions,
) (int64, error) {
	stmt := postgres.SELECT(postgres.COUNT(postgres.STAR).AS("count_result.count")).
		FROM(table.EmailSendRecords).
		WHERE(
			table.EmailSendRecords.ProductID.EQ(postgres.String(productID)).
				AND(table.EmailSendRecords.CreatedAt.GT_EQ(postgres.TimestampzT(since))),
		)
	return jetx.QueryCountWithStatement(ctx, r.db, stmt, options)
}
