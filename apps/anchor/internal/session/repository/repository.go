package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/session"
)

var _ Repository = (*repositoryImpl)(nil)

// Repository persists platform user sessions and their refresh tokens. No
// method is tenant scoped: the auth endpoints reach a session through the ids
// a verified token carries, before any tenant context exists.
type Repository interface {
	CreateSession(ctx context.Context, created session.Session) (session.Session, error)
	// LockSession takes a row lock on the session that lasts until the
	// surrounding transaction ends, so two refreshes of one session run one
	// after the other. Call it inside a transaction.
	LockSession(ctx context.Context, sessionID string) (functional.Option[session.Session], error)
	ExtendSession(ctx context.Context, sessionID string, expiresAt time.Time) error
	RevokeSession(ctx context.Context, sessionID string, revokedAt time.Time) error
	DeleteExpiredSessions(ctx context.Context, platformUserID string, now time.Time) error
	CreateRefreshToken(
		ctx context.Context, token session.RefreshToken,
	) (session.RefreshToken, error)
	FindRefreshToken(
		ctx context.Context, sessionID, tokenID string,
	) (functional.Option[session.RefreshToken], error)
	MarkRefreshTokenRotated(ctx context.Context, tokenID string, rotatedAt time.Time) error
	DeleteExpiredRefreshTokens(ctx context.Context, sessionID string, now time.Time) error
}

type repositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repositoryImpl{db: db}
}

func sessionToDomain(entity model.PlatformUserSessions) session.Session {
	return session.Session{
		ID:             entity.ID,
		PlatformUserID: entity.PlatformUserID,
		ExpiresAt:      entity.ExpiresAt,
		RevokedAt:      entity.RevokedAt,
	}
}

func refreshTokenToDomain(entity model.PlatformUserRefreshTokens) session.RefreshToken {
	return session.RefreshToken{
		ID:        entity.ID,
		SessionID: entity.SessionID,
		ExpiresAt: entity.ExpiresAt,
		RotatedAt: entity.RotatedAt,
	}
}

func (r *repositoryImpl) CreateSession(
	ctx context.Context, created session.Session,
) (session.Session, error) {
	entity := model.PlatformUserSessions{
		ID:             created.ID,
		PlatformUserID: created.PlatformUserID,
		ExpiresAt:      created.ExpiresAt,
	}
	stmt := table.PlatformUserSessions.INSERT(
		table.PlatformUserSessions.ID,
		table.PlatformUserSessions.PlatformUserID,
		table.PlatformUserSessions.ExpiresAt,
	).MODEL(entity).RETURNING(table.PlatformUserSessions.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, sessionToDomain).Value()
}

func (r *repositoryImpl) LockSession(
	ctx context.Context, sessionID string,
) (functional.Option[session.Session], error) {
	stmt := table.PlatformUserSessions.SELECT(table.PlatformUserSessions.AllColumns).
		FROM(table.PlatformUserSessions).
		WHERE(table.PlatformUserSessions.ID.EQ(postgres.String(sessionID))).
		FOR(postgres.NO_KEY_UPDATE())
	return transactor.QueryOptionalMap(ctx, r.db, stmt, sessionToDomain)
}

func (r *repositoryImpl) ExtendSession(ctx context.Context, sessionID string, expiresAt time.Time) error {
	stmt := table.PlatformUserSessions.UPDATE(table.PlatformUserSessions.ExpiresAt).
		SET(postgres.TimestampzT(expiresAt)).
		WHERE(table.PlatformUserSessions.ID.EQ(postgres.String(sessionID)))
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *repositoryImpl) RevokeSession(ctx context.Context, sessionID string, revokedAt time.Time) error {
	stmt := table.PlatformUserSessions.UPDATE(table.PlatformUserSessions.RevokedAt).
		SET(postgres.TimestampzT(revokedAt)).
		WHERE(
			table.PlatformUserSessions.ID.EQ(postgres.String(sessionID)).
				AND(table.PlatformUserSessions.RevokedAt.IS_NULL()),
		)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *repositoryImpl) DeleteExpiredSessions(ctx context.Context, platformUserID string, now time.Time) error {
	stmt := table.PlatformUserSessions.DELETE().
		WHERE(
			table.PlatformUserSessions.PlatformUserID.EQ(postgres.String(platformUserID)).
				AND(table.PlatformUserSessions.ExpiresAt.LT(postgres.TimestampzT(now))),
		)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *repositoryImpl) CreateRefreshToken(
	ctx context.Context, token session.RefreshToken,
) (session.RefreshToken, error) {
	entity := model.PlatformUserRefreshTokens{
		ID:        token.ID,
		SessionID: token.SessionID,
		ExpiresAt: token.ExpiresAt,
	}
	stmt := table.PlatformUserRefreshTokens.INSERT(
		table.PlatformUserRefreshTokens.ID,
		table.PlatformUserRefreshTokens.SessionID,
		table.PlatformUserRefreshTokens.ExpiresAt,
	).MODEL(entity).RETURNING(table.PlatformUserRefreshTokens.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, refreshTokenToDomain).Value()
}

func (r *repositoryImpl) FindRefreshToken(
	ctx context.Context, sessionID, tokenID string,
) (functional.Option[session.RefreshToken], error) {
	stmt := table.PlatformUserRefreshTokens.SELECT(table.PlatformUserRefreshTokens.AllColumns).
		FROM(table.PlatformUserRefreshTokens).
		WHERE(
			table.PlatformUserRefreshTokens.ID.EQ(postgres.String(tokenID)).
				AND(table.PlatformUserRefreshTokens.SessionID.EQ(postgres.String(sessionID))),
		)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, refreshTokenToDomain)
}

func (r *repositoryImpl) MarkRefreshTokenRotated(ctx context.Context, tokenID string, rotatedAt time.Time) error {
	stmt := table.PlatformUserRefreshTokens.UPDATE(table.PlatformUserRefreshTokens.RotatedAt).
		SET(postgres.TimestampzT(rotatedAt)).
		WHERE(table.PlatformUserRefreshTokens.ID.EQ(postgres.String(tokenID)))
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *repositoryImpl) DeleteExpiredRefreshTokens(ctx context.Context, sessionID string, now time.Time) error {
	stmt := table.PlatformUserRefreshTokens.DELETE().
		WHERE(
			table.PlatformUserRefreshTokens.SessionID.EQ(postgres.String(sessionID)).
				AND(table.PlatformUserRefreshTokens.ExpiresAt.LT(postgres.TimestampzT(now))),
		)
	return transactor.Exec(ctx, r.db, stmt).Err()
}
