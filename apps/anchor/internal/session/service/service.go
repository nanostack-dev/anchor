package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/session"
	"anchor/internal/session/repository"
)

var (
	ErrRefreshRefused  = errors.New("refresh token refused")
	ErrRefreshReplayed = errors.New("rotated refresh token replayed; session revoked")
)

type StartInput struct {
	PlatformUserID string `validate:"required"`
	FirstToken     session.RefreshToken
}

type RotateInput struct {
	PresentedTokenID string `validate:"required"`
	Next             session.RefreshToken
}

type RevokeInput struct {
	SessionID string `validate:"required"`
}

type Service interface {
	Start(ctx context.Context, input StartInput) error
	// Rotate returns ErrRefreshRefused for an unknown, expired or revoked
	// token, and ErrRefreshReplayed after revoking the session.
	Rotate(ctx context.Context, input RotateInput) error
	Revoke(ctx context.Context, input RevokeInput) error
}

type sessionService struct {
	repo       repository.Repository
	transactor transactor.Transactor
	logger     zerolog.Logger
}

func NewService(repo repository.Repository, tx transactor.Transactor, logger zerolog.Logger) Service {
	return &sessionService{
		repo:       repo,
		transactor: tx,
		logger:     logger.With().Str("component", "session_service").Logger(),
	}
}

func (s *sessionService) Start(ctx context.Context, input StartInput) error {
	if validationErr := validate.ValidateStruct(input); validationErr != nil {
		return validationErr
	}

	opened := session.Session{
		ID:             input.FirstToken.SessionID,
		PlatformUserID: input.PlatformUserID,
		ExpiresAt:      input.FirstToken.ExpiresAt,
	}
	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.DeleteExpiredSessions(txCtx, input.PlatformUserID, time.Now()); err != nil {
			return fmt.Errorf("failed to prune expired sessions: %w", err)
		}
		if _, err := s.repo.CreateSession(txCtx, opened); err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
		if _, err := s.repo.CreateRefreshToken(txCtx, input.FirstToken); err != nil {
			return fmt.Errorf("failed to store refresh token: %w", err)
		}
		return nil
	})
}

func (s *sessionService) Rotate(ctx context.Context, input RotateInput) error {
	if validationErr := validate.ValidateStruct(input); validationErr != nil {
		return validationErr
	}

	replayed := false
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		var exchangeErr error
		replayed, exchangeErr = s.exchange(txCtx, input, time.Now())
		return exchangeErr
	})
	if err != nil {
		return err
	}
	if replayed {
		s.logger.Warn().
			Str("session_id", input.Next.SessionID).
			Msg("rotated refresh token presented again; session revoked")
		return ErrRefreshReplayed
	}
	return nil
}

// exchange reports a replay with a nil error, so the transaction commits the
// revocation before Rotate refuses the refresh.
func (s *sessionService) exchange(ctx context.Context, input RotateInput, now time.Time) (bool, error) {
	sessionID := input.Next.SessionID
	foundSession, err := s.repo.LockSession(ctx, sessionID)
	if err != nil {
		return false, fmt.Errorf("failed to lock session: %w", err)
	}
	if foundSession.IsAbsent() {
		return false, ErrRefreshRefused
	}
	locked := foundSession.Value()
	if locked.IsRevoked() {
		return false, ErrRefreshRefused
	}

	foundToken, err := s.repo.FindRefreshToken(ctx, sessionID, input.PresentedTokenID)
	if err != nil {
		return false, fmt.Errorf("failed to find refresh token: %w", err)
	}
	if foundToken.IsAbsent() {
		return false, ErrRefreshRefused
	}
	presented := foundToken.Value()

	presentation := presented.PresentedAt(now)
	if presentation == session.Replay {
		return true, s.repo.RevokeSession(ctx, sessionID, now)
	}
	if presentation == session.FirstUse {
		if rotateErr := s.repo.MarkRefreshTokenRotated(ctx, presented.ID, now); rotateErr != nil {
			return false, fmt.Errorf("failed to rotate refresh token: %w", rotateErr)
		}
	}

	if pruneErr := s.repo.DeleteExpiredRefreshTokens(ctx, sessionID, now); pruneErr != nil {
		return false, fmt.Errorf("failed to prune expired refresh tokens: %w", pruneErr)
	}
	if _, storeErr := s.repo.CreateRefreshToken(ctx, input.Next); storeErr != nil {
		return false, fmt.Errorf("failed to store refresh token: %w", storeErr)
	}
	if extendErr := s.repo.ExtendSession(ctx, sessionID, input.Next.ExpiresAt); extendErr != nil {
		return false, fmt.Errorf("failed to extend session: %w", extendErr)
	}
	return false, nil
}

func (s *sessionService) Revoke(ctx context.Context, input RevokeInput) error {
	if validationErr := validate.ValidateStruct(input); validationErr != nil {
		return validationErr
	}
	return s.repo.RevokeSession(ctx, input.SessionID, time.Now())
}
