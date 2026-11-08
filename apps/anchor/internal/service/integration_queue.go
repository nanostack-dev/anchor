package service

import (
	"context"
	"database/sql"
	"log/slog"
	"maps"
	"slices"
	"time"

	"anchor/internal/applog"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/nanostack-dev/pgkit/queue"
	"go.uber.org/fx"
)

const (
	queueKeepDoneJobs   = 7 * 24 * time.Hour
	queueKeepFailedJobs = 30 * 24 * time.Hour
)

// NewIntegrationLock creates the pglock client used for distributed advisory
// locking (e.g. to ensure only one replica seeds the reconcile scheduler job).
func NewIntegrationLock(db *sql.DB) (*pglock.Client, error) {
	return pglock.New(db)
}

func NewIntegrationQueue(
	db *sql.DB,
	logger *slog.Logger,
) (*queue.Client, error) {
	queueClient, err := queue.New(db)
	if err != nil {
		return nil, err
	}

	queueClient.SetLogger(slogQueueAdapter{logger: logger.With(slog.String("component", "pgqueue"))})

	if schemaErr := queueClient.EnsureSchema(context.Background()); schemaErr != nil {
		return nil, schemaErr
	}

	return queueClient, nil
}

// RegisterQueueRetention deletes finished pgqueue jobs of every Anchor queue
// once they are older than queueKeepDoneJobs or queueKeepFailedJobs.
func RegisterQueueRetention(lc fx.Lifecycle, queueClient *queue.Client, logger *slog.Logger) {
	retentionCtx, stopRetention := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				defer close(stopped)
				if err := queueClient.RunRetention(retentionCtx, queue.RetentionPolicy{
					KeepDone:   queueKeepDoneJobs,
					KeepFailed: queueKeepFailedJobs,
				}); err != nil {
					logger.Error("pgqueue retention stopped with error", log.Err(err))
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

// slogQueueAdapter writes pgkit's queue lines through the component logger.
type slogQueueAdapter struct {
	logger *slog.Logger
}

func (a slogQueueAdapter) Debug(ctx context.Context, msg string, fields map[string]any) {
	a.log(ctx, slog.LevelDebug, msg, fields)
}

func (a slogQueueAdapter) Info(ctx context.Context, msg string, fields map[string]any) {
	a.log(ctx, slog.LevelInfo, msg, fields)
}

func (a slogQueueAdapter) Warn(ctx context.Context, msg string, fields map[string]any) {
	a.log(ctx, slog.LevelWarn, msg, fields)
}

func (a slogQueueAdapter) Error(ctx context.Context, msg string, fields map[string]any) {
	a.log(ctx, slog.LevelError, msg, fields)
}

// log writes fields as top-level attributes in key order, as zerolog's
// Fields did, with pgkit's call site as the caller.
func (a slogQueueAdapter) log(ctx context.Context, level slog.Level, msg string, fields map[string]any) {
	attrs := make([]slog.Attr, 0, len(fields))
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		attrs = append(attrs, slog.Any(key, fields[key]))
	}
	applog.Write(ctx, a.logger, 2, level, msg, attrs...) //nolint:mnd // log and the adapter method
}
