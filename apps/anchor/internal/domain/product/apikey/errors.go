package apikey

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

var ErrProductAPIKeyNotFound = fault.NotFound(
	"PRODUCT_API_KEY_NOT_FOUND",
	"This product has no API key with that identifier",
)
