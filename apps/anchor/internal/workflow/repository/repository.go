package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/workflow"
)

var _ Repository = (*repositoryImpl)(nil)

// Repository persists workflows and their runs. Every tenant-facing method is
// scoped by tenant and product.
type Repository interface {
	Create(ctx context.Context, wf workflow.Workflow) (workflow.Workflow, error)
	Update(ctx context.Context, wf workflow.Workflow) (functional.Option[workflow.Workflow], error)
	FindByID(ctx context.Context, tenantID, productID, workflowID string) (functional.Option[workflow.Workflow], error)
	List(ctx context.Context, tenantID, productID string) ([]workflow.Workflow, error)
	Delete(ctx context.Context, tenantID, productID, workflowID string) (bool, error)
	// FindEnabledByTriggerInternal lists the enabled workflows of a product
	// that start on the event type. It is read by the event worker only,
	// which knows the product from the event and no tenant.
	FindEnabledByTriggerInternal(ctx context.Context, productID, eventType string) ([]workflow.Workflow, error)
	// HasEnabledForTriggerInternal reports whether any enabled workflow of the
	// product starts on the event type. Emit reads it for every event.
	HasEnabledForTriggerInternal(ctx context.Context, productID, eventType string) (bool, error)

	// StartRun stores the run as running and reports false when the
	// workflow already has a run for the event, which makes a redelivered
	// event start nothing.
	StartRun(ctx context.Context, run workflow.Run) (bool, error)
	FinishRun(ctx context.Context, run workflow.Run) error
	ListRuns(
		ctx context.Context, tenantID, productID string, workflowID functional.Option[string], limit int,
	) ([]workflow.Run, error)
}

type repositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repositoryImpl{db: db}
}

func workflowScope(tenantID, productID string) postgres.BoolExpression {
	return table.ProductWorkflows.PlatformTenantID.EQ(postgres.String(tenantID)).
		AND(table.ProductWorkflows.ProductID.EQ(postgres.String(productID)))
}

func workflowByID(tenantID, productID, workflowID string) postgres.BoolExpression {
	return workflowScope(tenantID, productID).AND(table.ProductWorkflows.ID.EQ(postgres.String(workflowID)))
}

func (r *repositoryImpl) Create(ctx context.Context, wf workflow.Workflow) (workflow.Workflow, error) {
	entity, err := toModel(wf)
	if err != nil {
		return workflow.Workflow{}, err
	}
	stmt := table.ProductWorkflows.INSERT(
		table.ProductWorkflows.ID,
		table.ProductWorkflows.ProductID,
		table.ProductWorkflows.PlatformTenantID,
		table.ProductWorkflows.Name,
		table.ProductWorkflows.Description,
		table.ProductWorkflows.Enabled,
		table.ProductWorkflows.TriggerEventType,
		table.ProductWorkflows.Definition,
		table.ProductWorkflows.CreatedAt,
	).MODEL(entity).RETURNING(table.ProductWorkflows.AllColumns)
	created, err := transactor.Query[model.ProductWorkflows](ctx, r.db, stmt).Value()
	if err != nil {
		return workflow.Workflow{}, err
	}
	return toDomain(created)
}

func (r *repositoryImpl) Update(
	ctx context.Context, wf workflow.Workflow,
) (functional.Option[workflow.Workflow], error) {
	entity, err := toModel(wf)
	if err != nil {
		return functional.None[workflow.Workflow](), err
	}
	stmt := table.ProductWorkflows.UPDATE(
		table.ProductWorkflows.Name,
		table.ProductWorkflows.Description,
		table.ProductWorkflows.Enabled,
		table.ProductWorkflows.TriggerEventType,
		table.ProductWorkflows.Definition,
	).MODEL(entity).
		WHERE(workflowByID(wf.PlatformTenantID, wf.ProductID, wf.ID)).
		RETURNING(table.ProductWorkflows.AllColumns)
	return r.optional(ctx, stmt)
}

func (r *repositoryImpl) FindByID(
	ctx context.Context, tenantID, productID, workflowID string,
) (functional.Option[workflow.Workflow], error) {
	stmt := table.ProductWorkflows.SELECT(table.ProductWorkflows.AllColumns).
		FROM(table.ProductWorkflows).
		WHERE(workflowByID(tenantID, productID, workflowID))
	return r.optional(ctx, stmt)
}

func (r *repositoryImpl) List(ctx context.Context, tenantID, productID string) ([]workflow.Workflow, error) {
	stmt := table.ProductWorkflows.SELECT(table.ProductWorkflows.AllColumns).
		FROM(table.ProductWorkflows).
		WHERE(workflowScope(tenantID, productID)).
		ORDER_BY(table.ProductWorkflows.Name.ASC(), table.ProductWorkflows.ID.ASC())
	return r.list(ctx, stmt)
}

func (r *repositoryImpl) Delete(ctx context.Context, tenantID, productID, workflowID string) (bool, error) {
	stmt := table.ProductWorkflows.DELETE().
		WHERE(workflowByID(tenantID, productID, workflowID)).
		RETURNING(table.ProductWorkflows.ID)
	deleted, err := transactor.QueryOptional[model.ProductWorkflows](ctx, r.db, stmt)
	if err != nil {
		return false, err
	}
	return deleted.IsPresent(), nil
}

func (r *repositoryImpl) FindEnabledByTriggerInternal(
	ctx context.Context, productID, eventType string,
) ([]workflow.Workflow, error) {
	stmt := table.ProductWorkflows.SELECT(table.ProductWorkflows.AllColumns).
		FROM(table.ProductWorkflows).
		WHERE(
			table.ProductWorkflows.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductWorkflows.TriggerEventType.EQ(postgres.String(eventType))).
				AND(table.ProductWorkflows.Enabled.IS_TRUE()),
		).
		ORDER_BY(table.ProductWorkflows.CreatedAt.ASC())
	return r.list(ctx, stmt)
}

func (r *repositoryImpl) HasEnabledForTriggerInternal(
	ctx context.Context, productID, eventType string,
) (bool, error) {
	stmt := table.ProductWorkflows.SELECT(table.ProductWorkflows.ID).
		FROM(table.ProductWorkflows).
		WHERE(
			table.ProductWorkflows.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductWorkflows.TriggerEventType.EQ(postgres.String(eventType))).
				AND(table.ProductWorkflows.Enabled.IS_TRUE()),
		).
		LIMIT(1)
	found, err := transactor.QueryOptional[model.ProductWorkflows](ctx, r.db, stmt)
	if err != nil {
		return false, err
	}
	return found.IsPresent(), nil
}

func (r *repositoryImpl) StartRun(ctx context.Context, run workflow.Run) (bool, error) {
	entity, err := runToModel(run)
	if err != nil {
		return false, err
	}
	stmt := table.ProductWorkflowRuns.INSERT(
		table.ProductWorkflowRuns.ID,
		table.ProductWorkflowRuns.WorkflowID,
		table.ProductWorkflowRuns.ProductID,
		table.ProductWorkflowRuns.EventID,
		table.ProductWorkflowRuns.EventType,
		table.ProductWorkflowRuns.EventData,
		table.ProductWorkflowRuns.Trigger,
		table.ProductWorkflowRuns.Status,
		table.ProductWorkflowRuns.Steps,
		table.ProductWorkflowRuns.StartedAt,
	).MODEL(entity).
		ON_CONFLICT(table.ProductWorkflowRuns.WorkflowID, table.ProductWorkflowRuns.EventID).
		DO_NOTHING().
		RETURNING(table.ProductWorkflowRuns.ID)
	inserted, err := transactor.QueryOptional[model.ProductWorkflowRuns](ctx, r.db, stmt)
	if err != nil {
		return false, err
	}
	return inserted.IsPresent(), nil
}

func (r *repositoryImpl) FinishRun(ctx context.Context, run workflow.Run) error {
	entity, err := runToModel(run)
	if err != nil {
		return err
	}
	stmt := table.ProductWorkflowRuns.UPDATE(
		table.ProductWorkflowRuns.Status,
		table.ProductWorkflowRuns.Steps,
		table.ProductWorkflowRuns.Error,
		table.ProductWorkflowRuns.FinishedAt,
	).MODEL(entity).
		WHERE(table.ProductWorkflowRuns.ID.EQ(postgres.String(run.ID)))
	return transactor.Exec(ctx, r.db, stmt).Err()
}

type runRow struct {
	model.ProductWorkflowRuns

	Workflow model.ProductWorkflows
}

// ListRuns reads the latest runs of one workflow, or of every workflow of the
// product when workflowID is absent.
func (r *repositoryImpl) ListRuns(
	ctx context.Context, tenantID, productID string, workflowID functional.Option[string], limit int,
) ([]workflow.Run, error) {
	where := workflowScope(tenantID, productID)
	if workflowID.IsPresent() {
		where = where.AND(table.ProductWorkflowRuns.WorkflowID.EQ(postgres.String(workflowID.Value())))
	}
	stmt := postgres.SELECT(
		table.ProductWorkflowRuns.AllColumns,
		table.ProductWorkflows.ID,
		table.ProductWorkflows.Name,
	).FROM(
		table.ProductWorkflowRuns.INNER_JOIN(
			table.ProductWorkflows, table.ProductWorkflows.ID.EQ(table.ProductWorkflowRuns.WorkflowID),
		),
	).WHERE(where).
		ORDER_BY(table.ProductWorkflowRuns.StartedAt.DESC(), table.ProductWorkflowRuns.ID.DESC()).
		LIMIT(int64(limit))
	rows, err := transactor.Query[[]runRow](ctx, r.db, stmt).Value()
	if err != nil {
		return nil, err
	}
	runs := make([]workflow.Run, 0, len(rows))
	for _, row := range rows {
		run, decodeErr := runToDomain(row.ProductWorkflowRuns)
		if decodeErr != nil {
			return nil, decodeErr
		}
		run.WorkflowName = row.Workflow.Name
		runs = append(runs, run)
	}
	return runs, nil
}

func (r *repositoryImpl) optional(
	ctx context.Context, stmt postgres.Statement,
) (functional.Option[workflow.Workflow], error) {
	found, err := transactor.QueryOptional[model.ProductWorkflows](ctx, r.db, stmt)
	if err != nil || found.IsAbsent() {
		return functional.None[workflow.Workflow](), err
	}
	wf, err := toDomain(found.Value())
	if err != nil {
		return functional.None[workflow.Workflow](), err
	}
	return functional.Some(wf), nil
}

func (r *repositoryImpl) list(ctx context.Context, stmt postgres.Statement) ([]workflow.Workflow, error) {
	rows, err := transactor.Query[[]model.ProductWorkflows](ctx, r.db, stmt).Value()
	if err != nil {
		return nil, err
	}
	workflows := make([]workflow.Workflow, 0, len(rows))
	for _, row := range rows {
		wf, decodeErr := toDomain(row)
		if decodeErr != nil {
			return nil, decodeErr
		}
		workflows = append(workflows, wf)
	}
	return workflows, nil
}

func toModel(wf workflow.Workflow) (model.ProductWorkflows, error) {
	definition, err := json.Marshal(wf.Definition)
	if err != nil {
		return model.ProductWorkflows{}, fmt.Errorf("encode workflow definition: %w", err)
	}
	return model.ProductWorkflows{
		ID:               wf.ID,
		ProductID:        wf.ProductID,
		PlatformTenantID: wf.PlatformTenantID,
		Name:             wf.Name,
		Description:      wf.Description,
		Enabled:          wf.Enabled,
		TriggerEventType: wf.TriggerEventType,
		Definition:       string(definition),
		CreatedAt:        wf.CreatedAt,
	}, nil
}

func toDomain(row model.ProductWorkflows) (workflow.Workflow, error) {
	var definition workflow.Definition
	if err := json.Unmarshal([]byte(row.Definition), &definition); err != nil {
		return workflow.Workflow{}, fmt.Errorf("decode definition of workflow %s: %w", row.ID, err)
	}
	return workflow.Workflow{
		ID:               row.ID,
		ProductID:        row.ProductID,
		PlatformTenantID: row.PlatformTenantID,
		Name:             row.Name,
		Description:      row.Description,
		Enabled:          row.Enabled,
		TriggerEventType: row.TriggerEventType,
		Definition:       definition,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func runToModel(run workflow.Run) (model.ProductWorkflowRuns, error) {
	eventData, err := json.Marshal(run.EventData)
	if err != nil {
		return model.ProductWorkflowRuns{}, fmt.Errorf("encode run event data: %w", err)
	}
	steps, err := json.Marshal(run.Steps)
	if err != nil {
		return model.ProductWorkflowRuns{}, fmt.Errorf("encode run steps: %w", err)
	}
	return model.ProductWorkflowRuns{
		ID:         run.ID,
		WorkflowID: run.WorkflowID,
		ProductID:  run.ProductID,
		EventID:    run.EventID,
		EventType:  run.EventType,
		EventData:  string(eventData),
		Trigger:    string(run.Trigger),
		Status:     string(run.Status),
		Steps:      string(steps),
		Error:      run.Error,
		StartedAt:  run.StartedAt,
		FinishedAt: run.FinishedAt,
	}, nil
}

func runToDomain(row model.ProductWorkflowRuns) (workflow.Run, error) {
	eventData := map[string]string{}
	if err := json.Unmarshal([]byte(row.EventData), &eventData); err != nil {
		return workflow.Run{}, fmt.Errorf("decode event data of run %s: %w", row.ID, err)
	}
	steps := []workflow.StepResult{}
	if err := json.Unmarshal([]byte(row.Steps), &steps); err != nil {
		return workflow.Run{}, fmt.Errorf("decode steps of run %s: %w", row.ID, err)
	}
	return workflow.Run{
		ID:         row.ID,
		WorkflowID: row.WorkflowID,
		ProductID:  row.ProductID,
		EventID:    row.EventID,
		EventType:  row.EventType,
		EventData:  eventData,
		Trigger:    workflow.RunTrigger(row.Trigger),
		Status:     workflow.RunStatus(row.Status),
		Steps:      steps,
		Error:      row.Error,
		StartedAt:  row.StartedAt,
		FinishedAt: row.FinishedAt,
	}, nil
}
