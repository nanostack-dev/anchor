package service

import (
	"errors"
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	"anchor/internal/workflow/engine"
)

var errWorkflowNotFound = fault.NotFound(
	"WORKFLOW_NOT_FOUND",
	"This product has no workflow with that identifier.",
)

var errRunWouldLoop = fault.Conflict(
	"WORKFLOW_LOOP",
	"This request comes from a run of this workflow; running it again would start a loop.",
)

var errChainTooDeep = fault.Conflict(
	"WORKFLOW_CHAIN_TOO_DEEP",
	"This request comes from a chain of workflow runs that already holds the most runs one chain may hold.",
)

var errFinishRun = errors.New("workflow: record finished run")

func fieldTypeConflictError(conflict engine.FieldConflict) error {
	return fault.BadRequest("WORKFLOW_EVENT_FIELD_CONFLICT", conflict.Describe()).Metadata(map[string]any{
		"location":            fmt.Sprintf("steps[%d].params.data_types", conflict.StepIndex),
		"event":               conflict.Event,
		"field":               conflict.Field,
		"type":                string(conflict.Type),
		"other_type":          string(conflict.OtherType),
		"other_workflow_id":   conflict.OtherID,
		"other_workflow_name": conflict.OtherName,
	})
}

func loopError(path []engine.LoopHop) error {
	hops := make([]map[string]any, 0, len(path))
	for _, hop := range path {
		hops = append(hops, map[string]any{
			"workflow_id": hop.WorkflowID, "workflow_name": hop.WorkflowName,
			"trigger": hop.Trigger, "emits": hop.Emits,
		})
	}
	return fault.BadRequest(
		"WORKFLOW_LOOP",
		"Saving this would let the workflow start itself again: "+engine.DescribeLoop(path)+
			". Change a trigger or a step, or disable one of these workflows.",
	).Metadata(map[string]any{"loop": hops})
}
