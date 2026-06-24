package orgapikey

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
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

type SortFieldOrganizationAPIKey string

const (
	SortFieldOrganizationAPIKeyID        SortFieldOrganizationAPIKey = "id"
	SortFieldOrganizationAPIKeyCreatedAt SortFieldOrganizationAPIKey = "created_at"
	SortFieldOrganizationAPIKeyUpdatedAt SortFieldOrganizationAPIKey = "updated_at"
	SortFieldOrganizationAPIKeyName      SortFieldOrganizationAPIKey = "name"
	SortFieldOrganizationAPIKeyStatus    SortFieldOrganizationAPIKey = "status"
	SortFieldOrganizationAPIKeyLastUsed  SortFieldOrganizationAPIKey = "last_used_at"
)

type SearchOrganizationAPIKeysInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	Request        search.Request[SearchOrganizationAPIKeyFilter, SortFieldOrganizationAPIKey]
}

type SearchOrganizationAPIKeyFilter struct {
	OrganizationAPIKeyIDs []string `validate:"omitempty,dive"`
	Names                 []string `validate:"omitempty,dive,min=1"`
	Status                []string `validate:"omitempty,dive,oneof=ACTIVE INACTIVE"`
	LastUsedBefore        *time.Time
	LastUsedAfter         *time.Time
}

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
