package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/organizationinvitation"
)

var _ SettingsRepository = (*settingsRepositoryImpl)(nil)

// SettingsRepository persists the invitation settings of a Product. A Product
// with no stored settings has no row: the service answers the defaults.
type SettingsRepository interface {
	FindByProductID(
		ctx context.Context, productID string,
	) (functional.Option[organizationinvitation.Settings], error)
	Upsert(
		ctx context.Context, settings organizationinvitation.Settings,
	) (organizationinvitation.Settings, error)
}

type settingsRepositoryImpl struct {
	db     *sql.DB
	logger zerolog.Logger
}

func NewSettingsRepository(db *sql.DB, logger zerolog.Logger) SettingsRepository {
	return &settingsRepositoryImpl{
		db:     db,
		logger: logger.With().Str("component", "organization_invitation_settings_repository").Logger(),
	}
}

func settingsToDomain(entity model.OrganizationInvitationSettings) organizationinvitation.Settings {
	return organizationinvitation.Settings{
		ProductID:          entity.ProductID,
		InvitationDelivery: organizationinvitation.Delivery(entity.InvitationDelivery),
		EmailTemplateID:    entity.EmailTemplateID,
		AcceptURLTemplate:  entity.AcceptURLTemplate,
		DefaultExpiry:      time.Duration(entity.DefaultExpirySeconds) * time.Second,
	}
}

func (r *settingsRepositoryImpl) FindByProductID(
	ctx context.Context, productID string,
) (functional.Option[organizationinvitation.Settings], error) {
	stmt := table.OrganizationInvitationSettings.SELECT(table.OrganizationInvitationSettings.AllColumns).
		FROM(table.OrganizationInvitationSettings).
		WHERE(table.OrganizationInvitationSettings.ProductID.EQ(postgres.String(productID))).
		LIMIT(1)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, settingsToDomain)
}

func (r *settingsRepositoryImpl) Upsert(
	ctx context.Context, settings organizationinvitation.Settings,
) (organizationinvitation.Settings, error) {
	columns := table.OrganizationInvitationSettings
	entity := model.OrganizationInvitationSettings{
		ProductID:            settings.ProductID,
		InvitationDelivery:   string(settings.InvitationDelivery),
		EmailTemplateID:      settings.EmailTemplateID,
		AcceptURLTemplate:    settings.AcceptURLTemplate,
		DefaultExpirySeconds: int64(settings.DefaultExpiry / time.Second),
	}
	stmt := columns.INSERT(
		columns.ProductID,
		columns.InvitationDelivery,
		columns.EmailTemplateID,
		columns.AcceptURLTemplate,
		columns.DefaultExpirySeconds,
	).MODEL(entity).
		ON_CONFLICT(columns.ProductID).
		DO_UPDATE(postgres.SET(
			columns.InvitationDelivery.SET(columns.EXCLUDED.InvitationDelivery),
			columns.EmailTemplateID.SET(columns.EXCLUDED.EmailTemplateID),
			columns.AcceptURLTemplate.SET(columns.EXCLUDED.AcceptURLTemplate),
			columns.DefaultExpirySeconds.SET(columns.EXCLUDED.DefaultExpirySeconds),
		)).
		RETURNING(columns.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, settingsToDomain).Value()
}
