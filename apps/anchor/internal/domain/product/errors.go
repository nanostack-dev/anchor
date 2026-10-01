package product

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

var ErrProductNotFound = fault.NotFound(
	"PRODUCT_NOT_FOUND",
	"This tenant has no product with that identifier",
)
