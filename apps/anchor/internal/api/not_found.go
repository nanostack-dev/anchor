package api

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

func notFoundBody(code, message string) ApiErrorResponse {
	return *fault.NotFound(code, message)
}
