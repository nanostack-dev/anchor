package api

// valueOr returns *ptr when ptr is non-nil, or fallback otherwise. An
// optional query parameter the OpenAPI request validator does not require
// comes through a generated handler as a pointer — nil for "the caller
// omitted it" — so this is what turns that omission into whatever default
// the endpoint's own contract declares.
func valueOr[T any](ptr *T, fallback T) T {
	if ptr != nil {
		return *ptr
	}
	return fallback
}
