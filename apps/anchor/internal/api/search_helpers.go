package api

const (
	validationErrorCode = "VALIDATION_ERROR"
	metaRule            = "rule"
	metaValue           = "value"
)

// orEmpty reads an optional list property: a property that was not sent filters
// nothing, like an empty list.
func orEmpty[T any](list *[]T) []T {
	if list == nil {
		return nil
	}
	return *list
}
