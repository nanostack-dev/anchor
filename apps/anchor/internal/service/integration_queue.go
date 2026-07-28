package service

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/nanostack-dev/pgkit/queue"

	"github.com/rs/zerolog"
)

// NewIntegrationLock creates the pglock client used for distributed advisory
// locking (e.g. to ensure only one replica seeds the reconcile scheduler job).
func NewIntegrationLock(db *sql.DB) (*pglock.Client, error) {
	return pglock.New(db)
}

func NewIntegrationQueue(
	db *sql.DB,
	logger zerolog.Logger,
) (*queue.Client, error) {
	queueClient, err := queue.New(db)
	if err != nil {
		return nil, err
	}

	queueClient.SetLogger(zerologQueueAdapter{logger: logger.With().Str("component", "pgqueue").Logger()})

	if schemaErr := queueClient.EnsureSchema(context.Background()); schemaErr != nil {
		return nil, schemaErr
	}

	return queueClient, nil
}

type zerologQueueAdapter struct {
	logger zerolog.Logger
}

func (z zerologQueueAdapter) Debug(_ context.Context, msg string, fields map[string]any) {
	z.logger.Debug().Fields(fields).Msg(msg)
}

func (z zerologQueueAdapter) Info(_ context.Context, msg string, fields map[string]any) {
	z.logger.Info().Fields(fields).Msg(msg)
}

func (z zerologQueueAdapter) Warn(_ context.Context, msg string, fields map[string]any) {
	z.logger.Warn().Fields(fields).Msg(msg)
}

func (z zerologQueueAdapter) Error(_ context.Context, msg string, fields map[string]any) {
	z.logger.Error().Fields(fields).Msg(msg)
}
