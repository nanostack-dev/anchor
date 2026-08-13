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

// mapItems applies mapper to every item, returning a new slice rather than
// mutating items in place. Every handler that returns a paginated or listed
// response uses it to turn a []Domain into a []Response without a hand-rolled
// loop at the call site.
func mapItems[T any, R any](items []T, mapper func(T) R) []R {
	out := make([]R, 0, len(items))
	for _, item := range items {
		out = append(out, mapper(item))
	}
	return out
}
