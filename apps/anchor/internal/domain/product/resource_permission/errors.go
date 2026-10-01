package resourcepermission

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

var ErrResourcePermissionNotFound = fault.NotFound(
	"RESOURCE_PERMISSION_NOT_FOUND",
	"Resource permission does not exist",
)
