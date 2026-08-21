package api

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

// notFoundBody builds the body every 404 answer carries. The contract declares
// a body on the shared NotFound response, so an empty 404 would leave a client
// unable to tell which resource was absent.
func notFoundBody(code, message string) ApiErrorResponse {
	return *fault.NotFound(code, message)
}
