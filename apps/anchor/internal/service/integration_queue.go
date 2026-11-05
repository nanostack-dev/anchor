package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/nanostack-dev/pgkit/queue"
	"go.uber.org/fx"

	"github.com/rs/zerolog"
)

var queueRetention = queue.RetentionPolicy{
	KeepDone:   7 * 24 * time.Hour,
	KeepFailed: 30 * 24 * time.Hour,
}

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

// RegisterQueueRetention deletes finished pgqueue jobs of every Anchor queue
// once they are older than queueRetention keeps them.
func RegisterQueueRetention(lc fx.Lifecycle, queueClient *queue.Client, logger zerolog.Logger) {
	retentionCtx, stopRetention := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				defer close(stopped)
				if err := queueClient.RunRetention(retentionCtx, queueRetention); err != nil {
					logger.Error().Err(err).Msg("pgqueue retention stopped with error")
				}
			}()
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			stopRetention()
			select {
			case <-stopped:
				return nil
			case <-stopCtx.Done():
				return stopCtx.Err()
			}
		},
	})
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
