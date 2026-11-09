package repository

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/email"
	"anchor/internal/mapper"
)

func _() {
	nanorepo.Implement[SendRecordRepository, sendRecordRepositoryImpl](table.EmailSendRecords, emailSendRecordToDomain)

	// POST /v1/products/{product_id}/email/sends/search and the deprecated GET list.
	// full_text_search is accepted and ignored, as it always was.
	nanorepo.Search[model.EmailSendRecords]("EmailSend", table.EmailSendRecords,
		nanorepo.Map(emailSendRecordToDomain),
		nanorepo.Scope(table.EmailSendRecords.PlatformTenantID),
		nanorepo.Scope(table.EmailSendRecords.ProductID),
		nanorepo.Filter(table.EmailSendRecords.TemplateID, nanorepo.In),
		nanorepo.FilterAs[email.SendStatus](table.EmailSendRecords.Status, nanorepo.In),
		nanorepo.Sort(table.EmailSendRecords.CreatedAt, table.EmailSendRecords.Status),
		nanorepo.DefaultSort(table.EmailSendRecords.CreatedAt, nanorepo.Desc),
		nanorepo.DefaultLimit(10),
	)
}

// emailSendRecordToDomain maps the rows that the base returns. EmailSendRecordMapper has no state.
func emailSendRecordToDomain(row model.EmailSendRecords) email.SendRecord {
	return (&mapper.EmailSendRecordMapper{}).ToDomain(row)
}

func emailSendRecordsUpdatableColumns() postgres.ColumnList {
	return table.EmailSendRecords.AllColumns.Except(
		table.EmailSendRecords.CreatedAt, table.EmailSendRecords.UpdatedAt,
	)
}

type sendRecordRepositoryImpl struct {
	emailSendRecordsBase

	mapper *mapper.EmailSendRecordMapper
	logger *slog.Logger
}

func NewSendRecordRepository(
	db *sql.DB, m *mapper.EmailSendRecordMapper, logger *slog.Logger,
) SendRecordRepository {
	return &sendRecordRepositoryImpl{
		emailSendRecordsBase: newEmailSendRecordsBase(db),
		mapper:               m,
		logger:               logger.With(slog.String("component", "email_send_record_repository")),
	}
}

func (r *sendRecordRepositoryImpl) Create(
	ctx context.Context, _ string, _ string, record email.SendRecord,
) (email.SendRecord, error) {
	return r.CreateInternal(ctx, record)
}

func (r *sendRecordRepositoryImpl) CreateInternal(
	ctx context.Context, record email.SendRecord,
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
	return transactor.QueryMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).Value()
}

func (r *sendRecordRepositoryImpl) UpdateStatus(
	ctx context.Context,
	tenantID string,
	id string,
	status email.SendStatus,
	lastError *string,
	sentAt *time.Time,
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
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *sendRecordRepositoryImpl) CountSince(
	ctx context.Context, tenantID string, productID string, since time.Time,
) (int64, error) {
	stmt := postgres.SELECT(postgres.COUNT(postgres.STAR).AS("count_result.count")).
		FROM(table.EmailSendRecords).
		WHERE(
			table.EmailSendRecords.PlatformTenantID.EQ(postgres.String(tenantID)).
				AND(table.EmailSendRecords.ProductID.EQ(postgres.String(productID))).
				AND(table.EmailSendRecords.CreatedAt.GT_EQ(postgres.TimestampzT(since))),
		)
	return transactor.QueryCount(ctx, r.db, stmt).Value()
}
