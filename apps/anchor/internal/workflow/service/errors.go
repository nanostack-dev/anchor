package service

import (
	"errors"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

var errWorkflowNotFound = fault.NotFound(
	"WORKFLOW_NOT_FOUND",
	"This product has no workflow with that identifier.",
)

var errFinishRun = errors.New("workflow: record finished run")
