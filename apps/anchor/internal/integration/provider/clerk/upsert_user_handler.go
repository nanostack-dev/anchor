package clerk

import (
	"context"
	"fmt"
	"time"

	"anchor/internal/domain/integration"
	"anchor/internal/domain/product/user"
	"anchor/internal/events"
	"anchor/internal/integration/provider"

	"github.com/rs/zerolog"
)

func (p *Provider) executeUpsertUser(
	ctx context.Context,
	logger zerolog.Logger,
	instance *integration.Instance,
	data any,
) error {
	upsertData, ok := data.(UpsertUserData)
	if !ok {
		return provider.ErrInvalidCommandData(CommandUpsertUser, "UpsertUserData")
	}

	productUser := user.ProductUser{
		ProductID:  instance.ProductID,
		Email:      upsertData.Email,
		Name:       upsertData.Name,
		ExternalID: &upsertData.ExternalID,
		Status:     user.ProductUserStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	productUser.GenerateID()

	var upserted user.ProductUser
	var created bool
	var unchanged bool
	upsertErr := p.transactor.InTx(ctx, func(txCtx context.Context) error {
		existing, findErr := p.productUserRepo.FindByExternalID(
			txCtx, instance.ProductID, upsertData.ExternalID,
		)
		if findErr != nil {
			return findErr
		}
		if existing.IsPresent() && !clerkUpsertWouldChange(existing.Value(), upsertData) {
			unchanged = true
			return nil
		}
		var err error
		upserted, created, err = p.productUserRepo.UpsertByExternalID(txCtx, productUser)
		if err != nil {
			return err
		}
		eventType := events.ProductUserCreated
		clerkType := events.ClerkUserCreated
		if !created {
			eventType = events.ProductUserUpdated
			clerkType = events.ClerkUserUpdated
		}
		if emitErr := p.events.Emit(txCtx, events.Event{
			Type:      eventType,
			ProductID: instance.ProductID,
			Data:      events.Data{events.FieldProductUserID: upserted.ID},
		}); emitErr != nil {
			return emitErr
		}
		return p.events.Emit(txCtx, events.Event{
			Type:      clerkType,
			ProductID: instance.ProductID,
			Data:      events.Data{events.FieldProductUserID: upserted.ID},
		})
	})
	if upsertErr != nil {
		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeverityError,
			Message:               "Failed to upsert product user from integration event",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				clerkExternalIDKey: upsertData.ExternalID,
			}),
		})

		logger.Error().Err(upsertErr).
			Str(clerkExternalIDKey, upsertData.ExternalID).
			Str("product_id", instance.ProductID).
			Msg("failed to upsert product user")
		return fmt.Errorf("failed to upsert product user: %w", upsertErr)
	}

	if unchanged {
		logger.Debug().
			Str(clerkExternalIDKey, upsertData.ExternalID).
			Str("product_id", instance.ProductID).
			Msg("product user unchanged, skipping event")
		return nil
	}

	if created {
		logger.Info().
			Str("user_id", upserted.ID).
			Str(clerkExternalIDKey, upsertData.ExternalID).
			Str("product_id", instance.ProductID).
			Msg("product user created via integration")

		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeveritySuccess,
			Message:               "Product user created from integration event",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				"user_id":          upserted.ID,
				clerkExternalIDKey: upsertData.ExternalID,
			}),
		})
	} else {
		logger.Debug().
			Str("user_id", upserted.ID).
			Str(clerkExternalIDKey, upsertData.ExternalID).
			Str("product_id", instance.ProductID).
			Msg("product user already exists, skipping audit log")
	}

	return nil
}

func clerkUpsertWouldChange(existing user.ProductUser, data UpsertUserData) bool {
	return existing.Email != data.Email ||
		existing.Name != data.Name ||
		existing.Status != user.ProductUserStatusActive
}
