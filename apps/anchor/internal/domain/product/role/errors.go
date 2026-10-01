package role

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

var ErrProductRoleNotFound = fault.NotFound(
	"ROLE_NOT_FOUND",
	"This product has no role with that identifier",
)
