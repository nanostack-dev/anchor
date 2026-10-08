package service

import (
	"context"
	"slices"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
	"anchor/internal/workflow/engine"
	"anchor/internal/workflow/repository"
)

const (
	defaultRunListLimit = 25
	manualEventIDPrefix = "wfevt"
)

// Catalog is what a workflow can be built from.
type Catalog struct {
	Triggers  []engine.Trigger
	Actions   []engine.ActionSpec
	Operators []workflow.Operator
}

type WorkflowService interface {
	Catalog(ctx context.Context, input workflow.ListInput) (Catalog, error)
	Create(ctx context.Context, input workflow.CreateInput) (workflow.Workflow, error)
	Get(ctx context.Context, input workflow.GetInput) (workflow.Workflow, error)
	List(ctx context.Context, input workflow.ListInput) ([]workflow.Workflow, error)
	Update(ctx context.Context, input workflow.UpdateInput) (workflow.Workflow, error)
	Delete(ctx context.Context, input workflow.DeleteInput) error
	ListRuns(ctx context.Context, input workflow.ListRunsInput) ([]workflow.Run, error)
	ListProductRuns(ctx context.Context, input workflow.ListProductRunsInput) ([]workflow.Run, error)
	Run(ctx context.Context, input workflow.RunInput) (workflow.Run, error)
	DryRun(ctx context.Context, input workflow.DryRunInput) (workflow.Run, error)
	Emits(wf workflow.Workflow) []string
}

type workflowService struct {
	repo   repository.Repository
	engine *engine.Engine
	runner *Runner
	logger zerolog.Logger
}

func NewWorkflowService(
	repo repository.Repository, eng *engine.Engine, runner *Runner, logger zerolog.Logger,
) WorkflowService {
	return &workflowService{
		repo:   repo,
		engine: eng,
		runner: runner,
		logger: logger.With().Str("component", "workflow_service").Logger(),
	}
}

func (s *workflowService) Catalog(ctx context.Context, input workflow.ListInput) (Catalog, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return Catalog{}, err
	}
	workflows, err := s.repo.List(ctx, input.TenantID, input.ProductID)
	if err != nil {
		return Catalog{}, err
	}
	return Catalog{
		Triggers:  s.engine.Triggers(workflows),
		Actions:   s.engine.Actions(),
		Operators: workflow.AllOperators(),
	}, nil
}

// checkChains refuses a workflow whose steps send a custom event field with
// a type another step of the product already gives it differently, and an
// enabled workflow that would start itself again through its own writes or
// through other enabled workflows of the product. Both read the product's
// workflows once.
func (s *workflowService) checkChains(ctx context.Context, candidate workflow.Workflow) error {
	sends := slices.ContainsFunc(candidate.Definition.Steps, func(step workflow.Step) bool {
		return step.Action == engine.ActionWorkflowEmit
	})
	if !sends && !candidate.Enabled {
		return nil
	}
	others, err := s.repo.List(ctx, candidate.PlatformTenantID, candidate.ProductID)
	if err != nil {
		return err
	}
	if conflict := s.engine.FieldTypeConflict(candidate, others); sends && conflict != nil {
		return fieldTypeConflictError(*conflict)
	}
	if path := s.engine.FindLoop(candidate, others); candidate.Enabled && path != nil {
		return loopError(path)
	}
	return nil
}

func (s *workflowService) Create(ctx context.Context, input workflow.CreateInput) (workflow.Workflow, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return workflow.Workflow{}, err
	}
	if err := s.engine.Validate(input.TriggerEventType, input.Definition); err != nil {
		return workflow.Workflow{}, err
	}
	wf := fromWriteInput(input.TenantID, input.ProductID, input.WriteInput)
	wf.GenerateID()
	wf.CreatedAt = time.Now().UTC()
	if err := s.checkChains(ctx, wf); err != nil {
		return workflow.Workflow{}, err
	}
	created, err := s.repo.Create(ctx, wf)
	if err != nil {
		return workflow.Workflow{}, err
	}
	s.logger.Info().Str("product_id", input.ProductID).Str("workflow_id", created.ID).Msg("workflow created")
	return created, nil
}

func (s *workflowService) Get(ctx context.Context, input workflow.GetInput) (workflow.Workflow, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return workflow.Workflow{}, err
	}
	return s.find(ctx, input.TenantID, input.ProductID, input.WorkflowID)
}

func (s *workflowService) List(ctx context.Context, input workflow.ListInput) ([]workflow.Workflow, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return nil, err
	}
	workflows, err := s.repo.List(ctx, input.TenantID, input.ProductID)
	if err != nil || !slices.Contains(input.Include, workflow.IncludeLastRun) {
		return workflows, err
	}
	latest, err := s.repo.LatestRuns(ctx, input.TenantID, input.ProductID)
	if err != nil {
		return nil, err
	}
	for index := range workflows {
		if run, ran := latest[workflows[index].ID]; ran {
			workflows[index].LastRun = &run
		}
	}
	return workflows, nil
}

// Emits lists the event types a workflow's steps can emit.
func (s *workflowService) Emits(wf workflow.Workflow) []string {
	return s.engine.Emits(wf)
}

func (s *workflowService) Update(ctx context.Context, input workflow.UpdateInput) (workflow.Workflow, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return workflow.Workflow{}, err
	}
	if err := s.engine.Validate(input.TriggerEventType, input.Definition); err != nil {
		return workflow.Workflow{}, err
	}
	wf := fromWriteInput(input.TenantID, input.ProductID, input.WriteInput)
	wf.ID = input.WorkflowID
	if err := s.checkChains(ctx, wf); err != nil {
		return workflow.Workflow{}, err
	}
	updated, err := s.repo.Update(ctx, wf)
	if err != nil {
		return workflow.Workflow{}, err
	}
	if updated.IsAbsent() {
		return workflow.Workflow{}, errWorkflowNotFound
	}
	return updated.Value(), nil
}

func (s *workflowService) Delete(ctx context.Context, input workflow.DeleteInput) error {
	if err := validate.ValidateStruct(input); err != nil {
		return err
	}
	deleted, err := s.repo.Delete(ctx, input.TenantID, input.ProductID, input.WorkflowID)
	if err != nil {
		return err
	}
	if !deleted {
		return errWorkflowNotFound
	}
	return nil
}

func (s *workflowService) ListRuns(ctx context.Context, input workflow.ListRunsInput) ([]workflow.Run, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return nil, err
	}
	if _, err := s.find(ctx, input.TenantID, input.ProductID, input.WorkflowID); err != nil {
		return nil, err
	}
	return s.repo.ListRuns(
		ctx,
		input.TenantID,
		input.ProductID,
		functional.Some(input.WorkflowID),
		runLimit(input.Limit),
	)
}

func (s *workflowService) ListProductRuns(
	ctx context.Context, input workflow.ListProductRunsInput,
) ([]workflow.Run, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return nil, err
	}
	return s.repo.ListRuns(ctx, input.TenantID, input.ProductID, functional.None[string](), runLimit(input.Limit))
}

func (s *workflowService) Run(ctx context.Context, input workflow.RunInput) (workflow.Run, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return workflow.Run{}, err
	}
	wf, err := s.find(ctx, input.TenantID, input.ProductID, input.WorkflowID)
	if err != nil {
		return workflow.Run{}, err
	}
	causation := events.CausationFrom(ctx)
	if slices.Contains(causation.WorkflowIDs, wf.ID) {
		return workflow.Run{}, errRunWouldLoop
	}
	if causation.Depth >= workflow.MaxCausationDepth {
		return workflow.Run{}, errChainTooDeep
	}
	run, _, err := s.runner.Start(ctx, engine.Execution{
		Workflow:  wf,
		EventID:   ids.MustNew(manualEventIDPrefix),
		EventType: wf.TriggerEventType,
		EventData: nonNil(input.EventData),
		Trigger:   workflow.RunTriggerManual,
		Depth:     causation.Depth,
		Chain:     causation.WorkflowIDs,
	})
	return run, err
}

func (s *workflowService) DryRun(ctx context.Context, input workflow.DryRunInput) (workflow.Run, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return workflow.Run{}, err
	}
	if err := s.engine.Validate(input.TriggerEventType, input.Definition); err != nil {
		return workflow.Run{}, err
	}
	wf := fromWriteInput(input.TenantID, input.ProductID, input.WriteInput)
	wf.GenerateID()
	run := workflow.Run{}
	run.GenerateID()
	return s.engine.Execute(ctx, engine.Execution{
		Workflow:  wf,
		RunID:     run.ID,
		EventID:   ids.MustNew(manualEventIDPrefix),
		EventType: wf.TriggerEventType,
		EventData: nonNil(input.EventData),
		Trigger:   workflow.RunTriggerDryRun,
	}), nil
}

func (s *workflowService) find(ctx context.Context, tenantID, productID, workflowID string) (workflow.Workflow, error) {
	found, err := s.repo.FindByID(ctx, tenantID, productID, workflowID)
	if err != nil {
		return workflow.Workflow{}, err
	}
	if found.IsAbsent() {
		return workflow.Workflow{}, errWorkflowNotFound
	}
	return found.Value(), nil
}

func fromWriteInput(tenantID, productID string, input workflow.WriteInput) workflow.Workflow {
	definition := input.Definition
	if definition.Conditions == nil {
		definition.Conditions = []workflow.Condition{}
	}
	return workflow.Workflow{
		ProductID:        productID,
		PlatformTenantID: tenantID,
		Name:             input.Name,
		Description:      input.Description,
		Enabled:          input.Enabled,
		TriggerEventType: input.TriggerEventType,
		Definition:       definition,
	}
}

func runLimit(limit int) int {
	if limit <= 0 {
		return defaultRunListLimit
	}
	return limit
}

func nonNil(data map[string]string) map[string]string {
	if data == nil {
		return map[string]string{}
	}
	return data
}
