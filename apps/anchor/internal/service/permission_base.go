package service

import (
	"fmt"

	"github.com/nanostack-dev/shared/toolkit"

	"anchor/internal/domain/permission"
)

//nolint:gochecknoglobals // Static configuration data for permissions
var domains = []string{
	"organization",
	"organization_api_key",
	"organization_member",
	"workspace",
	"resources_permissions",
	"product_role",
	"product_user",
}

//nolint:gochecknoglobals // Static configuration data for permissions
var verbs = map[string]string{
	"read":   "Allow reading of the %s resource",
	"create": "Allow creating a new %s resource",
	"update": "Allow updating the %s resource",
	"delete": "Allow deleting the %s resource",
}

func GeneratePermissions() []permission.ProductPermission {
	var permissions []permission.ProductPermission

	for _, domain := range domains {
		for verb, description := range verbs {
			permissions = append(
				permissions, permission.ProductPermission{
					Name:        domain + ":" + verb,
					Description: toolkit.Ptr(fmt.Sprintf(description, domain)),
				},
			)
		}
	}

	permissions = append(permissions, permission.ProductPermission{
		Name:        "email:send",
		Description: toolkit.Ptr("Allow sending transactional email for the product"),
	})

	return permissions
}
