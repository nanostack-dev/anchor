// Package queueworker runs a pgkit queue worker for the life of the App.
//
// Each anchor worker embeds Worker in its own type, so the generated wiring
// sees one provider per worker and registers it as a nano.Runner: the App
// runs it after every Starter and cancels it when it stops, then waits for
// the jobs in flight to finish, within StopTimeout.
package queueworker

import (
	"context"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/pgkit/queue"
)

// Worker runs one pgkit worker and writes its lifecycle lines.
type Worker struct {
	worker *queue.Worker
	logger *slog.Logger
	spec   Spec
}

// Spec names the lines a worker writes.
type Spec struct {
	// Started is written at Info, with StartedAttrs, when the worker runs.
	Started      string
	StartedAttrs []slog.Attr
	// Failed is written at Error when the worker stops with an error before
	// the App stops.
	Failed string
	// Stopping, when set, is written at Info when the App stops the worker.
	Stopping string
}

// New wraps worker; logger carries the worker's component.
func New(worker *queue.Worker, logger *slog.Logger, spec Spec) Worker {
	return Worker{worker: worker, logger: logger, spec: spec}
}

// Run runs the worker until ctx is canceled. A worker that fails earlier is
// logged and does not stop the App: the API keeps serving, as it did when
// the worker ran in its own goroutine.
func (w *Worker) Run(ctx context.Context) error {
	w.logger.LogAttrs(ctx, slog.LevelInfo, w.spec.Started, w.spec.StartedAttrs...)
	if err := w.worker.Run(ctx); err != nil {
		w.logger.ErrorContext(ctx, w.spec.Failed, log.Err(err))
	}
	<-ctx.Done()
	if w.spec.Stopping != "" {
		w.logger.InfoContext(context.WithoutCancel(ctx), w.spec.Stopping)
	}
	return nil
}
