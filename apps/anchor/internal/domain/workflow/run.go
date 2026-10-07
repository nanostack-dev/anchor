package workflow

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

type RunTrigger string

const (
	RunTriggerEvent  RunTrigger = "event"
	RunTriggerManual RunTrigger = "manual"
	RunTriggerDryRun RunTrigger = "dry_run"
)

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusSkipped   RunStatus = "skipped"
)

type StepStatus string

const (
	StepStatusSucceeded StepStatus = "succeeded"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
	StepStatusSimulated StepStatus = "simulated"
)

// Run is one execution of a workflow against one event.
type Run struct {
	ID           string
	WorkflowID   string
	WorkflowName string
	ProductID    string
	EventID      string
	EventType    string
	EventData    map[string]string
	Trigger      RunTrigger
	Status       RunStatus
	Steps        []StepResult
	Error        *string
	StartedAt    time.Time
	FinishedAt   *time.Time
}

func (r *Run) GenerateID() {
	r.ID = ids.MustNew("wfrun")
}

type StepResult struct {
	StepID string         `json:"step_id"`
	Action ActionType     `json:"action"`
	Status StepStatus     `json:"status"`
	Params map[string]any `json:"params,omitempty"`
	Output map[string]any `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
}
