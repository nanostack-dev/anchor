package api

// valueOr returns *ptr if non-nil, or fallback otherwise.
func valueOr[T any](ptr *T, fallback T) T {
	if ptr != nil {
		return *ptr
	}
	return fallback
}
