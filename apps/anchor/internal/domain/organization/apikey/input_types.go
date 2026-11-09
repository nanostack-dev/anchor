package orgapikey

import (
	"time"
)

type CreateOrganizationAPIKeyInput struct {
	ProductID      string  `validate:"required,notblank"`
	OrganizationID string  `validate:"required,notblank"`
	Name           string  `validate:"required,notblank,max=100"`
	Description    *string `validate:"omitempty,max=500"`
	ExpiresAt      *time.Time
	Permissions    []string `validate:"required,min=1,dive,notblank"`
}

type GetOrganizationAPIKeyInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	ID             string `validate:"required,notblank"`
}

type UpdateOrganizationAPIKeyInput struct {
	ProductID      string  `validate:"required,notblank"`
	OrganizationID string  `validate:"required,notblank"`
	ID             string  `validate:"required,notblank"`
	Name           *string `validate:"required,notblank,max=100"`
	Description    *string `validate:"omitempty,max=500"`
	Status         *Status
}

type DeleteOrganizationAPIKeyInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	ID             string `validate:"required,notblank"`
}

// SortFieldOrganizationAPIKey is the sort_by type of POST /organizations/{id}/api-keys/search.
// The generated search owns the values; this type gives the OpenAPI schema its Go type.
type SortFieldOrganizationAPIKey string

type ValidateOrganizationAPIKeyScopesInput struct {
	ProductID      string   `validate:"required,notblank"`
	OrganizationID string   `validate:"required,notblank"`
	Scopes         []string `validate:"required,dive,notblank"`
	APIKeyValue    string   `validate:"required,notblank"`
}

// IntrospectOrganizationAPIKeyInput identifies an organization API key within a
// product without a supplied organization id. Scopes are optional: when present
// they are checked and reported via MissingPrivileges.
type IntrospectOrganizationAPIKeyInput struct {
	ProductID   string   `validate:"required,notblank"`
	Scopes      []string `validate:"omitempty,dive,notblank"`
	APIKeyValue string   `validate:"required,notblank"`
}

type ValidateOrganizationAPIKeyScopesOutput struct {
	APIKey            OrganizationAPIKey
	Permissions       []string
	MissingPrivileges []string
	Authorized        bool
	Inactive          bool
}
