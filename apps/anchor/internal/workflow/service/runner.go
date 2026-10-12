package service

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"

	"anchor/internal/domain/workflow"
	"anchor/internal/workflow/engine"
	"anchor/internal/workflow/repository"
)

// Runner executes a workflow and keeps its run: stored as running before the
// first step, and finished with every step's result.
type Runner struct {
	repo   repository.Repository
	engine *engine.Engine
	logger zerolog.Logger
}

func NewRunner(repo repository.Repository, eng *engine.Engine, logger zerolog.Logger) *Runner {
	return &Runner{repo: repo, engine: eng, logger: logger.With().Str("component", "workflow_runner").Logger()}
}

// Prevent records, without running a step, a run the loop guard refused.
func (r *Runner) Prevent(ctx context.Context, execution engine.Execution, reason string) error {
	now := time.Now().UTC()
	prevented := workflow.Run{
		WorkflowID: execution.Workflow.ID,
		ProductID:  execution.Workflow.ProductID,
		EventID:    execution.EventID,
		EventType:  execution.EventType,
		EventData:  execution.EventData,
		Trigger:    execution.Trigger,
		Status:     workflow.RunStatusSkipped,
		Steps:      []workflow.StepResult{},
		Error:      &reason,
		StartedAt:  now,
		FinishedAt: &now,
	}
	prevented.GenerateID()
	started, err := r.repo.StartRun(ctx, prevented)
	if err != nil || !started {
		return err
	}
	r.logger.Warn().
		Str("product_id", prevented.ProductID).
		Str("workflow_id", prevented.WorkflowID).
		Str("event_id", prevented.EventID).
		Msg("workflow run prevented by the loop guard")
	return nil
}

// Start reports false, with no error, when the workflow already ran for the
// event. A run starts at most once per event: a step that wrote is never
// repeated by a redelivery.
func (r *Runner) Start(ctx context.Context, execution engine.Execution) (workflow.Run, bool, error) {
	pending := workflow.Run{
		WorkflowID: execution.Workflow.ID,
		ProductID:  execution.Workflow.ProductID,
		EventID:    execution.EventID,
		EventType:  execution.EventType,
		EventData:  execution.EventData,
		Trigger:    execution.Trigger,
		Status:     workflow.RunStatusRunning,
		Steps:      []workflow.StepResult{},
	}
	pending.GenerateID()
	pending.StartedAt = time.Now().UTC()
	started, err := r.repo.StartRun(ctx, pending)
	if err != nil {
		return workflow.Run{}, false, err
	}
	if !started {
		return workflow.Run{}, false, nil
	}

	execution.RunID = pending.ID
	run := r.engine.Execute(ctx, execution)
	run.StartedAt = pending.StartedAt
	run.WorkflowName = execution.Workflow.Name
	if finishErr := r.repo.FinishRun(context.WithoutCancel(ctx), run); finishErr != nil {
		return run, true, errors.Join(errFinishRun, finishErr)
	}
	r.logger.Info().
		Str("product_id", run.ProductID).
		Str("workflow_id", run.WorkflowID).
		Str("run_id", run.ID).
		Str("status", string(run.Status)).
		Msg("workflow run finished")
	return run, true, nil
}
