package workspace

import apierror "github.com/nanostack-dev/nanostack-framework/pkg/apierror"

func NewNameExistsError(name, organizationID string) *apierror.Error {
	return apierror.NewBadRequestWithMetadata(
		"WORKSPACE_NAME_DUPLICATE",
		"Workspace with this name already exists in the organization",
		map[string]any{
			"name":            name,
			"organization_id": organizationID,
		},
	)
}
