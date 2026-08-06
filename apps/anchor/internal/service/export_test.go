package service

// Test-only re-exports so the external service_test package can exercise
// unexported helpers without widening the production API. The testpackage
// linter skips export_test.go by design.
var (
	ReapLogLevel              = reapLogLevel
	BuildOrganizationMetadata = buildOrganizationMetadata
)
