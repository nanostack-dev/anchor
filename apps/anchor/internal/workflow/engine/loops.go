package engine

import (
	"fmt"
	"slices"
	"strings"

	"anchor/internal/domain/workflow"
)

// LoopHop is one link of a loop: a workflow started by an event, and the
// event it emits that starts the next hop.
type LoopHop struct {
	WorkflowID   string
	WorkflowName string
	Trigger      string
	Emits        string
}

// FindLoop reports a chain of enabled workflows through which candidate
// would start itself again: candidate emits an event that, directly or
// through other workflows, emits candidate's trigger. Step conditions are
// not considered, so a loop a condition would break is still reported.
func (e *Engine) FindLoop(candidate workflow.Workflow, others []workflow.Workflow) []LoopHop {
	if !candidate.Enabled {
		return nil
	}
	enabled := []workflow.Workflow{candidate}
	for _, other := range others {
		if other.Enabled && other.ID != candidate.ID {
			enabled = append(enabled, other)
		}
	}
	byTrigger := map[string][]workflow.Workflow{}
	for _, wf := range enabled {
		byTrigger[wf.TriggerEventType] = append(byTrigger[wf.TriggerEventType], wf)
	}

	type node struct {
		event string
		path  []LoopHop
	}
	visited := map[string]bool{}
	var queue []node
	for _, emitted := range e.Emits(candidate) {
		queue = append(queue, node{event: emitted, path: []LoopHop{e.hop(candidate, emitted)}})
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.event == candidate.TriggerEventType {
			return current.path
		}
		if visited[current.event] {
			continue
		}
		visited[current.event] = true
		for _, next := range byTrigger[current.event] {
			if next.ID == candidate.ID {
				continue
			}
			for _, emitted := range e.Emits(next) {
				path := append(slices.Clone(current.path), e.hop(next, emitted))
				queue = append(queue, node{event: emitted, path: path})
			}
		}
	}
	return nil
}

func (e *Engine) hop(wf workflow.Workflow, emits string) LoopHop {
	return LoopHop{WorkflowID: wf.ID, WorkflowName: wf.Name, Trigger: wf.TriggerEventType, Emits: emits}
}

// DescribeLoop renders a loop as one readable line.
func DescribeLoop(path []LoopHop) string {
	parts := make([]string, 0, len(path))
	for _, hop := range path {
		parts = append(parts, fmt.Sprintf("“%s” (on %s) emits %s", hop.WorkflowName, hop.Trigger, hop.Emits))
	}
	return strings.Join(parts, " → ") + " → back to the start"
}
