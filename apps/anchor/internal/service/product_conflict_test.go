package service_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"

	"anchor/internal/service"
)

// TestIsProductNameConflict pins down which database errors are treated as a
// racing duplicate-name create (mapped to PRODUCT_ALREADY_EXISTS) versus a
// genuine failure that must still surface as an error.
func TestIsProductNameConflict(t *testing.T) {
	const (
		constraint     = "products_platform_tenant_id_name_key"
		lowerNameIndex = "idx_products_tenant_lower_name_unique"
	)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "bare pq unique violation on the product-name constraint",
			err:  &pq.Error{Code: "23505", Constraint: constraint},
			want: true,
		},
		{
			name: "jet-wrapped pq unique violation is still unwrapped",
			err:  fmt.Errorf("jet: %w", &pq.Error{Code: "23505", Constraint: constraint}),
			want: true,
		},
		{
			// The pre-insert check compares on LOWER(name), so a race on a name
			// differing only in case lands on the case-insensitive index.
			name: "unique violation on the case-insensitive name index",
			err:  &pq.Error{Code: "23505", Constraint: lowerNameIndex},
			want: true,
		},
		{
			name: "unique violation on a different constraint is not a name conflict",
			err:  &pq.Error{Code: "23505", Constraint: "products_pkey"},
			want: false,
		},
		{
			name: "different SQLSTATE on the same constraint is not a conflict",
			err:  &pq.Error{Code: "23503", Constraint: constraint},
			want: false,
		},
		{
			name: "non-pq error is not a conflict",
			err:  errors.New("connection refused"),
			want: false,
		},
		{
			name: "nil error is not a conflict",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.IsProductNameConflict(tc.err); got != tc.want {
				t.Fatalf("IsProductNameConflict(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
