package clerk

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/integration"
	"anchor/internal/domain/product/user"
	"anchor/internal/events"
	"anchor/internal/integration/provider"
	"anchor/internal/repository"
)

type upsertOutcome string

const (
	upsertOutcomeCreated              upsertOutcome = "created"
	upsertOutcomeUpdated              upsertOutcome = "updated"
	upsertOutcomeAdopted              upsertOutcome = "adopted"
	upsertOutcomeUnchanged            upsertOutcome = "unchanged"
	upsertOutcomeSkippedTombstoned    upsertOutcome = "tombstoned"
	upsertOutcomeSkippedStale         upsertOutcome = "stale"
	upsertOutcomeSkippedEmailConflict upsertOutcome = "email_conflict"
	upsertOutcomeSkippedUnverified    upsertOutcome = "unverified_adoption_blocked"

	clerkAuditUserIDKey = "user_id"
	clerkAuditReasonKey = "reason"
)

type upsertResult struct {
	outcome  upsertOutcome
	user     user.ProductUser
	holderID string
}

func (p *Provider) executeUpsertUser(
	ctx context.Context,
	logger *slog.Logger,
	instance *integration.Instance,
	data any,
) error {
	upsertData, ok := data.(UpsertUserData)
	if !ok {
		return provider.ErrInvalidCommandData(CommandUpsertUser, "UpsertUserData")
	}

	var res upsertResult
	upsertErr := p.transactor.InTx(ctx, func(txCtx context.Context) error {
		var err error
		res, err = p.applyUpsertInTx(txCtx, instance.ProductID, upsertData)
		return err
	})
	if upsertErr != nil {
		logger.ErrorContext(ctx, "failed to upsert product user",
			log.Err(upsertErr),
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)
		meta := map[string]any{
			clerkExternalIDKey: upsertData.ExternalID,
		}
		if conflictErr, isConflict := errors.AsType[*repository.ProductUserEmailConflictError](upsertErr); isConflict {
			meta[clerkAuditReasonKey] = "email_conflict"
			meta["holder_user_id"] = conflictErr.HolderID
		}
		// The failed write aborted the caller's transaction: the caller writes
		// this audit row after the rollback.
		return provider.WithFailureAudit(
			fmt.Errorf("failed to upsert product user: %w", upsertErr),
			integration.AuditLog{
				IntegrationInstanceID: instance.ID,
				Action:                integration.AuditActionUpsertUser,
				Severity:              integration.AuditSeverityError,
				Message:               "Failed to upsert product user from integration event",
				MetadataJSON:          provider.MustMarshalJSON(meta),
			},
		)
	}

	p.recordUpsertOutcome(ctx, logger, instance, upsertData, res)
	return nil
}

func (p *Provider) applyUpsertInTx(
	txCtx context.Context,
	productID string,
	upsertData UpsertUserData,
) (upsertResult, error) {
	if lockErr := p.productUserRepo.LockClerkUser(txCtx, productID, upsertData.ExternalID); lockErr != nil {
		return upsertResult{}, lockErr
	}

	tombstone, tombErr := p.productUserRepo.FindTombstone(txCtx, productID, upsertData.ExternalID)
	if tombErr != nil {
		return upsertResult{}, tombErr
	}
	if deletedAt, deleted := tombstone.Get(); deleted && isTombstoned(upsertData.EventAt, deletedAt) {
		return upsertResult{outcome: upsertOutcomeSkippedTombstoned}, nil
	}

	// The row lock holds the status read here until the upsert writes it
	// back, so the status this event keeps is the one stored at commit.
	existing, findErr := p.productUserRepo.FindByExternalID(
		transactor.ForUpdate(txCtx), productID, upsertData.ExternalID,
	)
	if findErr != nil {
		return upsertResult{}, findErr
	}

	if existing.IsPresent() {
		return p.applyUpdateExistingInTx(txCtx, productID, existing.Value(), upsertData)
	}
	return p.applyInsertOrAdoptInTx(txCtx, productID, upsertData)
}

func (p *Provider) applyUpdateExistingInTx(
	txCtx context.Context,
	productID string,
	existing user.ProductUser,
	upsertData UpsertUserData,
) (upsertResult, error) {
	if isStaleEvent(upsertData.EventAt, existing.LastSyncedEventAt) {
		return upsertResult{outcome: upsertOutcomeSkippedStale, user: existing}, nil
	}

	if !clerkUpsertWouldChange(existing, upsertData) {
		if upsertData.EventAt != nil &&
			(existing.LastSyncedEventAt == nil || upsertData.EventAt.After(*existing.LastSyncedEventAt)) {
			if advErr := p.productUserRepo.AdvanceLastSyncedEventAt(
				txCtx, productID, existing.ID, *upsertData.EventAt,
			); advErr != nil {
				return upsertResult{}, advErr
			}
		}
		return upsertResult{outcome: upsertOutcomeUnchanged, user: existing}, nil
	}

	if existing.Email != upsertData.Email {
		holderOpt, emailErr := p.productUserRepo.FindByProductAndEmail(
			transactor.ForUpdate(txCtx), productID, upsertData.Email,
		)
		if emailErr != nil {
			return upsertResult{}, emailErr
		}
		if holder, taken := holderOpt.Get(); taken && holder.ID != existing.ID {
			return handleEmailConflict(upsertData, holder)
		}
	}

	updated, err := p.productUserRepo.UpdateSynced(
		txCtx,
		productID,
		existing.ID,
		upsertData.Email,
		upsertData.Name,
		clerkUpsertStatus(functional.Some(existing)),
		upsertData.EventAt,
	)
	if err != nil {
		return upsertResult{}, err
	}
	if emitErr := p.emitUserSyncEvents(
		txCtx,
		productID,
		updated.ID,
		events.ProductUserUpdated,
		events.ClerkUserUpdated,
	); emitErr != nil {
		return upsertResult{}, emitErr
	}
	return upsertResult{outcome: upsertOutcomeUpdated, user: updated}, nil
}

func (p *Provider) applyInsertOrAdoptInTx(
	txCtx context.Context,
	productID string,
	upsertData UpsertUserData,
) (upsertResult, error) {
	holderOpt, emailErr := p.productUserRepo.FindByProductAndEmail(
		transactor.ForUpdate(txCtx), productID, upsertData.Email,
	)
	if emailErr != nil {
		return upsertResult{}, emailErr
	}
	if holder, exists := holderOpt.Get(); exists {
		return p.adoptOrConflictInTx(txCtx, productID, holder, upsertData)
	}

	productUser := user.ProductUser{
		ProductID:         productID,
		Email:             upsertData.Email,
		Name:              upsertData.Name,
		ExternalID:        &upsertData.ExternalID,
		Status:            clerkUpsertStatus(functional.None[user.ProductUser]()),
		LastSyncedEventAt: upsertData.EventAt,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	productUser.GenerateID()

	insertedOpt, insertErr := p.productUserRepo.InsertIfAbsent(txCtx, productUser)
	if insertErr != nil {
		return upsertResult{}, insertErr
	}
	inserted, ok := insertedOpt.Get()
	if !ok {
		if upsertData.Source == SyncSourceReconcile {
			return upsertResult{outcome: upsertOutcomeSkippedEmailConflict}, nil
		}
		return upsertResult{}, errors.New("product user insert raced with concurrent write")
	}
	if emitErr := p.emitUserSyncEvents(
		txCtx, productID, inserted.ID, events.ProductUserCreated, events.ClerkUserCreated,
	); emitErr != nil {
		return upsertResult{}, emitErr
	}
	return upsertResult{outcome: upsertOutcomeCreated, user: inserted}, nil
}

func (p *Provider) adoptOrConflictInTx(
	txCtx context.Context,
	productID string,
	holder user.ProductUser,
	upsertData UpsertUserData,
) (upsertResult, error) {
	if holder.ExternalID != nil {
		return handleEmailConflict(upsertData, holder)
	}
	if !upsertData.EmailVerified {
		return upsertResult{
			outcome:  upsertOutcomeSkippedUnverified,
			holderID: holder.ID,
		}, nil
	}
	adoptedOpt, adoptErr := p.productUserRepo.AdoptExternalID(
		txCtx,
		productID,
		holder.ID,
		upsertData.ExternalID,
		upsertData.Name,
		clerkUpsertStatus(functional.Some(holder)),
		upsertData.EventAt,
	)
	if adoptErr != nil {
		return upsertResult{}, adoptErr
	}
	adopted, ok := adoptedOpt.Get()
	if !ok {
		return upsertResult{}, errors.New("product user adoption raced with concurrent write")
	}
	if emitErr := p.emitUserSyncEvents(
		txCtx, productID, adopted.ID, events.ProductUserUpdated, events.ClerkUserCreated,
	); emitErr != nil {
		return upsertResult{}, emitErr
	}
	return upsertResult{outcome: upsertOutcomeAdopted, user: adopted}, nil
}

func handleEmailConflict(upsertData UpsertUserData, holder user.ProductUser) (upsertResult, error) {
	if upsertData.Source == SyncSourceReconcile {
		return upsertResult{
			outcome:  upsertOutcomeSkippedEmailConflict,
			holderID: holder.ID,
		}, nil
	}
	return upsertResult{}, &repository.ProductUserEmailConflictError{
		Email:    upsertData.Email,
		HolderID: holder.ID,
	}
}

func (p *Provider) emitUserSyncEvents(
	txCtx context.Context,
	productID, userID string,
	productEventType, clerkEventType events.Type,
) error {
	if emitErr := p.events.Emit(txCtx, events.Event{
		Type:      productEventType,
		ProductID: productID,
		Data:      events.Data{events.FieldProductUserID: userID},
	}); emitErr != nil {
		return emitErr
	}
	return p.events.Emit(txCtx, events.Event{
		Type:      clerkEventType,
		ProductID: productID,
		Data:      events.Data{events.FieldProductUserID: userID},
	})
}

func (p *Provider) recordUpsertOutcome(
	ctx context.Context,
	logger *slog.Logger,
	instance *integration.Instance,
	upsertData UpsertUserData,
	res upsertResult,
) {
	switch res.outcome {
	case upsertOutcomeCreated:
		logger.InfoContext(ctx, "product user created via integration",
			slog.String(clerkAuditUserIDKey, res.user.ID),
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)
		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeveritySuccess,
			Message:               "Product user created from integration event",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				clerkAuditUserIDKey: res.user.ID,
				clerkExternalIDKey:  upsertData.ExternalID,
			}),
		})
	case upsertOutcomeAdopted:
		logger.InfoContext(ctx, "existing product user adopted by Clerk external ID",
			slog.String(clerkAuditUserIDKey, res.user.ID),
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)
		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeveritySuccess,
			Message:               "Existing product user adopted by Clerk external ID",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				clerkAuditUserIDKey: res.user.ID,
				clerkExternalIDKey:  upsertData.ExternalID,
				"adopted":           true,
			}),
		})
	case upsertOutcomeUpdated:
		logger.DebugContext(ctx, "product user updated via integration",
			slog.String(clerkAuditUserIDKey, res.user.ID),
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)
	case upsertOutcomeUnchanged:
		logger.DebugContext(ctx, "product user unchanged, skipping event",
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)
	case upsertOutcomeSkippedTombstoned:
		logger.InfoContext(ctx, "skipped Clerk upsert for tombstoned user",
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)
		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeverityInfo,
			Message:               "Skipped stale upsert for deleted Clerk user",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				clerkExternalIDKey:  upsertData.ExternalID,
				clerkAuditReasonKey: string(res.outcome),
			}),
		})
	case upsertOutcomeSkippedStale:
		logger.InfoContext(ctx, "skipped stale Clerk upsert older than last synced event",
			slog.String(clerkAuditUserIDKey, res.user.ID),
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("product_id", instance.ProductID),
		)
		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeverityInfo,
			Message:               "Skipped stale Clerk upsert older than last synced event",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				clerkAuditUserIDKey: res.user.ID,
				clerkExternalIDKey:  upsertData.ExternalID,
				clerkAuditReasonKey: string(res.outcome),
			}),
		})
	case upsertOutcomeSkippedEmailConflict:
		logger.WarnContext(ctx, "skipped Clerk reconcile upsert due to email conflict with another external ID",
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("holder_user_id", res.holderID),
			slog.String("product_id", instance.ProductID),
		)
		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeverityWarning,
			Message:               "Skipped Clerk user during reconcile because email belongs to another user",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				clerkExternalIDKey:  upsertData.ExternalID,
				"holder_user_id":    res.holderID,
				clerkAuditReasonKey: string(res.outcome),
			}),
		})
	case upsertOutcomeSkippedUnverified:
		logger.WarnContext(ctx, "skipped adopting existing product user because Clerk email is not verified",
			slog.String(clerkExternalIDKey, upsertData.ExternalID),
			slog.String("holder_user_id", res.holderID),
			slog.String("product_id", instance.ProductID),
		)
		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionUpsertUser,
			Severity:              integration.AuditSeverityWarning,
			Message:               "Skipped linking Clerk user to existing email row because email is not verified",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				clerkExternalIDKey:  upsertData.ExternalID,
				"holder_user_id":    res.holderID,
				clerkAuditReasonKey: string(res.outcome),
			}),
		})
	}
}

func isTombstoned(eventAt *time.Time, deletedAt time.Time) bool {
	if eventAt == nil {
		return true
	}
	return !eventAt.After(deletedAt)
}

func isStaleEvent(eventAt, lastSyncedAt *time.Time) bool {
	if eventAt == nil || lastSyncedAt == nil {
		return false
	}
	return eventAt.Before(*lastSyncedAt)
}

// clerkUpsertStatus is the status a Clerk event writes: Clerk has no notion of
// an Anchor suspension, so a suspended user stays suspended; any other user,
// a new one included, becomes active.
func clerkUpsertStatus(existing functional.Option[user.ProductUser]) user.ProductUserStatus {
	if found, ok := existing.Get(); ok && found.Status == user.ProductUserStatusSuspended {
		return user.ProductUserStatusSuspended
	}
	return user.ProductUserStatusActive
}

func clerkUpsertWouldChange(existing user.ProductUser, data UpsertUserData) bool {
	return existing.Email != data.Email ||
		existing.Name != data.Name ||
		existing.Status != clerkUpsertStatus(functional.Some(existing))
}
