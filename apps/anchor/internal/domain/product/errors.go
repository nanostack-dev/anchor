package product

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

var ErrProductNotFound = fault.NotFound(
	"PRODUCT_NOT_FOUND",
	"This tenant has no product with that identifier",
)

var ErrProductProtected = fault.Conflict(
	"PRODUCT_PROTECTED",
	"This product is protected. Turn off Protected product in its configuration before deleting it",
)
