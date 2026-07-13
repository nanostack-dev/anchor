package middleware

// Test-only re-exports so the external middleware_test package can exercise
// unexported helpers. The testpackage linter skips export_test.go by design.
var (
	DeriveRouteAction  = deriveRouteAction
	IsSkippedPath      = isSkippedPath
	LastPathParamValue = lastPathParamValue
)
