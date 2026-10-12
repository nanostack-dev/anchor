package engine

import (
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	"anchor/internal/domain/workflow"
)

func unresolvedReferenceError(path string) error {
	return fmt.Errorf("{{%s}} has no value in this run", path)
}

func unknownOperatorError(operator workflow.Operator) error {
	return fmt.Errorf("unknown operator %q", operator)
}

func missingParamError(name string) error {
	return fmt.Errorf("parameter %q is empty after resolving its references", name)
}

func invalidJSONParamError(name string, err error) error {
	return fmt.Errorf("parameter %q must be a JSON object: %w", name, err)
}

// InvalidDefinitionError is the 400 every definition check returns, naming
// the part of the workflow that is wrong.
const invalidDefinitionCode = "INVALID_WORKFLOW_DEFINITION"

func InvalidDefinitionError(location, message string) error {
	return fault.BadRequest(invalidDefinitionCode, message).
		Metadata(map[string]any{"location": location})
}
