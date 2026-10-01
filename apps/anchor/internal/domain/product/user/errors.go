package user

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

var ErrProductUserNotFound = fault.NotFound(
	"PRODUCT_USER_NOT_FOUND",
	"This product has no user with that identifier",
)
